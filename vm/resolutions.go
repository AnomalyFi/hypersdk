// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"go.uber.org/zap"

	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/crypto/bls"
	"github.com/AnomalyFi/hypersdk/executor"
	"github.com/AnomalyFi/hypersdk/fees"
	"github.com/AnomalyFi/hypersdk/vilmo"
)

var (
	_ chain.VM                           = (*VM)(nil)
	_ block.ChainVM                      = (*VM)(nil)
	_ block.BuildBlockWithContextChainVM = (*VM)(nil)
)

func (vm *VM) ChainID() ids.ID {
	return vm.snowCtx.ChainID
}

func (vm *VM) NetworkID() uint32 {
	return vm.snowCtx.NetworkID
}

func (vm *VM) SubnetID() ids.ID {
	return vm.snowCtx.SubnetID
}

func (vm *VM) ValidatorState() validators.State {
	return vm.snowCtx.ValidatorState
}

func (vm *VM) Registry() (chain.ActionRegistry, chain.AuthRegistry) {
	return vm.actionRegistry, vm.authRegistry
}

func (vm *VM) Tracer() trace.Tracer {
	return vm.tracer
}

func (vm *VM) Logger() logging.Logger {
	return vm.snowCtx.Log
}

func (vm *VM) Rules(t int64) chain.Rules {
	return vm.c.Rules(t)
}

func (vm *VM) LastAcceptedBlock() *chain.StatelessBlock {
	return vm.lastAccepted
}

func (vm *VM) LastL1Head() int64 {
	// var f = <-vm.subCh
	return vm.L1Head.Int64()
}

func (vm *VM) IsBootstrapped() bool {
	return vm.bootstrapped.Get()
}

func (vm *VM) State() (*vilmo.Vilmo, error) {
	// TODO: enable state sync
	return vm.stateDB, nil
}

func (vm *VM) Mempool() chain.Mempool {
	return vm.mempool
}

func (vm *VM) IsRepeatTx(ctx context.Context, txs []*chain.Transaction, marker set.Bits, stop bool) set.Bits {
	_, span := vm.tracer.Start(ctx, "VM.IsRepeatTx")
	defer span.End()

	return vm.seenTxs.Contains(txs, marker, stop)
}

func (vm *VM) IsRepeatChunk(ctx context.Context, certs []*chain.ChunkCertificate, marker set.Bits) set.Bits {
	_, span := vm.tracer.Start(ctx, "VM.IsRepeatChunk")
	defer span.End()

	return vm.seenChunks.Contains(certs, marker, false)
}

func (vm *VM) IsSeenChunk(ctx context.Context, chunkID ids.ID) bool {
	return vm.seenChunks.HasID(chunkID)
}

func (vm *VM) Verified(ctx context.Context, b *chain.StatelessBlock) {
	ctx, span := vm.tracer.Start(ctx, "VM.Verified")
	defer span.End()

	vm.verifiedL.Lock()
	vm.verifiedBlocks[b.ID()] = b
	vm.verifiedL.Unlock()
	vm.parsedBlocks.Evict(b.ID())

	// We opt to not remove chunks [b.AvailableChunks] from [cm] here because
	// we may build on a different parent and we want to maximize the probability
	// any cert gets included. If this is not the case, the cert repeat inclusion check
	// is fast.
}

// @todo understand this?
func (vm *VM) processExecutedChunks() {
	// Always close [acceptorDone] or we may block shutdown.
	defer func() {
		close(vm.executorDone)
		vm.snowCtx.Log.Info("executor queue shutdown")
	}()

	// The VM closes [executedQueue] during shutdown. We wait for all enqueued blocks
	// to be processed before returning as a guarantee to listeners (which may
	// persist indexed state) instead of just exiting as soon as `vm.stop` is
	// closed.
	for ew := range vm.executedQueue {
		vm.metrics.executedProcessingBacklog.Dec()
		if ew.Chunk != nil {
			vm.processExecutedChunk(ew.Block, ew.Chunk, ew.Results, ew.InvalidTxs)
			vm.snowCtx.Log.Debug(
				"chunk async executed",
				zap.Uint64("blk", ew.Block.Height),
				zap.Stringer("chunkID", ew.Chunk.Chunk),
			)
			continue
		}
		vm.processExecutedBlock(ew.Block)
		vm.snowCtx.Log.Debug(
			"block async executed",
			zap.Uint64("blk", ew.Block.Height),
		)
	}
}

// @todo fix this?
func (vm *VM) ExecutedChunk(ctx context.Context, blk *chain.StatefulBlock, chunk *chain.FilteredChunk, results []*chain.Result, invalidTxs []ids.ID) {
	ctx, span := vm.tracer.Start(ctx, "VM.ExecutedChunk")
	defer span.End()

	// Mark all txs as seen (prevent replay in subsequent blocks)
	//
	// We do this before Accept to avoid maintaining a set of diffs
	// that we need to check for repeats on top of this.
	vm.seenTxs.Add(chunk.Txs)

	// Add chunk to backlog for async processing
	vm.metrics.executedProcessingBacklog.Inc()
	vm.executedQueue <- &executedWrapper{blk, chunk, results, invalidTxs}

	// Record units processed
	chunkUnits := fees.Dimensions{}
	for _, r := range results {
		nextUnits, err := fees.Add(chunkUnits, r.Units)
		if err != nil {
			vm.Fatal("unable to add executed units", zap.Error(err))
		}
		chunkUnits = nextUnits
	}
	vm.metrics.unitsExecutedBandwidth.Add(float64(chunkUnits[fees.Bandwidth]))
	vm.metrics.unitsExecutedCompute.Add(float64(chunkUnits[fees.Compute]))
	vm.metrics.unitsExecutedRead.Add(float64(chunkUnits[fees.StorageRead]))
	vm.metrics.unitsExecutedAllocate.Add(float64(chunkUnits[fees.StorageAllocate]))
	vm.metrics.unitsExecutedWrite.Add(float64(chunkUnits[fees.StorageWrite]))
}

func (vm *VM) ExecutedBlock(ctx context.Context, blk *chain.StatefulBlock) {
	ctx, span := vm.tracer.Start(ctx, "VM.ExecutedBlock")
	defer span.End()

	// We interleave results with chunks to ensure things are processed in the write order (if processed independently, we might
	// process a block execution before a chunk).
	vm.metrics.executedProcessingBacklog.Inc()
	vm.executedQueue <- &executedWrapper{Block: blk}
}

func (vm *VM) processExecutedBlock(blk *chain.StatefulBlock) {
	start := time.Now()
	defer func() {
		vm.metrics.executedBlockProcess.Observe(float64(time.Since(start)))
	}()

	// Clear authorization results
	vm.metrics.uselessChunkAuth.Add(float64(len(vm.cm.auth.SetMin(blk.Timestamp))))

	// Update timestamp in mempool
	//
	// We wait to update the min until here because we want to allow all execution
	// to complete and remove valid txs first.
	ctx := context.TODO()
	t := blk.Timestamp
	vm.metrics.mempoolExpired.Add(float64(len(vm.mempool.SetMinTimestamp(ctx, t))))
	vm.metrics.mempoolLen.Set(float64(vm.mempool.Len(ctx)))
	vm.metrics.mempoolSize.Set(float64(vm.mempool.Size(ctx)))

	// We need to wait until we may not try to verify the signature of a tx again.
	vm.rpcAuthorizedTxs.SetMin(t)

	// Must clear accepted txs before [SetMinTx] or else we will errnoueously
	// send [ErrExpired] messages.
	if err := vm.webSocketServer.SetMinTx(t); err != nil {
		vm.Fatal("unable to set min tx in websocket server", zap.Error(err))
	}
}

func (vm *VM) processExecutedChunk(
	blk *chain.StatefulBlock,
	chunk *chain.FilteredChunk,
	results []*chain.Result,
	invalidTxs []ids.ID,
) {
	start := time.Now()
	defer func() {
		vm.metrics.executedChunkProcess.Observe(float64(time.Since(start)))
	}()

	// Remove any executed transactions
	ctx := context.TODO()
	vm.mempool.Remove(ctx, chunk.Txs)

	// Send notifications as soon as transactions are executed
	if err := vm.webSocketServer.ExecuteChunk(blk.Height, chunk, results, invalidTxs); err != nil {
		vm.Fatal("unable to execute chunk in websocket server", zap.Error(err))
	}
}

func (vm *VM) Rejected(ctx context.Context, b *chain.StatelessBlock) {
	ctx, span := vm.tracer.Start(ctx, "VM.Rejected")
	defer span.End()

	vm.verifiedL.Lock()
	delete(vm.verifiedBlocks, b.ID())
	vm.verifiedL.Unlock()
	vm.mempool.Add(ctx, b.Txs)

	if err := vm.c.Rejected(ctx, b); err != nil {
		vm.Fatal("rejected processing failed", zap.Error(err))
	}

	// Ensure children of block are cleared, they may never be
	// verified
	vm.snowCtx.Log.Info("rejected block", zap.Stringer("id", b.ID()))
}

func (vm *VM) processAcceptedBlock(b *chain.StatelessBlock) {
	start := time.Now()
	defer func() {
		vm.metrics.blockProcess.Observe(float64(time.Since(start)))
	}()

	// We skip blocks that were not processed because metadata required to
	// process blocks opaquely (like looking at results) is not populated.
	//
	// We don't need to worry about dangling messages in listeners because we
	// don't allow subscription until the node is healthy.
	// if !b.Processed() {
	// 	vm.snowCtx.Log.Info("skipping unprocessed block", zap.Uint64("height", b.Hght))
	// 	return
	// }

	// Update controller
	if err := vm.c.Accepted(context.TODO(), b); err != nil {
		vm.Fatal("accepted processing failed", zap.Error(err))
	}

	// Send notifications as soon as transactions are executed.
	if err := vm.webSocketServer.AcceptBlock(b); err != nil {
		vm.Fatal("unable to accept block in websocket server", zap.Error(err))
	}

}

func (vm *VM) processAcceptedBlocks() {
	// Always close [acceptorDone] or we may block shutdown.
	defer func() {
		close(vm.acceptorDone)
		vm.snowCtx.Log.Info("acceptor queue shutdown")
	}()

	// The VM closes [acceptedQueue] during shutdown. We wait for all enqueued blocks
	// to be processed before returning as a guarantee to listeners (which may
	// persist indexed state) instead of just exiting as soon as `vm.stop` is
	// closed.
	for b := range vm.acceptedQueue {
		vm.processAcceptedBlock(b)
		vm.snowCtx.Log.Info(
			"block processed",
			zap.Stringer("blkID", b.ID()),
			zap.Uint64("height", b.Hght),
		)
	}
}

func (vm *VM) Accepted(ctx context.Context, b *chain.StatelessBlock) {
	ctx, span := vm.tracer.Start(ctx, "VM.Accepted")
	defer span.End()

	vm.metrics.txsAccepted.Add(float64(len(b.Txs)))

	// Update accepted blocks on-disk and caches
	if err := vm.UpdateLastAccepted(b); err != nil {
		vm.Fatal("unable to update last accepted", zap.Error(err))
	}

	// Remove from verified caches
	//
	// We do this after setting [lastAccepted] to avoid
	// a race where the block isn't accessible.
	vm.verifiedL.Lock()
	delete(vm.verifiedBlocks, b.ID())
	vm.verifiedL.Unlock()

	// Update replay protection heap
	//
	// Transactions are added to [seen] with their [expiry], so we don't need to
	// transform [blkTime] when calling [SetMin] here.
	blkTime := b.Tmstmp
	evicted := vm.seen.SetMin(blkTime)
	vm.Logger().Debug("txs evicted from seen", zap.Int("len", len(evicted)))
	vm.seen.Add(b.Txs)

	// Verify if emap is now sufficient (we need a consecutive run of blocks with
	// timestamps of at least [ValidityWindow] for this to occur).
	if !vm.isReady() {
		select {
		case <-vm.seenValidityWindow:
			// We could not be ready but seen a window of transactions if the state
			// to sync is large (takes longer to fetch than [ValidityWindow]).
		default:
			// The value of [vm.startSeenTime] can only be negative if we are
			// performing state sync.
			if vm.startSeenTime < 0 {
				vm.startSeenTime = blkTime
			}
			r := vm.Rules(blkTime)
			if blkTime-vm.startSeenTime > r.GetValidityWindow() {
				vm.seenValidityWindowOnce.Do(func() {
					close(vm.seenValidityWindow)
				})
			}
		}
	}

	// Update timestamp in mempool
	//
	// We rely on the [vm.waiters] map to notify listeners of dropped
	// transactions instead of the mempool because we won't need to iterate
	// through as many transactions.
	removed := vm.mempool.SetMinTimestamp(ctx, blkTime)

	// Enqueue block for processing
	vm.acceptedQueue <- b

	vm.snowCtx.Log.Info(
		"accepted block",
		zap.Stringer("blkID", b.ID()),
		zap.Uint64("height", b.Hght),
		zap.Int("txs", len(b.Txs)),
		zap.Stringer("parent root", b.StateRoot),
		zap.Int("size", len(b.Bytes())),
		zap.Int("dropped mempool txs", len(removed)),
		zap.Bool("state ready", vm.StateReady()),
	)
}

func (vm *VM) CacheValidators(ctx context.Context, height uint64) {
	vm.proposerMonitor.Fetch(ctx, height)
}

func (vm *VM) AddressPartition(ctx context.Context, epoch uint64, height uint64, addr codec.Address, partition uint8) (ids.NodeID, error) {
	return vm.proposerMonitor.AddressPartition(ctx, epoch, height, addr, partition)
}

func (vm *VM) IsValidator(ctx context.Context, height uint64, nid ids.NodeID) (bool, error) {
	ok, _, _, err := vm.proposerMonitor.IsValidator(ctx, height, nid)
	return ok, err
}

func (vm *VM) Proposers(ctx context.Context, diff int, depth int) (set.Set[ids.NodeID], error) {
	return vm.proposerMonitor.Proposers(ctx, diff, depth)
}

func (vm *VM) CurrentValidators(
	ctx context.Context,
) (map[ids.NodeID]*validators.GetValidatorOutput, map[string]struct{}) {
	return vm.proposerMonitor.Validators(ctx)
}

func (vm *VM) NodeID() ids.NodeID {
	return vm.snowCtx.NodeID
}

func (vm *VM) PreferredBlock(ctx context.Context) (*chain.StatelessBlock, error) {
	return vm.GetStatelessBlock(ctx, vm.preferred)
}

func (vm *VM) StopChan() chan struct{} {
	return vm.stop
}

func (vm *VM) EngineChan() chan<- common.Message {
	return vm.toEngine
}

// Used for integration and load testing
// func (vm *VM) Builder() builder.Builder {
// 	return vm.builder
// }

// func (vm *VM) Gossiper() gossiper.Gossiper {
// 	return vm.gossiper
// }

func (vm *VM) StateManager() chain.StateManager {
	return vm.c.StateManager()
}

func (vm *VM) RecordWaitAuth(t time.Duration) {
	vm.metrics.waitAuth.Observe(float64(t))
}

func (vm *VM) RecordWaitExec(c int) {
	vm.metrics.waitExec.Observe(float64(c))
}

func (vm *VM) RecordWaitPrecheck(t time.Duration) {
	vm.metrics.waitPrecheck.Observe(float64(t))
}

func (vm *VM) RecordWaitCommit(t time.Duration) {
	vm.metrics.waitCommit.Observe(float64(t))
}

func (vm *VM) RecordStateChanges(c int) {
	vm.metrics.stateChanges.Observe(float64(c))
}

func (vm *VM) GetVerifyAuth() bool {
	return vm.config.GetVerifyAuth()
}

func (vm *VM) GetAuthExecutionCores() int {
	return vm.config.GetAuthExecutionCores()
}

// This must be non-nil or the VM won't be able to produce chunks
func (vm *VM) Beneficiary() codec.Address {
	return vm.config.GetBeneficiary()
}

func (vm *VM) GetStoreBlockResultsOnDisk() bool {
	return vm.config.GetStoreBlockResultsOnDisk()
}

func (vm *VM) RecordTxsGossiped(c int) {
	vm.metrics.txsGossiped.Add(float64(c))
}

func (vm *VM) RecordTxsReceived(c int) {
	vm.metrics.txsReceived.Add(float64(c))
}

func (vm *VM) GetTargetChunkBuildDuration() time.Duration {
	return vm.config.GetTargetChunkBuildDuration()
}

// func (vm *VM) GetTargetGossipDuration() time.Duration {
// 	return vm.config.GetTargetGossipDuration()
// }

func (vm *VM) cacheAuth(auth chain.Auth) {
	bv, ok := vm.authEngine[auth.GetTypeID()]
	if !ok {
		return
	}
	bv.Cache(auth)
}

func (vm *VM) GetAuthBatchVerifier(authTypeID uint8, cores int, count int) (chain.AuthBatchVerifier, bool) {
	bv, ok := vm.authEngine[authTypeID]
	if !ok {
		return nil, false
	}
	return bv.GetBatchVerifier(cores, count), ok
}

func (vm *VM) RecordBlockVerify(t time.Duration) {
	vm.metrics.blockVerify.Observe(float64(t))
}

func (vm *VM) RecordBlockAccept(t time.Duration) {
	vm.metrics.blockAccept.Observe(float64(t))
}

func (vm *VM) RecordBlockExecute(t time.Duration) {
	vm.metrics.blockExecute.Observe(float64(t))
}

func (vm *VM) RecordRemainingMempool(l int) {
	vm.metrics.remainingMempool.Add(float64(l))
}

func (vm *VM) UnitPrices(context.Context) (fees.Dimensions, error) {
	// v, err := vm.stateDB.Get(chain.FeeKey(vm.StateManager().FeeKey()))
	// if err != nil {
	// 	return fees.Dimensions{}, err
	// }
	// return fees.NewManager(v).UnitPrices(), nil
	return vm.Rules(time.Now().UnixMilli()).GetUnitPrices(), nil
}

func (vm *VM) GetActionExecutionCores() int {
	return vm.config.GetActionExecutionCores()
}

func (vm *VM) GetExecutorRecorder() executor.Metrics {
	return vm.metrics.executorRecorder
}

func (vm *VM) StartCertStream(context.Context) {
	vm.cm.certs.StartStream()
}

func (vm *VM) StreamCert(ctx context.Context) (*chain.ChunkCertificate, bool) {
	return vm.cm.certs.Stream(ctx)
}

func (vm *VM) FinishCertStream(_ context.Context, certs []*chain.ChunkCertificate) {
	vm.cm.certs.FinishStream(certs)
}

func (vm *VM) RestoreChunkCertificates(ctx context.Context, certs []*chain.ChunkCertificate) {
	vm.cm.RestoreChunkCertificates(ctx, certs)
}

func (vm *VM) Engine() *chain.Engine {
	return vm.engine
}

func (vm *VM) IsIssuedTx(_ context.Context, tx *chain.Transaction) bool {
	return vm.issuedTxs.Has(tx)
}

func (vm *VM) IssueTx(_ context.Context, tx *chain.Transaction) {
	vm.issuedTxs.Add([]*chain.Transaction{tx})
}

func (vm *VM) Signer() *bls.PublicKey {
	return vm.snowCtx.PublicKey
}

func (vm *VM) Sign(msg *warp.UnsignedMessage) ([]byte, error) {
	return vm.snowCtx.WarpSigner.Sign(msg)
}

func (vm *VM) RequestChunks(block uint64, certs []*chain.ChunkCertificate, chunks chan *chain.Chunk) {
	vm.cm.RequestChunks(block, certs, chunks)
}

func (vm *VM) RecordEngineBacklog(c int) {
	vm.metrics.engineBacklog.Add(float64(c))
}

func (vm *VM) RecordExecutedChunks(c int) {
	vm.metrics.chunksExecuted.Add(float64(c))
}

func (vm *VM) RecordWaitRepeat(t time.Duration) {
	vm.metrics.waitRepeat.Observe(float64(t))
}

func (vm *VM) GetAuthRPCCores() int {
	return vm.config.GetAuthRPCCores()
}

func (vm *VM) GetAuthRPCBacklog() int {
	return vm.config.GetAuthRPCBacklog()
}

func (vm *VM) RecordRPCTxBacklog(c int64) {
	vm.metrics.rpcTxBacklog.Set(float64(c))
}

func (vm *VM) AddRPCAuthorized(tx *chain.Transaction) {
	vm.rpcAuthorizedTxs.Add([]*chain.Transaction{tx})
}

func (vm *VM) IsRPCAuthorized(txID ids.ID) bool {
	return vm.rpcAuthorizedTxs.HasID(txID)
}

func (vm *VM) RecordRPCAuthorizedTx() {
	vm.metrics.txRPCAuthorized.Inc()
}

func (vm *VM) RecordBlockVerifyFail() {
	vm.metrics.blockVerifyFailed.Inc()
}

func (vm *VM) RecordWebsocketConnection(c int) {
	vm.metrics.websocketConnections.Add(float64(c))
}

func (vm *VM) RecordChunkBuildTxDropped() {
	vm.metrics.chunkBuildTxsDropped.Inc()
}

func (vm *VM) RecordRPCTxInvalid() {
	vm.metrics.rpcTxInvalid.Inc()
}

func (vm *VM) RecordBlockBuildCertDropped() {
	vm.metrics.blockBuildCertsDropped.Inc()
}

func (vm *VM) RecordAcceptedEpoch(e uint64) {
	vm.metrics.lastAcceptedEpoch.Set(float64(e))
}

func (vm *VM) RecordExecutedEpoch(e uint64) {
	vm.metrics.lastExecutedEpoch.Set(float64(e))
}

func (vm *VM) GetAuthResult(chunkID ids.ID) bool {
	// TODO: clean up this invocation
	return vm.cm.auth.Wait(chunkID)
}

func (vm *VM) RecordWaitQueue(t time.Duration) {
	vm.metrics.waitQueue.Observe(float64(t))
}

func (vm *VM) GetPrecheckCores() int {
	return vm.config.GetPrecheckCores()
}

func (vm *VM) RecordVilmoBatchInit(t time.Duration) {
	vm.metrics.appendDBBatchInit.Observe(float64(t))
}

func (vm *VM) RecordVilmoBatchInitBytes(b int64) {
	vm.metrics.appendDBBatchInitBytes.Observe(float64(b))
}

func (vm *VM) RecordVilmoBatchesRewritten() {
	vm.metrics.appendDBBatchesRewritten.Inc()
}

func (vm *VM) RecordVilmoBatchPrepare(t time.Duration) {
	vm.metrics.appendDBBatchPrepare.Observe(float64(t))
}

func (vm *VM) RecordTStateIterate(t time.Duration) {
	vm.metrics.tstateIterate.Observe(float64(t))
}

func (vm *VM) RecordVilmoBatchWrite(t time.Duration) {
	vm.metrics.appendDBBatchWrite.Observe(float64(t))
}
