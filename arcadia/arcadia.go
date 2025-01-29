package arcadia

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/AnomalyFi/hypersdk/actions"
	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/consts"
	"github.com/AnomalyFi/hypersdk/crypto/bls"
	"github.com/AnomalyFi/hypersdk/emap"
	"github.com/AnomalyFi/hypersdk/utils"
	avacache "github.com/ava-labs/avalanchego/cache"

	hactions "github.com/AnomalyFi/hypersdk/actions"
)

// This package contains code objects to:
// i. 	Register validator with arcadia.
// ii. 	Receive block chunks from arcadia.
// iii. Validate block chunks.
// iv. 	Issue preconfs to arcadia.
// v. 	Store chunk info and transactions for faster auth processing.
// vi. 	Pull arcadia block content for block building.

// TODO: At the moment, arcadia uses the authWorkers `AuthVerifiers` initialized by the vm, these authWorkers are used by populateTxs method along with arcadia.
// Consider allocating arcadia its own authWorkers in future?

type Arcadia struct {
	URL string
	vm  VM

	validatorPublicKey *bls.PublicKey

	incomingChunks chan *ArcadiaToSEQChunkMessage
	issuePreconf   chan *ArcadiaToSEQChunkMessage

	currEpoch uint64

	epochUpdatechan chan *EpochUpdateInfo

	// chainID -> rollupInfo
	rollups  map[string]*actions.RollupInfo
	rollupsL sync.RWMutex

	builders *avacache.LRU[uint64, *bls.PublicKey]

	processedChunks *emap.EMap[*ArcadiaToSEQChunkMessage]
	rejectedChunks  *emap.EMap[*ArcadiaToSEQChunkMessage]

	epochInfoStoringDepth int

	lastReconnect time.Time
	conn          *websocket.Conn
	isConnected   atomic.Bool
	stopCalled    atomic.Bool
	stop          chan struct{}
}

const (
	pingPath               = "/livez"                             // used to check if arcadia is up.
	pathSubscribeValidator = "/ws/arcadia/v1/validator/subscribe" // subscribes validator for registering rollup chunks from arcadia.
	pathSendPreconf        = "/api/arcadia/v1/validator/preconf"  // validator sends preconf to arcadia.
	pathGetArcadiaBlock    = "/api/arcadia/v1/validator/block"    // validator requests arcadia block.
)

func NewArcadiaClient(url string, currEpoch uint64, currEpochBuilderPubKey *bls.PublicKey, availNs *[][]byte, vm VM) *Arcadia {
	url = strings.TrimRight(url, "/")

	cli := &Arcadia{
		URL:                url,
		vm:                 vm,
		validatorPublicKey: vm.Signer(),
		incomingChunks:     make(chan *ArcadiaToSEQChunkMessage, vm.GetChunkProcessingBackLog()),
		issuePreconf:       make(chan *ArcadiaToSEQChunkMessage, vm.GetChunkProcessingBackLog()),
		currEpoch:          currEpoch,
		epochUpdatechan:    make(chan *EpochUpdateInfo),
		rollups:            make(map[string]*hactions.RollupInfo),
		processedChunks:    emap.NewEMap[*ArcadiaToSEQChunkMessage](),
		rejectedChunks:     emap.NewEMap[*ArcadiaToSEQChunkMessage](),
		stop:               make(chan struct{}),

		lastReconnect: time.Now(),
		stopCalled:    atomic.Bool{},
		isConnected:   atomic.Bool{},
	}
	cli.builders = &avacache.LRU[uint64, *bls.PublicKey]{Size: DefaultBuilderLRUSize}

	// update epoch on arcadia, when changes.
	go func() {
		for {
			select {
			case newEpochInfo := <-cli.epochUpdatechan:
				cli.vm.Logger().Debug("epoch update received", zap.Uint64("epoch", newEpochInfo.Epoch))

				if cli.currEpoch != newEpochInfo.Epoch {
					ctx := context.Background()
					ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
					if err := cli.epochUpdate(ctx); err != nil {
						cli.vm.Logger().Error("unable to conduct epoch update", zap.Error(err))
					}
					cancel()
				}
				cli.currEpoch = newEpochInfo.Epoch

				cli.processedChunks.SetMin(int64(cli.currEpoch))
				cli.rejectedChunks.SetMin(int64(cli.currEpoch))
			case <-cli.stop:
				cli.vm.Logger().Info("shutitng down epoch update core")
				return
			case <-cli.vm.StopChan():
				return
			}
		}
	}()

	return cli
}

// Ping checks if arcadia is up and returns error if not.
func (cli *Arcadia) Ping() error {
	var client http.Client

	url := cli.URL + pingPath

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
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
	subscribeURL = replaceHTTPWithWS(subscribeURL)
	conn, _, err := websocket.DefaultDialer.Dial(subscribeURL, nil) //nolint:bodyclose
	if err != nil {
		cli.vm.Logger().Error("Failed to connect to WebSocket server", zap.Error(err))
		return err
	}

	// Authenticate with arcadia.

	// Arcadia sends random 32 byte array for validator to sign.
	msgType, msg, err := conn.ReadMessage()
	if err != nil {
		cli.vm.Logger().Error("Error reading msg bytes from arcadia", zap.Error(err))
		conn.Close()
		return err
	}
	if msgType != websocket.TextMessage {
		cli.vm.Logger().Error("Expected text message from arcadia, got something else.")
		conn.Close()
		return ErrUnexpectedMsgType
	}
	if len(msg) != 32 {
		cli.vm.Logger().Error("Expected 32 bytes from arcadia, got something else.")
		conn.Close()
		return ErrUnexpectedMsgSize
	}
	// Validator signs the message and sends back to arcadia.
	uwm, err := warp.NewUnsignedMessage(cli.vm.NetworkID(), cli.vm.ChainID(), msg)
	if err != nil {
		cli.vm.Logger().Error("Failed to create unsigned message from arcadia", zap.Error(err))
		conn.Close()
		return err
	}
	sig, err := cli.vm.Sign(uwm)
	if err != nil {
		cli.vm.Logger().Error("Failed to sign message from arcadia", zap.Error(err))
		conn.Close()
		return err
	}

	sbscb := SubscribeValidatorSignatureCallback{
		Signature:          sig,
		ValidatorPublicKey: cli.vm.Signer().Compress(),
	}
	err = conn.WriteJSON(sbscb)
	if err != nil {
		cli.vm.Logger().Error("Failed to send signature to arcadia", zap.Error(err))
		conn.Close()
		return err
	}

	// Authentication successful. Now listen for rollup chunks from arcadia.
	cli.conn = conn
	cli.isConnected.Store(true)
	// @todo handle the changes with retry for connection lost.
	// Listen for rollup block chunks from arcadia.
	go func() {
		for {
			if connected := cli.isConnected.Load(); !connected {
				time.Sleep(1 * time.Second)
				continue
			}

			msgType, rawMsg, err := conn.ReadMessage()
			if err != nil {
				// Handle WebSocket closure.
				if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) ||
					websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) ||
					strings.Contains(err.Error(), "close 1006") ||
					strings.Contains(err.Error(), "use of closed network connection") {

					cli.vm.Logger().Error("WebSocket connection closed", zap.Error(err))
					// Attempt reconnect to arcadia.
					cli.isConnected.Store(false)
					cli.Reconnect()
					// Exit the current loop if connection is closed.
					return
				}
				cli.vm.Logger().Error("Failed to read raw message from arcadia", zap.Error(err))
				continue
			}
			cli.vm.Logger().Info("received message from arcadia", zap.Int("msgType", msgType), zap.String("rawMsg", string(rawMsg)))

			if msgType == websocket.TextMessage {
				var newChunk ArcadiaToSEQChunkMessage
				err := json.Unmarshal(rawMsg, &newChunk)
				if err != nil {
					cli.vm.Logger().Warn("unable to parse chunk", zap.Error(err))
					continue
				}
				l2ChainID := "tob"
				height := uint64(0)
				if newChunk.Chunk.RoB != nil {
					l2ChainID = newChunk.Chunk.RoB.ChainID
					height = newChunk.Chunk.RoB.BlockNumber
				}
				cli.vm.Logger().Info("Received chunk from arcadia", zap.String("chunk id", newChunk.ChunkID.String()), zap.String("chainID", l2ChainID), zap.Uint64("height", height))
				cli.vm.RecordChunksReceived()
				cli.incomingChunks <- &newChunk
			} else {
				continue
			}
		}
	}()

	cli.vm.Logger().Info("subscribed to Arcadia", zap.String("endpoint", cli.URL), zap.String("path", pathSubscribeValidator))
	return nil
}

// Run starts the arcadia client.
func (cli *Arcadia) Run() {
	// Run the chunk processing go routines in the configured number of cores.
	g := &errgroup.Group{}
	for i := 0; i < cli.vm.GetChunkCores(); i++ {
		g.Go(func() error {
			for {
				select {
				case chunk := <-cli.incomingChunks:
					if cli.rejectedChunks.Has(chunk) {
						cli.vm.Logger().Info("chunk already rejected", zap.String("chunkID", chunk.ChunkID.String()))
						continue
					}
					if cli.processedChunks.Has(chunk) {
						cli.vm.Logger().Info("chunk already processed", zap.String("chunkID", chunk.ChunkID.String()))
						cli.issuePreconf <- chunk
						continue
					}
					t := time.Now()
					ctx := context.Background()
					ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
					err := cli.HandleRollupChunks(ctx, chunk)
					cancel()
					cli.vm.RecordChunkProcessDuration(time.Since(t))
					if err != nil {
						cli.vm.Logger().Error("chunk processing error", zap.String("chunkID", chunk.ChunkID.String()), zap.Error(err))
						cli.rejectedChunks.Add([]*ArcadiaToSEQChunkMessage{chunk})
						cli.vm.RecordChunksRejected()
						continue
					}
					cli.vm.Logger().Info("chunk processed", zap.String("chunk id", chunk.ChunkID.String()))
					cli.vm.RecordChunksAccepted()
					cli.processedChunks.Add([]*ArcadiaToSEQChunkMessage{chunk})
					cli.issuePreconf <- chunk
				case <-cli.stop:
					cli.vm.Logger().Info("shutting down chunk processing cores")
					return nil
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
				case <-cli.stop:
					cli.vm.Logger().Info("shutting down preconf issue cores")
					return nil
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
func (cli *Arcadia) HandleRollupChunks(ctx context.Context, chunk *ArcadiaToSEQChunkMessage) error {
	// handle rollup chunks from arcadia.
	// TODO: handle edge cases.
	// if a chunk is reached for a block that is just on the epoch transition boundary.

	chunkEpoch := chunk.Epoch

	builderPubkey, err := cli.auctionWinnerAtEpoch(ctx, chunkEpoch)
	if err != nil && err != database.ErrNotFound {
		return err
	}

	// there's a situation that builder pubkey to be nil but there's still chunks coming in
	// since that the auction tx didn't get included but the best auction bid declared on Arcadia
	// this situation that Arcadia will keep accepting chunks from the winning builder but SEQ nodes won't be able to verify it
	verifySig := err == nil

	chainID := "ToB"
	height := uint64(0)
	if chunk.Chunk.RoB != nil {
		chainID = chunk.Chunk.RoB.ChainID
		height = chunk.Chunk.RoB.BlockNumber
	}

	cli.vm.Logger().Debug("handling chunk", zap.String("chunkID", chunk.ChunkID.String()), zap.Uint64("currentEpoch", cli.currEpoch), zap.Uint64("chunkEpoch", chunk.Epoch), zap.String("chainID", chainID), zap.Uint64("height", height), zap.String("epochBuilder", hexutil.Encode(builderPubkey.Compress())))

	// use the current time as timestamp for tx.Base.ArcadiaExecute.
	currTime := time.Now().UnixMilli()
	if err := chunk.Initialize(cli.vm); err != nil {
		return fmt.Errorf("failed to initialize chunk: %w", err)
	}

	// sanity checks.
	if chunk.Chunk.ToB != nil && chunk.Chunk.RoB != nil {
		return ErrChunkWithBothToBAndRoB
	}

	if chunk.Chunk.ToB == nil && chunk.Chunk.RoB == nil {
		return ErrChunkWithNoToBAndRoB
	}

	if len(chunk.sTxs) != int(chunk.removedBitSet.Len()) {
		return ErrInvalidBitSetLengthMisMatch
	}
	// validate chunk id
	if _, err := verifyChunkID(chunk.ChunkID, chunk.Chunk); err != nil {
		return err
	}

	// signature verification.
	if verifySig {
		msg := binary.LittleEndian.AppendUint64(nil, chunkEpoch)
		msg = append(msg, chunk.ChunkID[:]...)
		builderSig, err := bls.SignatureFromBytes(chunk.BuilderSignature)
		if err != nil {
			return fmt.Errorf("failed to parse builder signature: %w", err)
		}
		if !bls.Verify(msg, builderPubkey, builderSig) {
			return ErrBuilderSignature
		}
	}

	// santiy checks passed, signature verificaton passed -> Chunk belongs to the current epoch and is signed by the correct builder.
	// we still need to check, if ChunkID is valid.

	var txs []*chain.Transaction
	if chunk.Chunk.ToB != nil {
		// validate ToB chunk.
		for i, tx := range chunk.sTxs {
			// if the tx is removed, skip the validation.
			if chunk.removedBitSet.Test(uint(i)) {
				continue
			}
			if err := tx.Base.ArcadiaExecute(cli.vm.ChainID(), cli.vm.Rules(currTime), currTime); err != nil {
				return fmt.Errorf("tx execution failed: %w, txID: %s", err, tx.ID().String())
			}
			// the tx should have a namespace either from the available namespaces list or defaultnamespace
			for _, action := range tx.Actions {
				if !cli.isValidNamespaceForEpoch(chunkEpoch, action.NMTNamespace()) {
					return fmt.Errorf("unregistered rollup namespace %s found in action for epoch %d", hexutil.Encode(action.NMTNamespace()), chunk.Epoch)
				}
				// Restrict ToB chunk transactions actions to only Transfer and SequencerMsg actions.
				if !(action.GetTypeID() == hactions.TransferID || action.GetTypeID() == hactions.MsgID) {
					return ErrToBChunkWithNonAcceptableActions
				}
			}
			txs = append(txs, tx)
		}
	} else {
		// validate RoB chunk.
		for i, tx := range chunk.sTxs {
			// if the tx is removed, skip the validation.
			if chunk.removedBitSet.Test(uint(i)) {
				continue
			}
			if len(tx.Actions) > 1 {
				return ErrMoreThanOneAction
			}
			if err := tx.Base.ArcadiaExecute(cli.vm.ChainID(), cli.vm.Rules(currTime), currTime); err != nil {
				return fmt.Errorf("tx execution failed: %w, txID: %s", err, tx.ID().String())
			}
			chainIDu64 := binary.LittleEndian.Uint64(tx.Actions[0].NMTNamespace())
			txChainID := hexutil.EncodeBig(big.NewInt(int64(chainIDu64)))
			if chunk.Chunk.RoB.ChainID != txChainID {
				return fmt.Errorf("chainID of tx not equal to chainID of chunk. tx chainID: %s, chunk chainID: %s", txChainID, chunk.Chunk.RoB.ChainID)
			}
			if !cli.isValidNamespaceForEpoch(chunkEpoch, tx.Actions[0].NMTNamespace()) {
				return fmt.Errorf("unregistered rollup namespace %s found in action for epoch %d", hexutil.Encode(tx.Actions[0].NMTNamespace()), chunk.Epoch)
			}
			// Restrict RoB chunk transactions actions to only SequencerMsg action?
			if tx.Actions[0].GetTypeID() != hactions.MsgID {
				return ErrNonSequencerMessage
			}
			txs = append(txs, tx)
		}
	}
	cli.vm.RecordValidTxsInChunksReceived(len(txs))
	// batch verify chunk tx signatures.
	job, err := cli.vm.AuthVerifiers().NewJob(len(txs))
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

	// Add the transactions to the map and give replay protection?

	// Add auth verified txs to a emap. When a block is accepted do setMinTx to remove all expired transactions.
	cli.vm.AddToArcadiaAuthVerifiedTxs(txs)

	return nil
}

// send preconfs to arcadia.
func (cli *Arcadia) IssuePreconfs(chunk *ArcadiaToSEQChunkMessage) error {
	var client http.Client

	url := cli.URL + pathSendPreconf

	pubKey := cli.validatorPublicKey.Compress()
	msg := append([]byte{}, chunk.ChunkID[:]...)
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

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqRaw))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		cli.vm.Logger().Error("failed to send preconf to arcadia", zap.Error(err))
		return err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		errMsg := new(httpErrorResp)
		if err := json.Unmarshal(bodyBytes, errMsg); err != nil {
			return fmt.Errorf("arcadia: unable to parse error message from a bad response: %d", resp.StatusCode)
		}
		return fmt.Errorf("error from arcadia: %s", errMsg.Message)
	}

	return nil
}

func (cli *Arcadia) GetBlockPayloadFromArcadia(ctx context.Context, maxBw, blockNumber uint64) (*ArcadiaBlockPayload, error) {
	var client http.Client

	url := cli.URL + pathGetArcadiaBlock

	reqr := GetBlockPayloadFromArcadia{
		MaxBandwidth: maxBw,
		BlockNumber:  blockNumber,
	}
	reqRaw, err := json.Marshal(reqr)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqRaw))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		errMsg := new(httpErrorResp)
		if err := json.Unmarshal(bodyBytes, errMsg); err != nil {
			return nil, fmt.Errorf("unable to parse error message from bad response: %d", resp.StatusCode)
		}
		return nil, fmt.Errorf("error from arcadia: %s", errMsg.Message)
	}

	var payload ArcadiaBlockPayload
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return nil, err
	}

	return &payload, nil
}

// Checks if chunkID given matches computed chunkID. returns err for chunkID mismatch and true for match.
func verifyChunkID(chunkID ids.ID, chunk *ArcadiaChunk) (bool, error) {
	var payload []byte
	if chunk.ToB != nil {
		pd, err := json.Marshal(chunk.ToB)
		if err != nil {
			return false, fmt.Errorf("error marshalling tob chunk: %w", err)
		}
		payload = pd
	} else {
		pd, err := json.Marshal(chunk.RoB)
		if err != nil {
			return false, fmt.Errorf("error marshalling rob chunk: %w", err)
		}
		payload = pd
	}
	payloadHash := sha256.Sum256(payload)
	chunkIDc := utils.ToID(payloadHash[:])
	if chunkIDc != chunkID {
		return false, fmt.Errorf("chunk id mismatch. received: %s, computed: %s", chunkID, chunkIDc)
	}
	return true, nil
}

// returns true, if namespace exists in the list of namespaces for the current epoch or default namespace.
func (cli *Arcadia) isValidNamespaceForEpoch(epoch uint64, namespace []byte) bool {
	cli.rollupsL.RLock()
	defer cli.rollupsL.RUnlock()

	if bytes.Equal(namespace, DefaultNMTNamespace) {
		return true
	}

	chainID := namespaceToChainIDStr(namespace)
	rollup := cli.rollups[chainID]
	if rollup == nil {
		cli.vm.Logger().Debug("rollup not exists", zap.String("chainID", chainID))
		return false
	}

	valid := true
	if rollup.StartEpoch > epoch {
		valid = false
	}

	if rollup.ExitEpoch != 0 && rollup.ExitEpoch < epoch {
		valid = false
	}

	return valid
}

func (cli *Arcadia) auctionWinnerAtEpoch(ctx context.Context, epoch uint64) (*bls.PublicKey, error) {
	winner, exists := cli.builders.Get(epoch)
	if exists {
		return winner, nil
	}

	auctionKey := actions.ArcadiaBidKey(epoch)
	values, errs := cli.vm.ReadState(ctx, [][]byte{auctionKey})
	if len(errs) != 1 || len(errs) != len(values) {
		return nil, ErrStateReadReturnNotCorrect
	}

	if errs[0] != nil {
		return nil, errs[0]
	}

	value := values[0]
	_, pubkeyBytes, _, err := actions.UnpackArcadiaAuctionWinner(value)
	if err != nil {
		return nil, err
	}

	winner, err = bls.PublicKeyFromBytes(pubkeyBytes)
	if err != nil {
		return nil, err
	}

	cli.builders.Put(epoch, winner)
	return winner, nil
}

// epochUpdate should be called at block basis to reduce reads
func (cli *Arcadia) epochUpdate(ctx context.Context) error {
	cli.rollupsL.Lock()
	defer cli.rollupsL.Unlock()

	rollupRegistryKey := actions.RollupRegistryKey()
	values, errs := cli.vm.ReadState(ctx, [][]byte{rollupRegistryKey})
	if len(errs) != 1 || len(errs) != len(values) {
		return ErrStateReadReturnNotCorrect
	}

	if errs[0] != nil {
		return errs[0]
	}

	value := values[0]
	namespaces, err := actions.UnpackNamespaces(value)
	if err != nil {
		return err
	}

	rollupInfoKeys := make([][]byte, 0, len(namespaces))
	for _, ns := range namespaces {
		rollupInfoKeys = append(rollupInfoKeys, actions.RollupInfoKey(ns))
	}

	values, errs = cli.vm.ReadState(ctx, [][]byte{rollupRegistryKey})
	if len(errs) != len(rollupInfoKeys) || len(errs) != len(values) {
		return ErrStateReadReturnNotCorrect
	}

	for i, err := range errs {
		if err != nil {
			return fmt.Errorf("err querying key: %+v, err: %w", rollupInfoKeys[i], err)
		}
	}

	newRollupInfo := make(map[string]*actions.RollupInfo)
	for _, value := range values {
		p := codec.NewReader(value, consts.NetworkSizeLimit)
		rollupInfo, err := actions.UnmarshalRollupInfo(p)
		if err != nil {
			return err
		}
		chainIDStr := namespaceToChainIDStr(rollupInfo.Namespace)
		cli.vm.Logger().Debug("setting rollup info", zap.String("chainID", chainIDStr), zap.Uint64("startEpoch", rollupInfo.StartEpoch), zap.Uint64("exitEpoch", rollupInfo.ExitEpoch))
		newRollupInfo[chainIDStr] = rollupInfo
	}

	cli.rollups = newRollupInfo
	return nil
}
