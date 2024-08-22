package actions

import (
	"encoding/binary"

	"github.com/AnomalyFi/hypersdk/consts"
)

const (
	AnchorRegisteryPrefix = 0xf0
	AnchorPrefix          = 0xf1
)

func AnchorRegistryKey() []byte {
	// state key must >= 2 bytes
	k := make([]byte, 1+consts.Uint16Len)
	k[0] = AnchorRegisteryPrefix
	binary.BigEndian.PutUint16(k[1:], AnchorChunks) //TODO: update the BalanceChunks to AnchorChunks
	return k
}

func AnchorKey(namespace []byte) []byte {
	k := make([]byte, 1+len(namespace)+consts.Uint16Len)
	k[0] = AnchorPrefix
	copy(k[1:], namespace[:])
	binary.BigEndian.PutUint16(k[1+len(namespace):], AnchorChunks) //TODO: update the BalanceChunks to AnchorChunks
	return k
}
