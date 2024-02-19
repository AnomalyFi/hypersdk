// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

// Note: Registry will error during initialization if a duplicate ID is assigned. We explicitly assign IDs to avoid accidental remapping.
const (
	transferID      uint8 = 0
	NMTTestActionID uint8 = 1
)

var DefaultNMTNamespace = make([]byte, 8)

const (
	TransferComputeUnits = 1
)
