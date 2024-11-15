package arcadia

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/crypto/bls"
	"github.com/AnomalyFi/hypersdk/emap"
	"github.com/AnomalyFi/hypersdk/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// This package contains code objects to:
// i. 	Register validator with arcadia.
// ii. 	Recieve block chunks from arcadia.
// iii. Validate block chunks.
// iv. 	Issue preconfs to arcadia.
// v. 	Store chunk info and transactions for faster auth processing.
// vi. 	Pull arcadia block content for block building.

// TODO: At the moment, arcadia uses the authWorkers `AuthVerifiers` initialized by the vm, these authWorkers are used by populateTxs method along with arcadia.
// Consider allocating arcadia its own authWorkers in future?

type Arcadia struct {
	URL                    string
	vm                     VM
	incomingChunks         chan *ArcadiaChunk
	issuePreconf           chan *ArcadiaChunk
	currEpoch              uint64
	currEpochBuilderPubKey *bls.PublicKey
	validatorPublicKey     *bls.PublicKey
	AvailableNamespaces    *[][]byte
	epochUpdatechan        chan *EpochUpdateInfo

	processedChunks *emap.EMap[*ArcadiaChunk]
	rejectedChunks  *emap.EMap[*ArcadiaChunk]
}

const (
	pingPath               = "/livez"                          // used to check if arcadia is up.
	pathSubscribeValidator = "/arcadia/v1/validator/subscribe" // subscribes validator for registering rollup chunks from arcadia.
	pathSendPreconf        = "/arcadia/v1/validator/preconf"   // validator sends preconf to arcadia.
	pathGetArcadiaBlock    = "/arcadia/v1/validator/block"     // validator requests arcadia block.
)

func NewArcadiaClient(url string, currEpoch uint64, currEpochBuilderPubKey *bls.PublicKey, availNs *[][]byte, vm VM) *Arcadia {
	url = strings.TrimRight(url, "/")
	return &Arcadia{
		URL:                    url,
		vm:                     vm,
		incomingChunks:         make(chan *ArcadiaChunk, vm.GetChunkProcessingBackLog()),
		issuePreconf:           make(chan *ArcadiaChunk, vm.GetChunkProcessingBackLog()),
		currEpoch:              currEpoch,
		currEpochBuilderPubKey: currEpochBuilderPubKey,
		validatorPublicKey:     vm.Signer(),
		AvailableNamespaces:    availNs,
		epochUpdatechan:        make(chan *EpochUpdateInfo),
		processedChunks:        emap.NewEMap[*ArcadiaChunk](),
		rejectedChunks:         emap.NewEMap[*ArcadiaChunk](),
	}
}

// Ping checks if arcadia is up and returns error if not.
func (cli *Arcadia) Ping() error {
	var client http.Client

	url := cli.URL + pingPath

	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		errMsg := new(httpErrorResp)
		if err := json.Unmarshal(bodyBytes, errMsg); err != nil {
			return fmt.Errorf("unable to parse error message from a bad response: %d", resp.StatusCode)
		}
		return fmt.Errorf("error from arcadia: %s", errMsg.Message)
	}

	return nil
}

// i.  	websocket conn starts.
// ii. 	arcadia sends random 32 bytes for validator to sign.
// iii. validator signs the random bytes with its private key and send to arcadia.
// iv. 	arcadia verifies the signature and checks if the public key belongs to one of the validators of seq.
// v. 	if above check passes, arcadia adds validator to listeners list and sends rollup chunks for preconfs.
func (cli *Arcadia) Subscribe() error {
	subscribeURL := cli.URL + pathSubscribeValidator
	conn, _, err := websocket.DefaultDialer.Dial(subscribeURL, nil)
	if err != nil {
		cli.vm.Logger().Error("Failed to connect to WebSocket server", zap.Error(err))
		return err
	}
	defer conn.Close()

	// Authenticate with arcadia.

	// Arcadia sends random 32 byte array for validator to sign.
	msgType, msg, err := conn.ReadMessage()
	if err != nil {
		cli.vm.Logger().Error("Error reading msg bytes from arcadia", zap.Error(err))
		return err
	}
	if msgType != websocket.TextMessage {
		cli.vm.Logger().Error("Expected text message from arcadia, got something else.")
		return ErrUnexpectedMsgType
	}
	if len(msg) != 32 {
		cli.vm.Logger().Error("Expected 32 bytes from arcadia, got something else.")
		return ErrUnexpectedMsgSize
	}
	// Validator signs the message and sends back to arcadia.
	uwm, err := warp.NewUnsignedMessage(cli.vm.NetworkID(), cli.vm.ChainID(), msg)
	if err != nil {
		cli.vm.Logger().Error("Failed to create unsigned message from arcadia", zap.Error(err))
		return err
	}
	sig, err := cli.vm.Sign(uwm)
	if err != nil {
		cli.vm.Logger().Error("Failed to sign message from arcadia", zap.Error(err))
		return err
	}

	sbscb := SubscribeValidatorSignatureCallback{
		Signature:          sig,
		ValidatorPublicKey: cli.vm.Signer().Compress(),
	}

	err = conn.WriteJSON(sbscb)
	if err != nil {
		cli.vm.Logger().Error("Failed to send signature to arcadia", zap.Error(err))
		return err
	}

	// Authentication successful. Now listen for rollup chunks from arcadia.

	// Listen for rollup block chunks from arcadia.
	go func() {
		for {
			var newChunk ArcadiaChunk
			err = conn.ReadJSON(newChunk)
			if err != nil {
				cli.vm.Logger().Error("Failed to read chunk from arcadia", zap.Error(err))
				continue
			}
			cli.incomingChunks <- &newChunk
		}
	}()

	return nil
}

// Run starts the arcadia client.
func (cli *Arcadia) Run() {
	// update epoch on arcadia, when changes.
	go func() {
		for {
			select {
			case newEpochInfo := <-cli.epochUpdatechan:
				cli.currEpoch = newEpochInfo.Epoch
				cli.currEpochBuilderPubKey = newEpochInfo.BuilderPubKey
				cli.AvailableNamespaces = newEpochInfo.AvailableNamespaces
				cli.processedChunks.SetMin(int64(cli.currEpoch))
				cli.rejectedChunks.SetMin(int64(cli.currEpoch))
			case <-cli.vm.StopChan():
				return
			}
		}
	}()

	// Run the chunk processing go routines in the configured number of cores.
	g := &errgroup.Group{}
	for i := 0; i < cli.vm.GetChunkCores(); i++ {
		g.Go(func() error {
			for {
				select {
				case chunk := <-cli.incomingChunks:
					if cli.rejectedChunks.Has(chunk) {
						cli.vm.Logger().Info("chunk already rejected", zap.String("chunk id", chunk.ChunkID.String()))
						continue
					}
					if cli.processedChunks.Has(chunk) {
						cli.vm.Logger().Info("chunk already preconfed", zap.String("chunk id", chunk.ChunkID.String()))
						cli.issuePreconf <- chunk
						continue
					}
					err := cli.HandleRollupChunks(chunk)
					if err != nil {
						cli.vm.Logger().Error("chunk processing error", zap.String("chunk id", chunk.ChunkID.String()), zap.Error(err))
						cli.rejectedChunks.Add([]*ArcadiaChunk{chunk})
					}
					cli.vm.Logger().Info("chunk processed", zap.String("chunk id", chunk.ChunkID.String()))
					cli.issuePreconf <- chunk
				case <-cli.vm.StopChan():
					return nil
				}
			}
		})
	}

	for i := 0; i < cli.vm.GetPreconfIssueCores(); i++ {
		g.Go(func() error {
			for {
				select {
				case chunk := <-cli.issuePreconf:
					cli.vm.Logger().Debug("issuing preconf", zap.String("chunk id", chunk.ChunkID.String()))
					err := cli.IssuePreconfs(chunk)
					if err != nil {
						cli.vm.Logger().Error("preconf issue failed", zap.String("chunk id", chunk.ChunkID.String()), zap.Error(err))
					}
					cli.vm.Logger().Info("preconf issued", zap.String("chunk id", chunk.ChunkID.String()))
				case <-cli.vm.StopChan():
					return nil
				}
			}
		})
	}
	if err := g.Wait(); err != nil {
		cli.vm.Logger().Error("chunk manager stopped with error", zap.Error(err))
	}
}

// HandleRollupChunks does
// i. 	Performs validation on the rollup chunks received from arcadia.
// ii. 	Store relevant chunk info in memory.
// iii. Issue preconfs to arcadia.
func (cli *Arcadia) HandleRollupChunks(chunk *ArcadiaChunk) error {
	// handle rollup chunks from arcadia.
	// TODO: handle edge cases.
	// if a chunk is reached for a block that is just on the epoch transition boundary.

	// use the current time as timestamp for tx.Base.ArcadiaExecute.
	currTime := time.Now().UnixMilli()
	// Chunk built is for correct epoch.
	if cli.currEpoch != chunk.Epoch {
		return fmt.Errorf("received chunk for epoch %d, but current epoch is %d", chunk.Epoch, cli.currEpoch)
	}

	// sanity checks.
	if chunk.ToBChunk != nil && chunk.RoBChunk != nil {
		return fmt.Errorf("received chunk with both ToB and RoB chunks")
	}

	if chunk.ToBChunk == nil && chunk.RoBChunk == nil {
		return fmt.Errorf("received chunk with no ToB or RoB chunks")
	}

	// signature verification.
	msg := binary.BigEndian.AppendUint64(nil, chunk.Epoch)
	msg = append(msg, chunk.ChunkID[:]...)
	builderSig, err := bls.SignatureFromBytes(chunk.BuilderSignature)
	if err != nil {
		return fmt.Errorf("failed to parse builder signature: %w", err)
	}
	if !bls.Verify(msg, cli.currEpochBuilderPubKey, builderSig) {
		return fmt.Errorf("wrong builder signature")
	}

	// santiy checks passed, signature verificaton passed -> Chunk belongs to the current epoch and is signed by the correct builder.
	// we still need to check, if ChunkID is valid.

	var jobBackLog int
	var txs []*chain.Transaction
	if chunk.ToBChunk != nil {
		// check if namespace exists.
		if !isContainsInMapping(chunk.ToBChunk.RollupIDs, chunk.ToBChunk.RollupIDToBlockNumber) {
			return fmt.Errorf("atleast a rollup id does not have block number mentioned")
		}
		// validate chunk id
		if _, err := verifyChunkID(chunk.ChunkID, chunk.ToBChunk); err != nil {
			return err
		}
		// validate ToB chunk.
		for _, tx := range chunk.ToBChunk.sTxs {
			// tx should have atleast 2 actions defined.
			if len(tx.Actions) < 2 {
				return fmt.Errorf("tx with less than 2 actions found in ToB chunk")
			}
			if err := tx.Base.ArcadiaExecute(cli.vm.ChainID(), cli.vm.Rules(currTime), currTime); err != nil {
				return fmt.Errorf("tx execution failed: %w, txID: %s", err, tx.ID().String())
			}
			// the tx should have a namespace either from the available namespaces list or defaultnamespace
			for _, action := range tx.Actions {
				if !cli.isValidNamespaceForEpoch(action.NMTNamespace()) {
					return fmt.Errorf("unregistered rollup namespace %s found in action for epoch %d", hexutil.Encode(action.NMTNamespace()), chunk.Epoch)
				}
			}
			// TODO: Should ToB chunk transactions actions restricted only to SequencerMsg and Transfer actions?
		}
		jobBackLog = len(chunk.ToBChunk.Transactions())
		txs = chunk.ToBChunk.Transactions()
	} else {
		// validate chunk id
		if _, err := verifyChunkID(chunk.ChunkID, chunk.RoBChunk); err != nil {
			return err
		}
		// validate RoB chunk.
		for _, tx := range chunk.RoBChunk.sTxs {
			if len(tx.Actions) > 1 {
				return fmt.Errorf("tx with more than 1 actions found in RoB chunk")
			}
			if err := tx.Base.ArcadiaExecute(cli.vm.ChainID(), cli.vm.Rules(currTime), currTime); err != nil {
				return fmt.Errorf("tx execution failed: %w, txID: %s", err, tx.ID().String())
			}
			if !cli.isValidNamespaceForEpoch(tx.Actions[0].NMTNamespace()) {
				return fmt.Errorf("unregistered rollup namespace %s found in action for epoch %d", hexutil.Encode(tx.Actions[0].NMTNamespace()), chunk.Epoch)
			}
			// TODO: Restrict RoB chunk transactions actions to only SequencerMsg action?
		}
		jobBackLog = len(chunk.ToBChunk.Transactions())
		txs = chunk.ToBChunk.Transactions()
	}

	// batch verify chunk tx signatures.
	job, err := cli.vm.AuthVerifiers().NewJob(jobBackLog)
	if err != nil {
		return err
	}

	bv := chain.NewAuthBatch(cli.vm, job, chunk.authCounts)
	for _, tx := range txs {
		txDigest, err := tx.Digest()
		if err != nil {
			return err
		}
		bv.Add(txDigest, tx.Auth)
	}
	bv.Done(nil)
	err = job.Wait()
	if err != nil {
		return fmt.Errorf("chunk batch signature verification failed. error: %w", err)
	}

	// Batch signature check has passed. All the transaction's signatures in the chunk are valid.
	// TODO: Add bonded accounts.
	// check if every transaction is fee payable? This check makes sense when paired with bonded accounts.
	// @todo add checks for chunk nonces. --> is tob nonce populated by arcadia?
	// Add the transactions to the map.

	// Add auth verified txs to a emap. When a block is accepted do setMinTx to remove all expired transactions.
	cli.vm.AddToArcadiaAuthVerifiedTxs(txs)
	// we are here at a situation. should we reject if any transaction is seen before and in the validity window?
	// yes. --> replay protection.

	return nil
}

// send preconfs to arcadia.
func (cli *Arcadia) IssuePreconfs(chunk *ArcadiaChunk) error {
	var client http.Client

	url := cli.URL + pathSendPreconf

	pubKey := cli.validatorPublicKey.Compress()
	msg := append([]byte{}, chunk.ChunkID[:]...)
	msg = append(msg, pubKey...)
	// Validator signs the message.
	uwm, err := warp.NewUnsignedMessage(cli.vm.NetworkID(), cli.vm.ChainID(), msg)
	if err != nil {
		cli.vm.Logger().Error("failed to create unsigned message for issuing preconf", zap.Error(err))
		return err
	}
	sig, err := cli.vm.Sign(uwm)
	if err != nil {
		cli.vm.Logger().Error("failed to sign message for issuing preconf", zap.Error(err))
		return err
	}
	valMsg := ValidatorMessage{
		ChunkID:            chunk.ChunkID,
		Signature:          sig,
		ValidatorPublicKey: pubKey,
	}
	reqRaw, err := json.Marshal(valMsg)
	if err != nil {
		cli.vm.Logger().Error("failed to marshal preconf message", zap.Error(err))
		return err
	}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(reqRaw))
	if err != nil {
		cli.vm.Logger().Error("failed to send preconf to arcadia", zap.Error(err))
		return err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		errMsg := new(httpErrorResp)
		if err := json.Unmarshal(bodyBytes, errMsg); err != nil {
			return fmt.Errorf("arcadia: unable to parse error message from a bad response: %d", resp.StatusCode)
		}
		return fmt.Errorf("error from arcadia: %s", errMsg.Message)
	}

	return nil
}

func (cli *Arcadia) GetBlockPaylodFromArcadia(maxBw uint64) (*ArcadiaBlockPayload, error) {
	var client http.Client

	url := cli.URL + pathGetArcadiaBlock

	req := GetBlockPayloadFromArcadia{
		MaxBandwidth: maxBw,
	}
	reqRaw, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(reqRaw))
	if err != nil {
		return nil, err
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		errMsg := new(httpErrorResp)
		if err := json.Unmarshal(bodyBytes, errMsg); err != nil {
			return nil, fmt.Errorf("unable to parse error message from bad response: %d", resp.StatusCode)
		}
		fmt.Errorf("error from arcadia: %s", errMsg.Message)
	}

	var payload ArcadiaBlockPayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return nil, err
	}

	return &payload, nil
}

// Checks if chunkID given matches computed chunkID. returns err for chunkID mismatch and true for match.
func verifyChunkID(chunkID ids.ID, chunk ChunkInterface) (bool, error) {
	b, err := chunk.Marshal()
	if err != nil {
		return false, fmt.Errorf("error marshalling tob chunk: %w", err)
	}
	chunkIDg := utils.ToID(b)
	if chunkIDg != chunkID {
		return false, fmt.Errorf("chunk id mismatch. received: %s, computed: %s", chunkID, chunkIDg)
	}
	return true, nil
}

// returns true, if namespace exists in the list of namespaces for the current epoch or default namespace.
func (cli *Arcadia) isValidNamespaceForEpoch(namespace []byte) bool {
	if bytes.Equal(namespace, DefaultNMTNamespace) {
		return true
	}

	for _, ns := range *cli.AvailableNamespaces {
		if bytes.Equal(ns, namespace) {
			return true
		}
	}

	return false
}
