package actions

import (
	"encoding/binary"

	"github.com/AnomalyFi/hypersdk/consts"
)

const (
	AnchorRegisteryPrefix = 0xf0
	ArcadiaRegistryPrefix = 0xf1
	RollupInfoPrefix      = 0xf2
)

func AnchorRegistryKey() []byte {
	k := make([]byte, 1+consts.Uint16Len)
	k[0] = AnchorRegisteryPrefix
	binary.BigEndian.PutUint16(k[1:], AnchorRegistryChunks)
	return k
}

func ArcadiaRegistryKey() []byte {
	k := make([]byte, 1+consts.Uint16Len)
	k[0] = ArcadiaRegistryPrefix
	binary.BigEndian.PutUint16(k[1:], ArcadiaRegistryChunks)
	return k
}

func RollupInfoKey(namespace []byte) []byte {
	k := make([]byte, 1+len(namespace)+consts.Uint16Len)
	k[0] = RollupInfoPrefix
	copy(k[1:], namespace[:])
	binary.BigEndian.PutUint16(k[1+len(namespace):], RollupInfoChunks)
	return k
}
