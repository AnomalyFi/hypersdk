package chain

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/hypersdk/executor"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"
)

const numTxs = 50000 // TODO: somehow estimate this (needed to ensure no backlog)

var ErrNotReady = errors.New("not ready")

type Processor struct {
	vm VM

	l        sync.Mutex
	complete bool
	err      error

	im        state.Immutable
	sm        StateManager
	cacheLock sync.RWMutex
	cache     map[string]*fetchData
	exectutor *executor.Executor
	ts        *tstate.TState
	results   []*Result

	input  chan *Chunk
	output []*Chunk
}

func NewProcessor(
	vm VM,
	chunks int,
) *Processor {
	return &Processor{
		vm: vm,

		input:  make(chan *Chunk, chunks),
		output: make([]*Chunk, 0, chunks),
	}
}

// TODO: handle mapping chunk to new chunk
// TODO: new chunk could have warp results + results?
// TODO: kickoff signature verification before begin execution
func (p *Processor) process(ctx context.Context, c *Chunk) (*Chunk, error) {
	for _, tx := range c.Txs {
		stateKeys, err := tx.StateKeys(p.sm)
		if err != nil {
			// TODO: don't stop, just skip
			e.Stop()
			return nil, nil, err
		}
		e.Run(stateKeys, func() error {
			// Fetch keys from cache
			var (
				reads    = make(map[string]uint16, len(stateKeys))
				storage  = make(map[string][]byte, len(stateKeys))
				toLookup = make([]string, 0, len(stateKeys))
			)
			cacheLock.RLock()
			for k := range stateKeys {
				if v, ok := cache[k]; ok {
					reads[k] = v.chunks
					if v.exists {
						storage[k] = v.v
					}
					continue
				}
				toLookup = append(toLookup, k)
			}
			cacheLock.RUnlock()

			// Fetch keys from disk
			var toCache map[string]*fetchData
			if len(toLookup) > 0 {
				toCache = make(map[string]*fetchData, len(toLookup))
				for _, k := range toLookup {
					v, err := im.GetValue(ctx, []byte(k))
					if errors.Is(err, database.ErrNotFound) {
						reads[k] = 0
						toCache[k] = &fetchData{nil, false, 0}
						continue
					} else if err != nil {
						return err
					}
					// We verify that the [NumChunks] is already less than the number
					// added on the write path, so we don't need to do so again here.
					numChunks, ok := keys.NumChunks(v)
					if !ok {
						return ErrInvalidKeyValue
					}
					reads[k] = numChunks
					toCache[k] = &fetchData{v, true, numChunks}
					storage[k] = v
				}
			}

			// Execute transaction
			//
			// It is critical we explicitly set the scope before each transaction is
			// processed
			tsv := ts.NewView(stateKeys, storage)

			// Ensure we have enough funds to pay fees
			if err := tx.PreExecute(ctx, feeManager, sm, r, tsv, t); err != nil {
				return err
			}

			// Wait to execute transaction until we have the warp result processed.
			var warpVerified bool
			warpMsg, ok := b.warpMessages[tx.ID()]
			if ok {
				select {
				case warpVerified = <-warpMsg.verifiedChan:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			result, err := tx.Execute(ctx, feeManager, reads, sm, r, tsv, t, ok && warpVerified)
			if err != nil {
				return err
			}
			results[i] = result

			// Update block metadata with units actually consumed (if more is consumed than block allows, we will non-deterministically
			// exit with an error based on which tx over the limit is processed first)
			if ok, d := feeManager.Consume(result.Consumed, r.GetMaxBlockUnits()); !ok {
				return fmt.Errorf("%w: %d too large", ErrInvalidUnitsConsumed, d)
			}

			// Commit results to parent [TState]
			tsv.Commit()

			// Update key cache
			if len(toCache) > 0 {
				cacheLock.Lock()
				for k := range toCache {
					cache[k] = toCache[k]
				}
				cacheLock.Unlock()
			}
			return nil
		})
	}
	if err := e.Wait(); err != nil {
		return nil, nil, err
	}

	// Return tstate that can be used to add block-level keys to state
	return results, ts, nil
}

func (p *Processor) Run(ctx context.Context, im state.Immutable) {
	ctx, span := p.vm.Tracer().Start(ctx, "Processor.Run")
	defer span.End()

	// Setup the processor
	p.im = im
	p.sm = p.vm.StateManager()
	p.cache = make(map[string]*fetchData, numTxs)
	p.exectutor = executor.New(numTxs, p.vm.GetTransactionExecutionCores(), p.vm.GetExecutorVerifyRecorder())
	p.ts = tstate.New(numTxs * 2)
	p.results = make([]*Result, numTxs)

	// Handle chunks
	for {
		select {
		case c, ok := <-p.input:
			if !ok {
				p.l.Lock()
				p.complete = true
				p.l.Unlock()
				return
			}

			p.l.Lock()
			if p.err != nil {
				p.l.Unlock()
				continue
			}

			filtered, err := p.process(ctx, c)
			p.l.Lock()
			if err != nil && p.err == nil {
				p.err = ctx.Err()
				p.l.Unlock()
				continue
			}
			p.output = append(p.output, filtered)
			p.l.Unlock()

		case <-ctx.Done():
			p.l.Lock()
			if p.err != nil {
				p.err = ctx.Err()
			}
			p.l.Unlock()
			return
		}
	}
}

// Allows processing to start before all chunks are acquired.
func (p *Processor) Add(chunk *Chunk) {
	p.input <- chunk
}

func (p *Processor) Done() {
	close(p.input)
}

// TODO: figure out how to return warp?
func (p *Processor) Results() ([]*Chunk, error) {
	p.l.Lock()
	defer p.l.Unlock()

	if !p.complete {
		return nil, ErrNotReady
	}
	if p.err != nil {
		return nil, p.err
	}
	return p.output, p.err
}
