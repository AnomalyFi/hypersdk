// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"slices"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	hactions "github.com/ava-labs/hypersdk/actions"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*AnchorRegister)(nil)

const (
	CreateAnchor = iota
	DeleteAnchor
	UpdateAnchor
)

type AnchorRegister struct {
	Info      hactions.AnchorInfo `json:"info"`
	Namespace []byte              `json:"namespace"`
	OpCode    int                 `json:"opcode"`
}

func (*AnchorRegister) GetTypeID() uint8 {
	return hactions.AnchorRegisterID
}

func (t *AnchorRegister) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.AnchorKey(t.Namespace)): state.All,
		string(storage.AnchorRegistryKey()):    state.All,
	}
}

func (*AnchorRegister) StateKeyChunks() []uint16 {
	return []uint16{hactions.AnchorChunks, hactions.AnchorChunks}
}

func (*AnchorRegister) OutputsWarpMessage() bool {
	return false
}

func (t *AnchorRegister) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
	_ bool,
) (bool, []byte, *warp.UnsignedMessage, error) {
	if !bytes.Equal(t.Namespace, t.Info.Namespace) {
		return false, utils.ErrBytes(fmt.Errorf("provided namespace(%s) not match to the one(%s) in anchor info", hex.EncodeToString(t.Namespace), hex.EncodeToString(t.Info.Namespace))), nil, nil
	}

	namespaces, _, err := storage.GetAnchors(ctx, mu)
	if err != nil {
		return false, utils.ErrBytes(err), nil, nil
	}

	switch t.OpCode {
	case CreateAnchor:
		namespaces = append(namespaces, t.Namespace)
		if err := storage.SetAnchor(ctx, mu, t.Namespace, &t.Info); err != nil {
			return false, utils.ErrBytes(err), nil, nil
		}
	case UpdateAnchor:
		namespaces = append(namespaces, t.Namespace)
		if err := storage.SetAnchor(ctx, mu, t.Namespace, &t.Info); err != nil {
			return false, utils.ErrBytes(err), nil, nil
		}
	case DeleteAnchor:
		nsIdx := -1
		for i, ns := range namespaces {
			if bytes.Equal(t.Namespace, ns) {
				nsIdx = i
				break
			}
		}
		namespaces = slices.Delete(namespaces, nsIdx, nsIdx+1)
		if err := storage.DelAnchor(ctx, mu, t.Namespace); err != nil {
			return false, utils.ErrBytes(err), nil, nil
		}
	default:
		return false, utils.ErrBytes(fmt.Errorf("op code(%d) not supported", t.OpCode)), nil, nil
	}

	if err := storage.SetAnchors(ctx, mu, namespaces); err != nil {
		return false, utils.ErrBytes(err), nil, nil
	}

	return true, nil, nil, nil
}

func (*AnchorRegister) ComputeUnits(chain.Rules) uint64 {
	return AnchorRegisterComputeUnits
}

func (t *AnchorRegister) Size() int {
	return codec.BytesLen(t.Namespace) + codec.AddressLen + consts.BoolLen
}

func (t *AnchorRegister) Marshal(p *codec.Packer) {
	t.Info.Marshal(p)
	p.PackBytes(t.Namespace)
	p.PackInt(t.OpCode)
}

func UnmarshalAnchorRegister(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var anchorReg AnchorRegister
	info, err := hactions.UnmarshalAnchorInfo(p)
	if err != nil {
		return nil, err
	}
	anchorReg.Info = *info
	p.UnpackBytes(-1, false, &anchorReg.Namespace)
	anchorReg.OpCode = p.UnpackInt(false)
	return &anchorReg, nil
}

func (*AnchorRegister) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

func (*AnchorRegister) NMTNamespace() []byte {
	return defaultNMTNamespace // TODO: mark this the same to registering namespace?
}