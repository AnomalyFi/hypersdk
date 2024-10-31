package actions

import (
	"encoding/binary"

	"github.com/AnomalyFi/hypersdk/consts"
)

const (
	RollupRegisteryPrefix = 0xf0
	RollupInfoPrefix      = 0xf2
)

func RollupRegistryKey() []byte {
	k := make([]byte, 1+consts.Uint16Len)
	k[0] = RollupRegisteryPrefix
	binary.BigEndian.PutUint16(k[1:], RollupRegistryChunks)
	return k
}

func RollupInfoKey(namespace []byte) []byte {
	k := make([]byte, 1+len(namespace)+consts.Uint16Len)
	k[0] = RollupInfoPrefix
	copy(k[1:], namespace[:])
	binary.BigEndian.PutUint16(k[1+len(namespace):], RollupInfoChunks)
	return k
}
