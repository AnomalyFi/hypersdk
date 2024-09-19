// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"

	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/fees"
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
	UnitPrices(context.Context) (fees.Dimensions, error)
	NameSpacesPrice(ctx context.Context, namespace []string) ([]uint64, error)
	Proposers(ctx context.Context, diff int, depth int) (set.Set[ids.NodeID], error)
	CurrentValidators(
		context.Context,
	) (map[ids.NodeID]*validators.GetValidatorOutput, map[string]struct{})
	HasDiskBlock(height uint64) (bool, error)
	GetDiskBlock(ctx context.Context, height uint64) (*chain.StatelessBlock, error)
	GetDiskBlockResults(ctx context.Context, height uint64) ([]*chain.Result, error)
	GetDiskFeeManager(ctx context.Context, height uint64) ([]byte, error)
	GetVerifyAuth() bool
	ReplaceAnchor(url string) bool
}
