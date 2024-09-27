package anchor

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/attestantio/go-eth2-client/spec/bellatrix"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/flashbots/go-boost-utils/bls"
)

type Data = hexutil.Bytes

type SEQPayloadRequest struct {
	Slot                   uint64                                    `json:"slot"`
	ToBBlindedBeaconBlock  AnchorSignedBlindedBeaconBlock            `json:"tobblindedbeaconblock"`
	RoBBlindedBeaconBlocks map[string]AnchorSignedBlindedBeaconBlock `json:"robblindedbeaconblocks"`
}

type AnchorHeader struct {
	Header    *common.Hash `json:"header"`
	BlockHash string       `json:"block_hash"`
	Value     *big.Int     `json:"value"`
}

type ExecHeadersInfo struct {
	// Make signature based off ToBHash + RoBHashes then we use this signature for Baton/Anchor to check against
	ToBHash   *AnchorHeader            `json:"tobhash"`
	RoBHashes map[string]*AnchorHeader `json:"robhashes"`
}

func HashExecHeaders(headers *ExecHeadersInfo) ([32]byte, error) {
	// Use JSON serialization to hash the struct
	payloadBytes, err := json.Marshal(*headers)
	if err != nil {
		return [32]byte{}, fmt.Errorf("failed to serialize ExecHeaders: %w", err)
	}

	// Use sha256 to hash the serialized ExecHeaders data
	hash := sha256.Sum256(payloadBytes)
	return hash, nil
}

type AnchorGetHeaderResponse struct {
	ExecHeaders ExecHeadersInfo `json:"exec_headers"`
	BlockInfo   AnchorBlockInfo `json:"block_info"`
	ParentHash  common.Hash     `json:"parent_hash"`
	// Exec headers signed by baton's key.
	ExecHeadersSig []byte `json:"exec_headers_sig"`
}

type AnchorBlockInfo struct {
	Slot uint64 `json:"slot"`
	// nodeID of chunk producing validator.
	Producer       ids.NodeID    `json:"producer"`
	ProposerPubkey bls.PublicKey `json:"proposer_pubkey"`
}

type SEQHeaderResponse struct {
	Slot uint64 `json:"slot"`
	// nodeID of chunk producing validator.
	Producer ids.NodeID `json:"producer"`
	// block builder address
	PriorityFeeReceiverAddr codec.Address `json:"priorityfeereceiveraddr"`
	// hash of the anchor chunks (tob + robs)
	ChunkHash phase0.Hash32            `json:"chunkhash"`
	ToBHash   phase0.Hash32            `json:"tobhash"`
	RoBHashes map[string]phase0.Hash32 `json:"robhashes"`
}

type AnchorSignedBlindedBeaconBlock struct {
	Message   *AnchorBlindedBeaconBlock
	Signature phase0.BLSSignature `ssz-size:"96"`
}

type AnchorBlindedBeaconBlock struct {
	Slot          phase0.Slot
	ProposerIndex phase0.ValidatorIndex
	ParentRoot    phase0.Root `ssz-size:"32"`
	StateRoot     phase0.Root `ssz-size:"32"`
	Body          *AnchorBlindedBeaconBlockBody
}

type AnchorBlindedBeaconBlockBody struct {
	ExecutionPayloadHeader *AnchorExecutionPayloadHeader
}

// receiving payload from SEQ
type AnchorExecutionPayloadHeader struct {
	FeeRecipient     bellatrix.ExecutionAddress `ssz-size:"20"`
	StateRoot        [32]byte                   `ssz-size:"32"`
	ReceiptsRoot     [32]byte                   `ssz-size:"32"`
	LogsBloom        [256]byte                  `ssz-size:"256"`
	BlockNumber      uint64
	Timestamp        uint64
	BlockHash        phase0.Hash32 `ssz-size:"32"`
	TransactionsRoot phase0.Root   `ssz-size:"32"`
	ChunkDigest      phase0.Root   `ssz-size:"32"`
}

type SEQPayloadResponse struct {
	Slot        uint64                       `json:"slot"`
	ToBPayload  ExecutionPayload2            `json:"tobpayload"`
	RoBPayloads map[string]ExecutionPayload2 `json:"robpayloads"`
}

type ExecutionPayload2 struct {
	Slot      uint64      `json:"slot"`
	BlockHash common.Hash `json:"blockHash"`
	// Array of transaction objects, each object is a byte list (DATA) representing
	// TransactionType || TransactionPayload or LegacyTransaction as defined in EIP-2718
	Transactions []byte `json:"transactions"`
}

type AnchorGetPayloadRequest struct {
	Slot           uint64 `json:"slot"`
	ProposerPubKey []byte `json:"proposer_pubkey"`
	ParentHash     string `json:"parent_hash"`
	// Exec headers signed by validator's private key. Should be [48]byte signature.
	SignedHeaders []byte `json:"signed_headers"`
}

// Note ExecPayloadsSig is the execpayloads with Baton's private key. It is verified by Anchor.
type AnchorGetPayloadResponse struct {
	Slot            uint64           `json:"slot"`
	ExecPayloads    ExecPayloadsInfo `json:"execpayloads"`
	ExecPayloadsSig []byte           `json:"execpayloads_sig"`
}

type ExecPayloadsInfo struct {
	ToBPayload  *ExecutionPayload           `json:"tobpayload"`
	RoBPayloads map[string]ExecutionPayload `json:"robpayloads"`
}

type ExecutionPayload struct {
	// hypersdk transactions in byte slice format
	Transactions []byte `json:"transactions"`
}
