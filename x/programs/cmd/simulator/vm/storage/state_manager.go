// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"

	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/state"
	"github.com/ava-labs/avalanchego/ids"
)

type StateManager struct{}

func (m *StateManager) SponsorStateKeys(_ codec.Address) state.Keys {
	// TODO implement me
	panic("implement me")
}

func (m *StateManager) CanDeduct(_ context.Context, _ codec.Address, _ state.Immutable, _ uint64) error {
	// TODO implement me
	panic("implement me")
}

func (m *StateManager) Deduct(_ context.Context, _ codec.Address, _ state.Mutable, _ uint64) error {
	// TODO implement me
	panic("implement me")
}

func (m *StateManager) Refund(_ context.Context, _ codec.Address, _ state.Mutable, _ uint64) error {
	// TODO implement me
	panic("implement me")
}

func (*StateManager) HeightKey() []byte {
	return HeightKey()
}

func (*StateManager) TimestampKey() []byte {
	return TimestampKey()
}

func (*StateManager) FeeKey() []byte {
	return FeeKey()
}

func (*StateManager) IncomingWarpKeyPrefix(sourceChainID ids.ID, msgID ids.ID) []byte {
	return IncomingWarpKeyPrefix(sourceChainID, msgID)
}

func (*StateManager) OutgoingWarpKeyPrefix(txID ids.ID) []byte {
	return OutgoingWarpKeyPrefix(txID)
}
