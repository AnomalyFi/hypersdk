package actions

import (
	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/utils"
	"github.com/ava-labs/avalanchego/ids"
)

// TODO: do we need to rename it to `RollupInfo`
type AnchorInfo struct {
	FeeRecipient codec.Address `json:"feeRecipient"`
	Namespace    []byte        `json:"namespace"`
}

func NewAnchorInfo(namespace []byte, feeRecipient codec.Address) *AnchorInfo {
	return &AnchorInfo{
		FeeRecipient: feeRecipient,
		Namespace:    namespace,
	}
}

func (a *AnchorInfo) ID() ids.ID {
	return utils.ToID([]byte(a.Namespace))
}

func (a *AnchorInfo) Size() int {
	return codec.AddressLen + codec.BytesLen(a.Namespace)
}

func (a *AnchorInfo) Marshal(p *codec.Packer) error {
	p.PackBytes(a.Namespace)
	p.PackAddress(a.FeeRecipient)

	if err := p.Err(); err != nil {
		return err
	}

	return nil
}

func UnmarshalAnchorInfo(p *codec.Packer) (*AnchorInfo, error) {
	ret := new(AnchorInfo)

	// p := codec.NewReader(raw, consts.NetworkSizeLimit)
	p.UnpackBytes(32, false, &ret.Namespace) // TODO: set limit for the length of namespace bytes
	p.UnpackAddress(&ret.FeeRecipient)

	if err := p.Err(); err != nil {
		return nil, err
	}
	return ret, nil
}
