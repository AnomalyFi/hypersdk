package arcadia

import (
	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/crypto/bls"
	"github.com/ava-labs/avalanchego/ids"
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
	RollupIDs             []string             `json:"rollupIDs"`
	RollupIDToBlockNumber map[string]uint64    `json:"rollupIDToBlockNumber"`
	Txs                   []*chain.Transaction `json:"txs"`
	Nonce                 uint64               `json:"nonce"`
}

type ArcadiaRoBChunk struct {
	RollupID    string               `json:"rollupID"`
	BlockNumber uint64               `json:"blockNumber"`
	Txs         []*chain.Transaction `json:"txs"`
	Nonce       uint64               `json:"nonce"` // Do we need nonce for rob chunk?
}

type ArcadiaChunk struct {
	ChunkID          ids.ID           `json:"chunkId"`
	Epoch            uint64           `json:"epoch"`
	BuilderSignature []byte           `json:"builderSignature"` // builder signs over [epochNumber, chunkID]
	ToBChunk         *ArcadiaToBChunk `json:"toBChunk,omitempty"`
	RoBChunk         *ArcadiaRoBChunk `json:"roBChunk,omitempty"`
}

type ValidatorMessage struct {
	ChunkID            ids.ID `json:"chunkId"`
	Signature          []byte `json:"signature"`
	ValidatorPublicKey []byte `json:"validatorPublicKey"`
}

type ArcadiaBlockPayload struct {
	Transactions []byte `json:"transactions"`
	// @todo should tob and rob transactions get seperated?
}

type GetBlockPayloadFromArcadia struct {
	MaxBandwidth uint64 `json:"maxBandwidth"`
	// @todo should seq validator public key be sent?
}

type ChunkInterface interface {
	Marshal() ([]byte, error)
	Transactions() []*chain.Transaction
}
