// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/keys"
)

// compactionOffset is used to randomize the height that we compact
// deleted blocks. This prevents all nodes on the network from deleting
// data from disk at the same time.
var compactionOffset int = -1

func init() {
	rand.Seed(time.Now().UnixNano())
}

const (
	blockPrefix         = 0x0 // TODO: move to flat files (https://github.com/ava-labs/hypersdk/issues/553)
	blockIDHeightPrefix = 0x1 // ID -> Height
	blockHeightIDPrefix = 0x2 // Height -> ID (don't always need full block from disk)
	warpSignaturePrefix = 0x3
	warpFetchPrefix     = 0x4
	chunkPrefix         = 0x5 // pruneable chunks (sort by slot)
	filteredChunkPrefix = 0x6 // long-term persistence chunks (TODO: move to flat files or external storage)
)

var (
	isSyncing    = []byte("is_syncing")
	lastAccepted = []byte("last_accepted")

	signatureLRU = &cache.LRU[string, *chain.WarpSignature]{Size: 1024}
)

func PrefixBlockKey(height uint64) []byte {
	k := make([]byte, 1+consts.Uint64Len)
	k[0] = blockPrefix
	binary.BigEndian.PutUint64(k[1:], height)
	return k
}

func PrefixBlockIDHeightKey(id ids.ID) []byte {
	k := make([]byte, 1+consts.IDLen)
	k[0] = blockIDHeightPrefix
	copy(k[1:], id[:])
	return k
}

func PrefixBlockHeightIDKey(height uint64) []byte {
	k := make([]byte, 1+consts.Uint64Len)
	k[0] = blockHeightIDPrefix
	binary.BigEndian.PutUint64(k[1:], height)
	return k
}

func (vm *VM) HasGenesis() (bool, error) {
	return vm.HasDiskBlock(0)
}

func (vm *VM) GetGenesis(ctx context.Context) (*chain.StatelessBlock, error) {
	return vm.GetDiskBlock(ctx, 0)
}

func (vm *VM) SetLastAcceptedHeight(height uint64) error {
	return vm.vmDB.Put(lastAccepted, binary.BigEndian.AppendUint64(nil, height))
}

func (vm *VM) HasLastAccepted() (bool, error) {
	return vm.vmDB.Has(lastAccepted)
}

func (vm *VM) GetLastAcceptedHeight() (uint64, error) {
	b, err := vm.vmDB.Get(lastAccepted)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(b), nil
}

func (vm *VM) shouldComapct(expiryHeight uint64) bool {
	if compactionOffset == -1 {
		compactionOffset = rand.Intn(vm.config.GetBlockCompactionFrequency()) //nolint:gosec
		vm.Logger().Info("setting compaction offset", zap.Int("n", compactionOffset))
	}
	return expiryHeight%uint64(vm.config.GetBlockCompactionFrequency()) == uint64(compactionOffset)
}

// UpdateLastAccepted updates the [lastAccepted] index, stores [blk] on-disk,
// adds [blk] to the [acceptedCache], and deletes any expired blocks from
// disk.
//
// Blocks written to disk are only used when restarting the node. During normal
// operation, we only fetch blocks from memory.
//
// We store blocks by height because it doesn't cause nearly as much
// compaction as storing blocks randomly on-disk (when using [block.ID]).
func (vm *VM) UpdateLastAccepted(blk *chain.StatelessBlock) error {
	batch := vm.vmDB.NewBatch()
	bigEndianHeight := binary.BigEndian.AppendUint64(nil, blk.Height())
	if err := batch.Put(lastAccepted, bigEndianHeight); err != nil {
		return err
	}
	if err := batch.Put(PrefixBlockKey(blk.Height()), blk.Bytes()); err != nil {
		return err
	}
	if err := batch.Put(PrefixBlockIDHeightKey(blk.ID()), bigEndianHeight); err != nil {
		return err
	}
	blkID := blk.ID()
	if err := batch.Put(PrefixBlockHeightIDKey(blk.Height()), blkID[:]); err != nil {
		return err
	}
	expiryHeight := blk.Height() - uint64(vm.config.GetAcceptedBlockWindow())
	var expired bool
	if expiryHeight > 0 && expiryHeight < blk.Height() { // ensure we don't free genesis
		if err := batch.Delete(PrefixBlockKey(expiryHeight)); err != nil {
			return err
		}
		blkID, err := vm.vmDB.Get(PrefixBlockHeightIDKey(expiryHeight))
		if err == nil {
			if err := batch.Delete(PrefixBlockIDHeightKey(ids.ID(blkID))); err != nil {
				return err
			}
		} else {
			vm.Logger().Warn("unable to delete blkID", zap.Uint64("height", expiryHeight), zap.Error(err))
		}
		if err := batch.Delete(PrefixBlockHeightIDKey(expiryHeight)); err != nil {
			return err
		}
		expired = true
		vm.metrics.deletedBlocks.Inc()
		vm.Logger().Info("deleted block", zap.Uint64("height", expiryHeight))
	}
	if err := batch.Write(); err != nil {
		return fmt.Errorf("%w: unable to update last accepted", err)
	}
	vm.lastAccepted = blk
	vm.acceptedBlocksByID.Put(blk.ID(), blk)
	vm.acceptedBlocksByHeight.Put(blk.Height(), blk.ID())
	if expired && vm.shouldComapct(expiryHeight) {
		go func() {
			start := time.Now()
			if err := vm.CompactDiskBlocks(expiryHeight); err != nil {
				vm.Logger().Error("unable to compact blocks", zap.Error(err))
				return
			}
			vm.Logger().Info("compacted disk blocks", zap.Uint64("end", expiryHeight), zap.Duration("t", time.Since(start)))
		}()
	}
	// TODO: clean up expired chunks
	return nil
}

func (vm *VM) GetDiskBlock(ctx context.Context, height uint64) (*chain.StatelessBlock, error) {
	b, err := vm.vmDB.Get(PrefixBlockKey(height))
	if err != nil {
		return nil, err
	}
	return chain.ParseBlock(ctx, b, choices.Accepted, vm)
}

func (vm *VM) HasDiskBlock(height uint64) (bool, error) {
	return vm.vmDB.Has(PrefixBlockKey(height))
}

func (vm *VM) GetBlockHeightID(height uint64) (ids.ID, error) {
	b, err := vm.vmDB.Get(PrefixBlockHeightIDKey(height))
	if err != nil {
		return ids.Empty, err
	}
	return ids.ID(b), nil
}

func (vm *VM) GetBlockIDHeight(blkID ids.ID) (uint64, error) {
	b, err := vm.vmDB.Get(PrefixBlockIDHeightKey(blkID))
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(b), nil
}

// CompactDiskBlocks forces compaction on the entire range of blocks up to [lastExpired].
//
// This can be used to ensure we clean up all large tombstoned keys on a regular basis instead
// of waiting for the database to run a compaction (and potentially delete GBs of data at once).
//
// TODO: change this to be WS driven as well
func (vm *VM) CompactDiskBlocks(lastExpired uint64) error {
	return vm.vmDB.Compact([]byte{blockPrefix}, PrefixBlockKey(lastExpired))
}

func (vm *VM) GetDiskIsSyncing() (bool, error) {
	v, err := vm.vmDB.Get(isSyncing)
	if errors.Is(err, database.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return v[0] == 0x1, nil
}

func (vm *VM) PutDiskIsSyncing(v bool) error {
	if v {
		return vm.vmDB.Put(isSyncing, []byte{0x1})
	}
	return vm.vmDB.Put(isSyncing, []byte{0x0})
}

func (vm *VM) GetOutgoingWarpMessage(txID ids.ID) (*warp.UnsignedMessage, error) {
	p := vm.c.StateManager().OutgoingWarpKeyPrefix(txID)
	k := keys.EncodeChunks(p, chain.MaxOutgoingWarpChunks)
	vs, errs := vm.ReadState(context.TODO(), [][]byte{k})
	v, err := vs[0], errs[0]
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return warp.ParseUnsignedMessage(v)
}

func PrefixWarpSignatureKey(txID ids.ID, signer *bls.PublicKey) []byte {
	k := make([]byte, 1+consts.IDLen+bls.PublicKeyLen)
	k[0] = warpSignaturePrefix
	copy(k[1:], txID[:])
	copy(k[1+consts.IDLen:], bls.PublicKeyToCompressedBytes(signer))
	return k
}

func (vm *VM) StoreWarpSignature(txID ids.ID, signer *bls.PublicKey, signature []byte) error {
	k := PrefixWarpSignatureKey(txID, signer)
	// Cache any signature we produce for later queries from peers
	if bytes.Equal(vm.pkBytes, bls.PublicKeyToCompressedBytes(signer)) {
		signatureLRU.Put(string(k), chain.NewWarpSignature(vm.pkBytes, signature))
	}
	return vm.vmDB.Put(k, signature)
}

func (vm *VM) GetWarpSignature(txID ids.ID, signer *bls.PublicKey) (*chain.WarpSignature, error) {
	k := PrefixWarpSignatureKey(txID, signer)
	if ws, ok := signatureLRU.Get(string(k)); ok {
		return ws, nil
	}
	v, err := vm.vmDB.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ws := &chain.WarpSignature{
		PublicKey: bls.PublicKeyToCompressedBytes(signer),
		Signature: v,
	}
	return ws, nil
}

func (vm *VM) GetWarpSignatures(txID ids.ID) ([]*chain.WarpSignature, error) {
	prefix := make([]byte, 1+consts.IDLen)
	prefix[0] = warpSignaturePrefix
	copy(prefix[1:], txID[:])
	iter := vm.vmDB.NewIteratorWithPrefix(prefix)
	defer iter.Release()

	// Collect all signatures we have for a txID
	signatures := []*chain.WarpSignature{}
	for iter.Next() {
		k := iter.Key()
		signatures = append(signatures, &chain.WarpSignature{
			PublicKey: k[len(k)-bls.PublicKeyLen:],
			Signature: iter.Value(),
		})
	}
	return signatures, iter.Error()
}

func PrefixWarpFetchKey(txID ids.ID) []byte {
	k := make([]byte, 1+consts.IDLen)
	k[0] = warpFetchPrefix
	copy(k[1:], txID[:])
	return k
}

func (vm *VM) StoreWarpFetch(txID ids.ID) error {
	k := PrefixWarpFetchKey(txID)
	return vm.vmDB.Put(k, binary.BigEndian.AppendUint64(nil, uint64(time.Now().UnixMilli())))
}

func (vm *VM) GetWarpFetch(txID ids.ID) (int64, error) {
	k := PrefixWarpFetchKey(txID)
	v, err := vm.vmDB.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return -1, nil
	}
	if err != nil {
		return -1, err
	}
	return int64(binary.BigEndian.Uint64(v)), nil
}

func PrefixChunkKey(slot int64, chunk ids.ID) []byte {
	k := make([]byte, 1+consts.Int64Len+consts.IDLen)
	k[0] = chunkPrefix
	binary.BigEndian.PutUint64(k[1:], uint64(slot))
	copy(k[1+consts.Int64Len:], chunk[:])
	return k
}

func (vm *VM) StoreChunk(chunk *chain.Chunk) error {
	cid, err := chunk.ID()
	if err != nil {
		return err
	}
	k := PrefixChunkKey(chunk.Slot, cid)
	b, err := chunk.Marshal()
	if err != nil {
		return err
	}
	return vm.vmDB.Put(k, b)
}

func (vm *VM) GetChunk(slot int64, chunk ids.ID) (*chain.Chunk, error) {
	k := PrefixChunkKey(slot, chunk)
	b, err := vm.vmDB.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return chain.UnmarshalChunk(b, vm)
}

// TODO: add function to clear chunks

func PrefixFilteredChunkKey(chunk ids.ID) []byte {
	k := make([]byte, 1+consts.IDLen)
	k[0] = filteredChunkPrefix
	copy(k[1:], chunk[:])
	return k
}

func (vm *VM) StoreFilteredChunk(chunk *chain.FilteredChunk) error {
	cid, err := chunk.ID()
	if err != nil {
		return err
	}
	k := PrefixFilteredChunkKey(cid)
	b, err := chunk.Marshal()
	if err != nil {
		return err
	}
	return vm.vmDB.Put(k, b)
}

func (vm *VM) GetFilteredChunk(chunk ids.ID) (*chain.FilteredChunk, error) {
	k := PrefixFilteredChunkKey(chunk)
	b, err := vm.vmDB.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return chain.UnmarshalFilteredChunk(b, vm)
}
