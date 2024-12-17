package actions

import (
	"github.com/ava-labs/avalanchego/ids"

	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/consts"
	"github.com/AnomalyFi/hypersdk/utils"
)

type RollupInfo struct {
	FeeRecipient        codec.Address `json:"feeRecipient"`
	Namespace           []byte        `json:"namespace"`
	AuthoritySEQAddress codec.Address `json:"authoritySEQAddress"`
	SequencerPublicKey  []byte        `json:"sequencerPublicKey"`
	StartEpoch          uint64        `json:"startEpoch"`
}

func NewRollupInfo(namespace []byte, feeRecipient codec.Address, authoritySEQAddress codec.Address, sequencerPublicKey []byte, startEpoch uint64) *RollupInfo {
	return &RollupInfo{
		FeeRecipient:        feeRecipient,
		Namespace:           namespace,
		AuthoritySEQAddress: authoritySEQAddress,
		SequencerPublicKey:  sequencerPublicKey,
		StartEpoch:          startEpoch,
	}
}

func (a *RollupInfo) ID() ids.ID {
	return utils.ToID(a.Namespace)
}

func (a *RollupInfo) Size() int {
	return 2*codec.AddressLen + codec.BytesLen(a.Namespace) + codec.BytesLen(a.SequencerPublicKey) + consts.Uint64Len
}

func (a *RollupInfo) Marshal(p *codec.Packer) {
	p.PackBytes(a.Namespace)
	p.PackAddress(a.FeeRecipient)
	p.PackAddress(a.AuthoritySEQAddress)
	p.PackBytes(a.SequencerPublicKey)
	p.PackUint64(a.StartEpoch)
}

func UnmarshalRollupInfo(p *codec.Packer) (*RollupInfo, error) {
	ret := new(RollupInfo)

	p.UnpackBytes(32, true, &ret.Namespace) // TODO: set limit for the length of namespace bytes
	p.UnpackAddress(&ret.FeeRecipient)
	p.UnpackAddress(&ret.AuthoritySEQAddress)
	p.UnpackBytes(48, true, &ret.SequencerPublicKey)
	ret.StartEpoch = p.UnpackUint64(false)
	if err := p.Err(); err != nil {
		return nil, err
	}
	return ret, nil
}

// TODO need to fix this to be a rollup exiting the epoch
// The way the mapping needs to work is that we have a list of all the epochs
// and then we are updating the epoch's when a rollup opts out we can append the namespace there.
type EpochInfo struct {
	Epoch     uint64 `json:"epoch"`
	Namespace []byte `json:"namespace"`
}

type EpochExitInfo struct {
	Exits []*EpochInfo `json:"exits"`
}

func (e *EpochExitInfo) Marshal(p *codec.Packer) {
	p.PackInt(len(e.Exits))
	for _, exit := range e.Exits {
		p.PackBytes(exit.Namespace)
		p.PackUint64(exit.Epoch)
	}
}

func UnmarshalEpochExitsInfo(p *codec.Packer) (*EpochExitInfo, error) {
	count := p.UnpackInt(true)
	exits := make([]*EpochInfo, count)
	for i := 0; i < count; i++ {
		ret := new(EpochInfo)
		p.UnpackBytes(32, false, &ret.Namespace)
		ret.Epoch = p.UnpackUint64(true)

		if err := p.Err(); err != nil {
			return nil, err
		}

		exits[i] = ret
	}
	return &EpochExitInfo{Exits: exits}, p.Err()
}

func NewEpochInfo(namespace []byte, epoch uint64) *EpochInfo {
	return &EpochInfo{
		Epoch:     epoch,
		Namespace: namespace,
	}
}

func (a *EpochInfo) ID() ids.ID {
	return utils.ToID(a.Namespace)
}

func (a *EpochInfo) Size() int {
	return consts.Uint64Len + codec.BytesLen(a.Namespace)
}

func (a *EpochInfo) Marshal(p *codec.Packer) {
	p.PackBytes(a.Namespace)
	p.PackUint64(a.Epoch)
}

func UnmarshalEpochInfo(p *codec.Packer) (*EpochInfo, error) {
	ret := new(EpochInfo)

	p.UnpackBytes(32, false, &ret.Namespace) // TODO: set limit for the length of namespace bytes
	ret.Epoch = p.UnpackUint64(true)

	if err := p.Err(); err != nil {
		return nil, err
	}
	return ret, nil
}
