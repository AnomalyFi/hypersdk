package actions

import (
	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/utils"
	"github.com/ava-labs/avalanchego/ids"
)

type RollupInfo struct {
	FeeRecipient        codec.Address `json:"feeRecipient"`
	Namespace           []byte        `json:"namespace"`
	AuthoritySEQAddress codec.Address `json:"authoritySEQAddress"`
}

func NewRollupInfo(namespace []byte, feeRecipient codec.Address, authoritySEQAddress codec.Address) *RollupInfo {
	return &RollupInfo{
		FeeRecipient:        feeRecipient,
		Namespace:           namespace,
		AuthoritySEQAddress: authoritySEQAddress,
	}
}

func (a *RollupInfo) ID() ids.ID {
	return utils.ToID([]byte(a.Namespace))
}

func (a *RollupInfo) Size() int {
	return 2*codec.AddressLen + codec.BytesLen(a.Namespace)
}

func (a *RollupInfo) Marshal(p *codec.Packer) error {
	p.PackBytes(a.Namespace)
	p.PackAddress(a.FeeRecipient)
	p.PackAddress(a.AuthoritySEQAddress)
	if err := p.Err(); err != nil {
		return err
	}

	return nil
}

func UnmarshalRollupInfo(p *codec.Packer) (*RollupInfo, error) {
	ret := new(RollupInfo)

	p.UnpackBytes(32, false, &ret.Namespace) // TODO: set limit for the length of namespace bytes
	p.UnpackAddress(&ret.FeeRecipient)
	p.UnpackAddress(&ret.AuthoritySEQAddress)
	if err := p.Err(); err != nil {
		return nil, err
	}
	return ret, nil
}
