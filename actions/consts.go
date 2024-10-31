package actions

const (
	RollupRegisterComputeUnits = 5
)

const (
	// pre-defined actions starts from 0xf0
	RollupRegisterID uint8 = 0xf0
)

// The length of data stored at AnchorRegistryKey depends on number of rollups registered.
// Each chunk gives a state storage of 64 bytes.
// Its safe to limit the data of state storage for AnchorRegistryKey to atleast 3 KiB.
const (
	RollupRegistryChunks uint16 = 3 * 16
)

// 2 AddressLen* 33 + 1 MaxNameSpaceLen *  32 = 98 bytes
const RollupInfoChunks uint16 = 2
