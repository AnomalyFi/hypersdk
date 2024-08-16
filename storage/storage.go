// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/AnomalyFi/hypersdk/filedb"
	"github.com/AnomalyFi/hypersdk/pebble"
	"github.com/AnomalyFi/hypersdk/utils"
	"github.com/AnomalyFi/hypersdk/vilmo"
)

// TODO: add option to use a single DB with prefixes to allow for atomic writes
// TODO: should filedb and blobdb wrapped with database.Database interface?
// compaction is not necessary in blobdb and filebd, handles compaction in its own way.
func New(chainDataDir string, logger logging.Logger, gatherer metrics.MultiGatherer) (database.Database, *filedb.FileDB, *vilmo.Vilmo, database.Database, error) {
	// TODO: tune Pebble config based on each sub-db focus
	cfg := pebble.NewDefaultConfig()
	vmPath, err := utils.InitSubDirectory(chainDataDir, vm)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	vmDB, vmDBRegistry, err := pebble.New(vmPath, cfg)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	if gatherer != nil {
		if err := gatherer.Register(vm, vmDBRegistry); err != nil {
			return nil, nil, nil, nil, err
		}
	}

	blobPath, err := utils.InitSubDirectory(chainDataDir, blob)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	blobDBCfg := filedb.NewDefaultConfig()
	blobDB := filedb.New(blobPath, blobDBCfg)

	statePath, err := utils.InitSubDirectory(chainDataDir, state)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	stateCfg := vilmo.NewDefaultConfig()
	stateDB, _, err := vilmo.New(logger, statePath, stateCfg)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	metaPath, err := utils.InitSubDirectory(chainDataDir, metadata)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	metaDB, metaDBRegistry, err := pebble.New(metaPath, cfg)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if gatherer != nil {
		if err := gatherer.Register(metadata, metaDBRegistry); err != nil {
			return nil, nil, nil, nil, err
		}
	}
	return vmDB, blobDB, stateDB, metaDB, nil
}
