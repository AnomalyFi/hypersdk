// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/consts"
	"github.com/AnomalyFi/hypersdk/crypto/bls"
	"github.com/AnomalyFi/hypersdk/utils"
	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"go.uber.org/zap"
)

const (
	refreshTime            = 30 * time.Second
	proposerMonitorLRUSize = 60
)

type proposerInfo struct {
	validators   map[ids.NodeID]*validators.GetValidatorOutput
	partitionSet []ids.NodeID
	warpSet      []*warp.Validator
	totalWeight  uint64
}

type ProposerMonitor struct {
	vm *VM

	fetchLock sync.Mutex
	proposers *cache.LRU[uint64, *proposerInfo] // safe for concurrent use.

	currentLock        sync.Mutex
	currentPHeight     uint64
	lastFetchedPHeight time.Time
	currentValidators  map[ids.NodeID]*validators.GetValidatorOutput
}

func NewProposerMonitor(vm *VM) *ProposerMonitor {
	return &ProposerMonitor{
		vm:        vm,
		proposers: &cache.LRU[uint64, *proposerInfo]{Size: proposerMonitorLRUSize},
	}
}

// TODO: don't add validators that won't be validators for the entire epoch
func (p *ProposerMonitor) fetch(ctx context.Context, height uint64) *proposerInfo {
	validators, err := p.vm.snowCtx.ValidatorState.GetValidatorSet(
		ctx,
		height,
		p.vm.snowCtx.SubnetID,
	)
	if err != nil {
		p.vm.snowCtx.Log.Error("failed to fetch proposer set", zap.Uint64("height", height), zap.Error(err))
		return nil
	}
	partitionSet, warpSet, totalWeight, err := utils.ConstructCanonicalValidatorSet(validators)
	if err != nil {
		p.vm.snowCtx.Log.Error("failed to construct canonical validator set", zap.Uint64("height", height), zap.Error(err))
		return nil
	}
	info := &proposerInfo{
		validators:   validators,
		partitionSet: partitionSet,
		warpSet:      warpSet,
		totalWeight:  totalWeight,
	}
	p.proposers.Put(height, info)
	return info
}

// Fetch is used to pre-cache sets that will be used later
//
// TODO: remove lock? replace with lockmap?
func (p *ProposerMonitor) Fetch(ctx context.Context, height uint64) *proposerInfo {
	p.fetchLock.Lock()
	defer p.fetchLock.Unlock()

	return p.fetch(ctx, height)
}

func (p *ProposerMonitor) IsValidator(ctx context.Context, height uint64, nodeID ids.NodeID) (bool, *bls.PublicKey, uint64, error) {
	info, ok := p.proposers.Get(height)
	if !ok {
		info = p.Fetch(ctx, height)
	}
	if info == nil {
		return false, nil, 0, errors.New("could not get validator set for height")
	}
	output, exists := info.validators[nodeID]
	if exists {
		return true, output.PublicKey, output.Weight, nil
	}
	return false, nil, 0, nil
}

// GetWarpValidatorSet returns the validator set of [subnetID] in a canonical ordering.
// Also returns the total weight on [subnetID].
func (p *ProposerMonitor) GetWarpValidatorSet(ctx context.Context, height uint64) ([]*warp.Validator, uint64, error) {
	info, ok := p.proposers.Get(height)
	if !ok {
		info = p.Fetch(ctx, height)
	}
	if info == nil {
		return nil, 0, errors.New("could not get validator set for height")
	}
	return info.warpSet, info.totalWeight, nil
}

func (p *ProposerMonitor) GetValidatorSet(ctx context.Context, height uint64, includeMe bool) (set.Set[ids.NodeID], error) {
	info, ok := p.proposers.Get(height)
	if !ok {
		info = p.Fetch(ctx, height)
	}
	if info == nil {
		return nil, errors.New("could not get validator set for height")
	}
	vdrSet := set.NewSet[ids.NodeID](len(info.validators))
	for v := range info.validators {
		if v == p.vm.snowCtx.NodeID && !includeMe {
			continue
		}
		vdrSet.Add(v)
	}
	return vdrSet, nil
}

func (p *ProposerMonitor) refreshCurrent(ctx context.Context) error {
	pHeight, err := p.vm.snowCtx.ValidatorState.GetCurrentHeight(ctx)
	if err != nil {
		p.currentLock.Unlock()
		return err
	}
	p.currentPHeight = pHeight
	validators, err := p.vm.snowCtx.ValidatorState.GetValidatorSet(
		ctx,
		pHeight,
		p.vm.snowCtx.SubnetID,
	)
	if err != nil {
		return err
	}
	p.lastFetchedPHeight = time.Now()
	p.currentValidators = validators
	return nil
}

// Prevent unnecessary map copies
func (p *ProposerMonitor) IterateCurrentValidators(
	ctx context.Context,
	f func(ids.NodeID, *validators.GetValidatorOutput),
) error {
	// Refresh P-Chain height if [refreshTime] has elapsed
	p.currentLock.Lock()
	if time.Since(p.lastFetchedPHeight) > refreshTime {
		if err := p.refreshCurrent(ctx); err != nil {
			p.currentLock.Unlock()
			return err
		}
	}
	validators := p.currentValidators
	p.currentLock.Unlock()

	// Iterate over the validators
	for k, v := range validators {
		f(k, v)
	}
	return nil
}

func (p *ProposerMonitor) IsValidHeight(ctx context.Context, height uint64) (bool, error) {
	p.currentLock.Lock()
	defer p.currentLock.Unlock()

	if height <= p.currentPHeight {
		return true, nil
	}
	if err := p.refreshCurrent(ctx); err != nil {
		return false, err
	}
	return height <= p.currentPHeight, nil
}

// Prevent unnecessary map copies
func (p *ProposerMonitor) IterateValidators(
	ctx context.Context,
	height uint64,
	f func(ids.NodeID, *validators.GetValidatorOutput),
) error {
	info, ok := p.proposers.Get(height)
	if !ok {
		info = p.Fetch(ctx, height)
	}
	if info == nil {
		return errors.New("could not get validator set for height")
	}
	for k, v := range info.validators {
		f(k, v)
	}
	return nil
}

func (p *ProposerMonitor) RandomValidator(ctx context.Context, height uint64) (ids.NodeID, error) {
	info, ok := p.proposers.Get(height)
	if !ok {
		info = p.Fetch(ctx, height)
	}
	if info == nil {
		return ids.NodeID{}, errors.New("could not get validator set for height")
	}
	for k := range info.validators { // Golang map iteration order is random
		return k, nil
	}
	return ids.NodeID{}, fmt.Errorf("no validators")
}

// TODO: Generate a Deterministic PRNG for assigning namespace to validator.
func (p *ProposerMonitor) AddressPartition(ctx context.Context, epoch uint64, height uint64, addr codec.Address, partition uint8) (ids.NodeID, error) {
	// Get determinisitc ordering of validators
	info, ok := p.proposers.Get(height)
	if !ok {
		info = p.Fetch(ctx, height)
	}
	if info == nil {
		return ids.NodeID{}, errors.New("could not get validator set for height")
	}
	if len(info.partitionSet) == 0 {
		return ids.NodeID{}, errors.New("no validators")
	}

	// Compute seed
	seedBytes := make([]byte, consts.Uint64Len*2+codec.AddressLen)
	binary.BigEndian.PutUint64(seedBytes, epoch) // ensures partitions rotate even if P-Chain height is static
	binary.BigEndian.PutUint64(seedBytes[consts.Uint64Len:], height)
	copy(seedBytes[consts.Uint64Len*2:], addr[:])
	seed := utils.ToID(seedBytes)

	// Select validator
	//
	// It is important to ensure each partition is actually a unique validator, otherwise
	// the censorship resistance that partitions are supposed to provide is lost (all partitions
	// could be allocated to a single validator if we aren't careful).
	seedInt := new(big.Int).SetBytes(seed[:])
	partitionInt := new(big.Int).Add(seedInt, big.NewInt(int64(partition)))
	partitionIdx := new(big.Int).Mod(partitionInt, big.NewInt(int64(len(info.partitionSet)))).Int64()
	return info.partitionSet[int(partitionIdx)], nil
}
