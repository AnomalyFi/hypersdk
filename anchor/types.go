package anchor

import (
	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/attestantio/go-eth2-client/spec/bellatrix"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type Data = hexutil.Bytes

type SEQPayloadRequest struct {
	Slot                   uint64                                    `json:"slot"`
	ToBBlindedBeaconBlock  AnchorSignedBlindedBeaconBlock            `json:"tobblindedbeaconblock"`
	RoBBlindedBeaconBlocks map[string]AnchorSignedBlindedBeaconBlock `json:"robblindedbeaconblocks"`
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
	Slot          uint64 `json:"slot"`
	ProposerIndex uint64 `json:"proposer_index"`
	// Hash of exec headers. Must match the value sent by AnchorGetHeaderResponse.
	HeadersHash string `json:"headers_hash"`
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
