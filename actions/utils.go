package actions

import (
	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/consts"
	"github.com/AnomalyFi/hypersdk/crypto/bls"
)

func PackNamespaces(namespaces [][]byte) ([]byte, error) {
	p := codec.NewWriter(len(namespaces)*8, consts.NetworkSizeLimit)
	p.PackInt(len(namespaces))
	for _, ns := range namespaces {
		p.PackBytes(ns)
	}
	return p.Bytes(), p.Err()
}

func UnpackNamespaces(raw []byte) ([][]byte, error) {
	p := codec.NewReader(raw, consts.NetworkSizeLimit)
	nsLen := p.UnpackInt(false)
	namespaces := make([][]byte, 0, nsLen)
	for i := 0; i < nsLen; i++ {
		ns := make([]byte, 0, 8)
		p.UnpackBytes(-1, false, &ns)
		namespaces = append(namespaces, ns)
	}

	return namespaces, p.Err()
}

func UnpackBidderPublicKeyFromStateData(raw []byte) (*bls.PublicKey, error) {
	pubKeyBytes := raw[8 : 8+48]
	pubkey, err := bls.PublicKeyFromBytes(pubKeyBytes)
	if err != nil {
		return nil, err
	}
	return pubkey, nil
}
