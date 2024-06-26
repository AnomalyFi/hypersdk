// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

import (
	"context"

	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/logging"
)

type VM interface {
	StopChan() chan struct{}
	EngineChan() chan<- common.Message
	PreferredBlock(context.Context) (*chain.StatelessBlock, error)
	Logger() logging.Logger
	Mempool() chain.Mempool
	Rules(int64) chain.Rules
}
