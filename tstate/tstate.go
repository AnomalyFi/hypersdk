// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import (
	"context"

	"github.com/AnomalyFi/hypersdk/smap"
	"github.com/AnomalyFi/hypersdk/state"

	"github.com/ava-labs/avalanchego/utils/maybe"
)

type change struct {
	chunkIdx int
	txIdx    int
	v        maybe.Maybe[[]byte]
}

// TState defines a struct for storing temporary state.
type TState struct {
	viewKeys    [][][]string
	changedKeys *smap.SMap[*change]
}

// New returns a new instance of TState. Initializes the storage and changedKeys
// maps to have an initial size of [storageSize] and [changedSize] respectively.
func New(changedSize int) *TState {
	return &TState{
		viewKeys:    make([][][]string, 1024), // set to max chunks that could ever be in a single block
		changedKeys: smap.New[*change](changedSize),
	}
}

func (ts *TState) getChangedValue(_ context.Context, key []byte) ([]byte, bool, bool) {
	if v, ok := ts.changedKeys.Get(string(key)); ok {
		if v.v.IsNothing() {
			return nil, true, false
		}
		return v.v.Value(), true, true
	}
	return nil, false, false
}

func (ts *TState) PrepareChunk(idx, size int) {
	ts.viewKeys[idx] = make([][]string, size)
}

// Iterate over changes in deterministic order
//
// Iterate should only be called once tstate is done being modified.
func (ts *TState) Iterate(f func([]byte, maybe.Maybe[[]byte]) error) error {
	// TODO: make naming more generic
	for chunkIdx, txs := range ts.viewKeys {
		if txs == nil {
			// Once we run out of views, exit
			break
		}
		for txIdx, keys := range txs {
			// Skip invalid txs
			if keys == nil {
				continue
			}

			// Ensure we iterate deterministically
			for _, key := range keys {
				v, ok := ts.changedKeys.Get(key)
				if !ok {
					continue
				}
				if v.chunkIdx != chunkIdx || v.txIdx != txIdx {
					// If we weren't the latest modification, skip
					continue
				}
				if err := f([]byte(key), v.v); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (ts *TState) PersistChanges(ctx context.Context, batch state.Mutable) error {
	return ts.Iterate(func(key []byte, value maybe.Maybe[[]byte]) error {
		if value.IsNothing() {

			return batch.Remove(ctx, key)
		} else {

			return batch.Insert(ctx, key, value.Value())
		}
	})
}
