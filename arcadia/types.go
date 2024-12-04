package arcadia

import (
	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/crypto/bls"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/bits-and-blooms/bitset"
)

type EpochUpdateInfo struct {
	Epoch               uint64
	BuilderPubKey       *bls.PublicKey
	AvailableNamespaces *[][]byte
}

type SubscribeValidatorSignatureCallback struct {
	Signature          []byte `json:"signature"`
	ValidatorPublicKey []byte `json:"validatorPublicKey"`
}

type ArcadiaToBChunk struct {
	RollupIDs             []string          `json:"rollupIDs"`
	RollupIDToBlockNumber map[string]uint64 `json:"rollupIDToBlockNumber"`
	Txs                   []byte            `json:"txs"`
	RemovedBitSet         []byte            `json:"removedBitSet"`
	Nonce                 uint64            `json:"nonce"`

	sTxs []*chain.Transaction
	bs   *bitset.BitSet
}

type ArcadiaRoBChunk struct {
	RollupID      string `json:"rollupID"`
	BlockNumber   uint64 `json:"blockNumber"`
	Txs           []byte `json:"txs"`
	RemovedBitSet []byte `json:"removedBitSet"`
	Nonce         uint64 `json:"nonce"`

	sTxs []*chain.Transaction
	bs   *bitset.BitSet
}

type CrossRollupBundle struct {
	BundleHash        string              `json:"bundleHash"`
	Txs               []byte              `json:"txs"` // a seq seqMsgTx
	RevertingTxHashes map[string]struct{} `json:"revertingTxHashes"`

	seqTx    *chain.Transaction
	authType uint8
}

type ToBChunk struct {
	Bundles               []*CrossRollupBundle `json:"bundles"`
	RollupIDToBlockNumber map[string]uint64    `json:"blockNumber"`

	rollupIDs []string
}

type RoBChunk struct {
	ChainID     string   `json:"chain_id"`
	Txs         [][]byte `json:"txs"` // individually marshalled list of SEQ txs
	BlockNumber uint64   `json:"block_number"`
}

type ArcadiaChunk struct {
	ToB *ToBChunk `json:"tob"`
	RoB *RoBChunk `json:"rob"`

	ToBNonce uint64 `json:"tobnonce"`
}

type ArcadiaToSEQChunkMessage struct {
	ChunkID          ids.ID `json:"chunkID"`
	Epoch            uint64 `json:"epoch"`
	BuilderSignature []byte `json:"builderSignature"` // builder signs over [epochNumber, chunkID]
	// ToBChunk         *ArcadiaToBChunk `json:"toBChunk,omitempty"`
	// RoBChunk         *ArcadiaRoBChunk `json:"roBChunk,omitempty"`
	Chunk         *ArcadiaChunk `json:"chunk"`
	RemovedBitSet []byte        `json:"removedBitSet"`

	sTxs          []*chain.Transaction
	removedBitSet bitset.BitSet
	authCounts    map[uint8]int
}

type ValidatorMessage struct {
	ChunkID            ids.ID `json:"chunkId"`
	Signature          []byte `json:"signature"`
	ValidatorPublicKey []byte `json:"validatorPublicKey"`
}

type ArcadiaBlockPayload struct {
	Transactions []byte `json:"transactions"`
}

type GetBlockPayloadFromArcadia struct {
	MaxBandwidth uint64 `json:"maxBandwidth"`
	BlockNumber  uint64 `json:"blockNumber"`
}

type ChunkInterface interface {
	Marshal() ([]byte, error)
	Transactions() []*chain.Transaction
}
