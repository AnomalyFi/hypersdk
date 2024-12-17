package actions

import (
	"encoding/binary"

	"github.com/AnomalyFi/hypersdk/consts"
)

const (
	RollupRegisteryPrefix = 0xf0
	RollupInfoPrefix      = 0xf2
	ArcadiaBidPrefix      = 0xf3
	EpochExitsPrefix      = 0xf4
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
	copy(k[1:], namespace)
	binary.BigEndian.PutUint16(k[1+len(namespace):], RollupInfoChunks)
	return k
}

func ArcadiaBidKey(epoch uint64) []byte {
	k := make([]byte, 1+8+consts.Uint16Len)
	k[0] = ArcadiaBidPrefix
	binary.BigEndian.PutUint64(k[1:], epoch)
	binary.BigEndian.PutUint16(k[9:], ArcadiaBidChunks)
	return k
}

func EpochExitsKey(epoch uint64) []byte {
	k := make([]byte, 1+8+consts.Uint16Len)
	k[0] = EpochExitsPrefix
	binary.BigEndian.PutUint64(k[1:], epoch)
	binary.BigEndian.PutUint16(k[9:], EpochExitsChunks)
	return k
}
