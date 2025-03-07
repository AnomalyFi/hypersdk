// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"fmt"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/consts"
)

const BaseSize = consts.Uint64Len*2 + ids.IDLen

type Base struct {
	// Timestamp is the expiry of the transaction (inclusive). Once this time passes and the
	// transaction is not included in a block, it is safe to regenerate it.
	Timestamp int64 `json:"timestamp"`

	// ChainID protects against replay attacks on different VM instances.
	ChainID ids.ID `json:"chainId"`

	// PriorityFee is the fee the user is willing to pay to have the transaction prioritized during block building.
	// PriorityFee is charged in excess to the multidimensional fee and feeMarket fee.
	// There can be zero priority fee specified for a transaction.
	PriorityFee uint64 `json:"priorityFee"`

	// MaxFee is the max fee the user will pay for the transaction to be executed. The chain
	// will charge anything up to this price if the transaction makes it on-chain.
	//
	// If the fee is too low to pay all fees, the transaction will be dropped.
	MaxFee uint64 `json:"maxFee"`
}

func (b *Base) Execute(chainID ids.ID, r Rules, timestamp int64) error {
	switch {
	case b.Timestamp%consts.MillisecondsPerSecond != 0:
		// TODO: make this modulus configurable
		return fmt.Errorf("%w: timestamp=%d", ErrMisalignedTime, b.Timestamp)
	case b.Timestamp < timestamp: // tx: 100 block: 110
		return ErrTimestampTooLate
	case b.Timestamp > timestamp+r.GetValidityWindow(): // tx: 100 block 10
		return ErrTimestampTooEarly
	case b.ChainID != chainID:
		return ErrInvalidChainID
	default:
		return nil
	}
}

func (b *Base) ArcadiaExecute(chainID ids.ID, r Rules, timestamp int64) error {
	switch {
	case b.Timestamp%consts.MillisecondsPerSecond != 0:
		// TODO: make this modulus configurable
		return fmt.Errorf("%w: timestamp=%d", ErrMisalignedTime, b.Timestamp)
	case b.Timestamp < timestamp: // tx: 100 block: 110
		return ErrTimestampTooLate
	case b.Timestamp > timestamp+r.GetValidityWindow(): // tx: 100 block 10
		return ErrTimestampTooEarly
	case b.Timestamp < timestamp+r.GetValidityWindow()/2:
		return ErrExpiryTooSoon
	case b.ChainID != chainID:
		return ErrInvalidChainID
	default:
		return nil
	}
}

func (*Base) Size() int {
	return BaseSize
}

func (b *Base) Marshal(p *codec.Packer) {
	p.PackInt64(b.Timestamp)
	p.PackID(b.ChainID)
	p.PackUint64(b.PriorityFee)
	p.PackUint64(b.MaxFee)
}

func UnmarshalBase(p *codec.Packer) (*Base, error) {
	var base Base
	base.Timestamp = p.UnpackInt64(true)
	if base.Timestamp%consts.MillisecondsPerSecond != 0 {
		// TODO: make this modulus configurable
		return nil, fmt.Errorf("%w: timestamp=%d", ErrMisalignedTime, base.Timestamp)
	}
	p.UnpackID(true, &base.ChainID)
	base.PriorityFee = p.UnpackUint64(false)
	base.MaxFee = p.UnpackUint64(true)
	return &base, p.Err()
}
