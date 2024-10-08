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

	smath "github.com/ava-labs/avalanchego/utils/math"
)

var _ chain.Action = (*MintAsset)(nil)

type MintAsset struct {
	// To is the recipient of the [Value].
	To codec.Address `json:"to"`

	// Asset is the [ActionID] that created the asset.
	Asset ids.ID `json:"asset"`

	// Number of assets to mint to [To].
	Value uint64 `json:"value"`

	// TODO: add boolean to indicate whether sender will
	// create recipient account
}

func (*MintAsset) GetTypeID() uint8 {
	return mintAssetID
}

func (m *MintAsset) StateKeys(codec.Address, ids.ID) state.Keys {
	return state.Keys{
		string(storage.AssetKey(m.Asset)):         state.Read | state.Write,
		string(storage.BalanceKey(m.To, m.Asset)): state.All,
	}
}

func (*MintAsset) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.AssetChunks, storage.BalanceChunks}
}

func (m *MintAsset) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) ([][]byte, error) {
	if m.Asset == ids.Empty {
		return nil, ErrOutputAssetIsNative
	}
	if m.Value == 0 {
		return nil, ErrOutputValueZero
	}
	exists, symbol, decimals, metadata, supply, owner, err := storage.GetAsset(ctx, mu, m.Asset)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrOutputAssetMissing
	}
	if owner != actor {
		return nil, ErrOutputWrongOwner
	}
	newSupply, err := smath.Add64(supply, m.Value)
	if err != nil {
		return nil, err
	}
	if err := storage.SetAsset(ctx, mu, m.Asset, symbol, decimals, metadata, newSupply, actor); err != nil {
		return nil, err
	}
	if err := storage.AddBalance(ctx, mu, m.To, m.Asset, m.Value, true); err != nil {
		return nil, err
	}
	return nil, nil
}

func (*MintAsset) ComputeUnits(chain.Rules) uint64 {
	return MintAssetComputeUnits
}

func (*MintAsset) Size() int {
	return codec.AddressLen + ids.IDLen + consts.Uint64Len
}

func (m *MintAsset) Marshal(p *codec.Packer) {
	p.PackAddress(m.To)
	p.PackID(m.Asset)
	p.PackUint64(m.Value)
}

func UnmarshalMintAsset(p *codec.Packer) (chain.Action, error) {
	var mint MintAsset
	p.UnpackAddress(&mint.To)
	p.UnpackID(true, &mint.Asset) // empty ID is the native asset
	mint.Value = p.UnpackUint64(true)
	return &mint, p.Err()
}

func (*MintAsset) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}
