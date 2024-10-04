// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//nolint:revive
package config

import (
	"time"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/profiler"
	"github.com/ava-labs/avalanchego/utils/units"

	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/trace"
)

type Config struct{}

func (c *Config) GetLogLevel() logging.Level                { return logging.Info }
func (c *Config) GetAuthVerificationCores() int             { return 1 }
func (c *Config) GetRootGenerationCores() int               { return 1 }
func (c *Config) GetTransactionExecutionCores() int         { return 1 }
func (c *Config) GetStateFetchConcurrency() int             { return 1 }
func (c *Config) GetMempoolSize() int                       { return 2_048 }
func (c *Config) GetMempoolSponsorSize() int                { return 32 }
func (c *Config) GetMempoolExemptSponsors() []codec.Address { return nil }
func (c *Config) GetStreamingBacklogSize() int              { return 1024 }
func (c *Config) GetIntermediateNodeCacheSize() int         { return 4 * units.GiB }
func (c *Config) GetStateIntermediateWriteBufferSize() int  { return 32 * units.MiB }
func (c *Config) GetStateIntermediateWriteBatchSize() int   { return 4 * units.MiB }
func (c *Config) GetValueNodeCacheSize() int                { return 2 * units.GiB }
func (c *Config) GetTraceConfig() *trace.Config             { return &trace.Config{Enabled: false} }
func (c *Config) GetStateSyncParallelism() int              { return 4 }
func (c *Config) GetStateSyncServerDelay() time.Duration    { return 0 } // used for testing

func (c *Config) GetParsedBlockCacheSize() int     { return 128 }
func (c *Config) GetStateHistoryLength() int       { return 256 }
func (c *Config) GetAcceptedBlockWindowCache() int { return 128 }    // 256MB at 2MB blocks
func (c *Config) GetAcceptedBlockWindow() int      { return 50_000 } // ~3.5hr with 250ms block time (100GB at 2MB)
func (c *Config) GetStateSyncMinBlocks() uint64    { return 768 }    // set to max int for archive nodes to ensure no skips
func (c *Config) GetAcceptorSize() int             { return 64 }
func (c *Config) GetStoreBlockResultsOnDisk() bool { return true }

func (c *Config) GetContinuousProfilerConfig() *profiler.Config {
	return &profiler.Config{Enabled: false}
}
func (c *Config) GetVerifyAuth() bool                    { return true }
func (c *Config) GetTargetBuildDuration() time.Duration  { return 100 * time.Millisecond }
func (c *Config) GetProcessingBuildSkip() int            { return 16 }
func (c *Config) GetTargetGossipDuration() time.Duration { return 20 * time.Millisecond }
func (c *Config) GetBlockCompactionFrequency() int       { return 32 } // 64 MB of deletion if 2 MB blocks
func (c *Config) GetETHL1RPC() string                    { return "http://localhost:8545" }
func (c *Config) GetETHL1WS() string                     { return "ws://localhost:8546" }
func (c *Config) GetAnchorURL() string                   { return "" }
func (c *Config) GetAnchorManager() string {
	return "0x987647e360d340fa0224bf2c2c11919fd0d90e3797c9869994c22a52b4d901fd9450795a9a9aa9562639359ed0fdc19e"
} // default bls pubkey for anchor manager
