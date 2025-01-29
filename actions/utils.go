package actions

import (
	"encoding/binary"

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

func UnpackEpochExitsInfo(raw []byte) (*EpochExitInfo, error) {
	p := codec.NewReader(raw, consts.NetworkSizeLimit)
	return UnmarshalEpochExitsInfo(p)
}

func PackArcadiaAuctionWinner(bidPrice uint64, winnerPubkey []byte, winnerSig []byte) []byte {
	v := make([]byte, consts.Uint64Len+BLSPubkeyLength+BLSSignatureLength)
	binary.BigEndian.PutUint64(v, bidPrice)
	copy(v[consts.Uint64Len:], winnerPubkey)
	copy(v[consts.Uint64Len+BLSPubkeyLength:], winnerSig)

	return v
}

func UnpackArcadiaAuctionWinner(value []byte) (uint64, []byte, []byte, error) {
	if len(value) != consts.Uint64Len+BLSPubkeyLength+BLSSignatureLength {
		return 0, nil, nil, ErrAuctionWinnerValueNotCorrect
	}

	bidPrice := binary.BigEndian.Uint64(value[:consts.Uint64Len])
	pubkey := value[consts.Uint64Len : consts.Uint64Len+BLSPubkeyLength]
	sig := value[consts.Uint64Len+BLSPubkeyLength:]

	return bidPrice, pubkey, sig, nil
}
