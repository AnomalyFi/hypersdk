package arcadia

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"

	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/crypto/bls"
	"github.com/AnomalyFi/hypersdk/workers"
)

type VM interface {
	chain.AuthVM
	chain.Parser

	RecordChunksReceived()
	RecordChunksRejected()
	RecordChunksAccepted()
	RecordValidTxsInChunksReceived(int)
	RecordChunkProcessDuration(time.Duration)

	NodeID() ids.NodeID
	Signer() *bls.PublicKey
	Sign(msg *warp.UnsignedMessage) ([]byte, error)
	AuthVerifiers() workers.Workers
	Rules(int64) chain.Rules
	Registry() (chain.ActionRegistry, chain.AuthRegistry)
	GetVerifyAuth() bool
	GetChunkCores() int
	GetPreconfIssueCores() int
	GetChunkProcessingBackLog() int
	AddToArcadiaAuthVerifiedTxs(txs []*chain.Transaction)
	ReadState(ctx context.Context, keys [][]byte) ([][]byte, []error)
	GetCurrentEpoch() uint64
	NetworkID() uint32
	ChainID() ids.ID
	Logger() logging.Logger
	StopChan() chan struct{}
}
