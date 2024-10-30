package actions

const (
	AnchorRegisterComputeUnits  = 5
	ArcadiaRegisterComputeUnits = 5
)

const (
	// pre-defined actions starts from 0xf0
	AnchorRegisterID  uint8 = 0xf0
	ArcadiaRegisterID uint8 = 0xf1
)

// The length of data stored at AnchorRegistryKey depends on number of rollups registered.
// Each chunk gives a state storage of 64 bytes.
// Its safe to limit the data of state storage for AnchorRegistryKey to atleast 3 KiB.
const (
	AnchorRegistryChunks  uint16 = 3 * 16
	ArcadiaRegistryChunks uint16 = 3 * 16
)

// 2 AddressLen* 33 + 1 MaxNameSpaceLen *  32 = 98 bytes
const RollupInfoChunks uint16 = 2
