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

type ArcadiaChunk struct {
	ChunkID          ids.ID           `json:"chunkID"`
	Epoch            uint64           `json:"epoch"`
	BuilderSignature []byte           `json:"builderSignature"` // builder signs over [epochNumber, chunkID]
	ToBChunk         *ArcadiaToBChunk `json:"toBChunk,omitempty"`
	RoBChunk         *ArcadiaRoBChunk `json:"roBChunk,omitempty"`

	authCounts map[uint8]int
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
