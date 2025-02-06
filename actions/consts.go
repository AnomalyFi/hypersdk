package actions

import "errors"

const (
	RollupRegisterComputeUnits = 5
)

const (
	// pre-defined actions starts from 0xf0
	RollupRegisterID uint8 = 0xf0
	EpochExitID      uint8 = 0xf1
)

// The length of data stored at RollupRegistryKey depends on number of rollups registered.
// Each chunk gives a state storage of 64 bytes.
// Its safe to limit the data of state storage for RollupRegistryKey to atleast 3 KiB.
const (
	RollupRegistryChunks uint16 = 3 * 16
	EpochExitsChunks     uint16 = 3 * 16
	ArcadiaBidChunks     uint16 = 3
)

// 2 * AddressLen(33) + 1 MaxNamespaceLen(32) + BLS Pubkey Length(48) + 2*Epoch(8) = 162
const RollupInfoChunks uint16 = 4

const (
	TransferID uint8 = 0
	MsgID      uint8 = 1
)

var (
	ErrAuctionWinnerValueWrongLength = errors.New("got auction winner value with wrong length, wanted: 152")
)
