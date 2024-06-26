// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"

	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
)

type VM interface {
	ChainID() ids.ID
	NetworkID() uint32
	SubnetID() ids.ID
	Tracer() trace.Tracer
	Logger() logging.Logger
	Registry() (chain.ActionRegistry, chain.AuthRegistry)
	Submit(
		ctx context.Context,
		verifySig bool,
		txs []*chain.Transaction,
	) (errs []error)
	LastAcceptedBlock() *chain.StatelessBlock
	LastL1Head() int64
	UnitPrices(context.Context) (chain.Dimensions, error)
	GetOutgoingWarpMessage(ids.ID) (*warp.UnsignedMessage, error)
	GetWarpSignatures(ids.ID) ([]*chain.WarpSignature, error)
	CurrentValidators(
		context.Context,
	) (map[ids.NodeID]*validators.GetValidatorOutput, map[string]struct{})
	GatherSignatures(context.Context, ids.ID, []byte)
	GetVerifySignatures() bool
	HasDiskBlock(height uint64) (bool, error)
	GetDiskBlock(height uint64) (*chain.StatefulBlock, error)
	GetDiskBlockResults(ctx context.Context, height uint64) ([]*chain.Result, error)
	GetDiskFeeManager(ctx context.Context, height uint64) ([]byte, error)
}
