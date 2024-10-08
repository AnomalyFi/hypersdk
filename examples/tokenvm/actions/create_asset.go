// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/consts"
	"github.com/AnomalyFi/hypersdk/examples/tokenvm/storage"
	"github.com/AnomalyFi/hypersdk/state"
)

var _ chain.Action = (*CreateAsset)(nil)

type CreateAsset struct {
	Symbol   []byte `json:"symbol"`
	Decimals uint8  `json:"decimals"`
	Metadata []byte `json:"metadata"`
}

func (*CreateAsset) GetTypeID() uint8 {
	return createAssetID
}

func (*CreateAsset) StateKeys(_ codec.Address, actionID ids.ID) state.Keys {
	return state.Keys{
		string(storage.AssetKey(actionID)): state.Allocate | state.Write,
	}
}

func (*CreateAsset) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.AssetChunks}
}

func (c *CreateAsset) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	actionID ids.ID,
) ([][]byte, error) {
	if len(c.Symbol) == 0 {
		return nil, ErrOutputSymbolEmpty
	}
	if len(c.Symbol) > MaxSymbolSize {
		return nil, ErrOutputSymbolTooLarge
	}
	if c.Decimals > MaxDecimals {
		return nil, ErrOutputDecimalsTooLarge
	}
	if len(c.Metadata) == 0 {
		return nil, ErrOutputMetadataEmpty
	}
	if len(c.Metadata) > MaxMetadataSize {
		return nil, ErrOutputMetadataTooLarge
	}
	// It should only be possible to overwrite an existing asset if there is
	// a hash collision.
	if err := storage.SetAsset(ctx, mu, actionID, c.Symbol, c.Decimals, c.Metadata, 0, actor); err != nil {
		return nil, err
	}
	return nil, nil
}

func (*CreateAsset) ComputeUnits(chain.Rules) uint64 {
	return CreateAssetComputeUnits
}

func (c *CreateAsset) Size() int {
	// TODO: add small bytes (smaller int prefix)
	return codec.BytesLen(c.Symbol) + consts.Uint8Len + codec.BytesLen(c.Metadata)
}

func (c *CreateAsset) Marshal(p *codec.Packer) {
	p.PackBytes(c.Symbol)
	p.PackByte(c.Decimals)
	p.PackBytes(c.Metadata)
}

func UnmarshalCreateAsset(p *codec.Packer) (chain.Action, error) {
	var create CreateAsset
	p.UnpackBytes(MaxSymbolSize, true, &create.Symbol)
	create.Decimals = p.UnpackByte()
	p.UnpackBytes(MaxMetadataSize, true, &create.Metadata)
	return &create, p.Err()
}

func (*CreateAsset) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
