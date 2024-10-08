// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"context"
	"encoding/binary"
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"

	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/consts"
	"github.com/AnomalyFi/hypersdk/emap"
	"github.com/AnomalyFi/hypersdk/pubsub"
)

type WebSocketServer struct {
	logger logging.Logger
	s      *pubsub.Server

	blockListeners *pubsub.Connections

	txL         sync.Mutex
	txListeners map[ids.ID]*pubsub.Connections
	expiringTxs *emap.EMap[*chain.Transaction] // ensures all tx listeners are eventually responded to
}

func NewWebSocketServer(vm VM, maxPendingMessages int) (*WebSocketServer, *pubsub.Server) {
	w := &WebSocketServer{
		logger:         vm.Logger(),
		blockListeners: pubsub.NewConnections(),
		txListeners:    map[ids.ID]*pubsub.Connections{},
		expiringTxs:    emap.NewEMap[*chain.Transaction](),
	}
	cfg := pubsub.NewDefaultServerConfig()
	cfg.MaxPendingMessages = maxPendingMessages
	w.s = pubsub.New(w.logger, cfg, w.MessageCallback(vm))
	return w, w.s
}

// Note: no need to have a tx listener removal, this will happen when all
// submitted transactions are cleared.
func (w *WebSocketServer) AddTxListener(tx *chain.Transaction, c *pubsub.Connection) {
	w.txL.Lock()
	defer w.txL.Unlock()

	// TODO: limit max number of tx listeners a single connection can create
	txID := tx.ID()
	if _, ok := w.txListeners[txID]; !ok {
		w.txListeners[txID] = pubsub.NewConnections()
	}
	w.txListeners[txID].Add(c)
	w.expiringTxs.Add([]*chain.Transaction{tx})
}

// If never possible for a tx to enter mempool, call this
func (w *WebSocketServer) RemoveTx(txID ids.ID, err error) error {
	w.txL.Lock()
	defer w.txL.Unlock()

	return w.removeTx(txID, err)
}

func (w *WebSocketServer) removeTx(txID ids.ID, err error) error {
	listeners, ok := w.txListeners[txID]
	if !ok {
		return nil
	}
	bytes, err := PackRemovedTxMessage(txID, err)
	if err != nil {
		return err
	}
	w.s.Publish(append([]byte{TxMode}, bytes...), listeners)
	delete(w.txListeners, txID)
	// [expiringTxs] will be cleared eventually (does not support removal)
	return nil
}

func (w *WebSocketServer) SetMinTx(t int64) error {
	w.txL.Lock()
	defer w.txL.Unlock()

	expired := w.expiringTxs.SetMin(t)
	for _, id := range expired {
		if err := w.removeTx(id, ErrExpired); err != nil {
			return err
		}
	}
	if exp := len(expired); exp > 0 {
		w.logger.Debug("expired listeners", zap.Int("count", exp))
	}
	return nil
}

func (w *WebSocketServer) AcceptBlock(b *chain.StatelessBlock) error {
	if w.blockListeners.Len() > 0 {
		bytes, err := PackBlockMessage(b)
		if err != nil {
			return err
		}
		inactiveConnection := w.s.Publish(append([]byte{BlockMode}, bytes...), w.blockListeners)
		for _, conn := range inactiveConnection {
			w.blockListeners.Remove(conn)
		}
	}

	w.txL.Lock()
	defer w.txL.Unlock()
	results := b.Results()
	for i, tx := range b.Txs {
		txID := tx.ID()
		listeners, ok := w.txListeners[txID]
		if !ok {
			continue
		}
		// Publish to tx listener
		bytes, err := PackAcceptedTxMessage(txID, results[i])
		if err != nil {
			return err
		}
		w.s.Publish(append([]byte{TxMode}, bytes...), listeners)
		delete(w.txListeners, txID)
		// [expiringTxs] will be cleared eventually (does not support removal)
	}
	return nil
}

func (w *WebSocketServer) MessageCallback(vm VM) pubsub.Callback {
	// Assumes controller is initialized before this is called
	var (
		actionRegistry, authRegistry = vm.Registry()
		tracer                       = vm.Tracer()
		log                          = vm.Logger()
	)

	return func(msgBytes []byte, c *pubsub.Connection) {
		ctx, span := tracer.Start(context.Background(), "WebSocketServer.Callback")
		defer span.End()

		// Check empty messages
		if len(msgBytes) == 0 {
			log.Error("failed to unmarshal msg",
				zap.Int("len", len(msgBytes)),
			)
			return
		}

		// TODO: convert into a router that can be re-used in custom WS
		// implementations
		switch msgBytes[0] {
		case BlockMode:
			w.blockListeners.Add(c)
			log.Debug("added block listener")
			if len(msgBytes) > 1 {
				currentBlockHeight := vm.LastAcceptedBlock().Height()
				blockNumber := binary.BigEndian.Uint64(msgBytes[1:])
				if currentBlockHeight <= blockNumber {
					w.logger.Error("Invalid block height", zap.Uint64("current block height", currentBlockHeight), zap.Uint64("given block height", blockNumber))
					return
				}
				has, err := vm.HasDiskBlock(blockNumber)
				if err != nil || !has {
					w.logger.Error("Could not find block on disk", zap.Uint64("height", blockNumber))
					return
				} else {
					for i := blockNumber; i <= currentBlockHeight; i++ {
						blk, err := vm.GetDiskBlock(ctx, i)
						if err != nil {
							w.logger.Error("Couldnt find block on disk", zap.Uint64("height", i))
							return
						}
						blkResults, err := vm.GetDiskBlockResults(ctx, i)
						if err != nil {
							w.logger.Error("Couldnt find block results on disk", zap.Uint64("height", i))
							return
						}
						feeBytes, err := vm.GetDiskFeeManager(ctx, i)
						if err != nil {
							w.logger.Error("Something went wrong, Couldnt find block results on disk")
							w.logger.Error("Couldnt get feeBytes on disk", zap.Uint64("height", i))
							return
						}
						bytes, err := PackBlockMessageLegacy(blk.StatefulBlock, blkResults, feeBytes)
						if err != nil {
							return
						}
						if !c.Send(append([]byte{BlockMode}, bytes...)) {
							w.logger.Error("dropping message to subscribed connection due to too many pending messages")
						}
					}
				}
			}
		case TxMode:
			msgBytes = msgBytes[1:]
			// Unmarshal TX
			p := codec.NewReader(msgBytes, consts.NetworkSizeLimit) // will likely be much smaller
			tx, err := chain.UnmarshalTx(p, actionRegistry, authRegistry)
			if err != nil {
				log.Error("failed to unmarshal tx",
					zap.Int("len", len(msgBytes)),
					zap.Error(err),
				)
				return
			}

			// Verify tx
			if vm.GetVerifyAuth() {
				msg, err := tx.Digest()
				if err != nil {
					// Should never occur because populated during unmarshal
					return
				}
				if err := tx.Auth.Verify(ctx, msg); err != nil {
					log.Error("failed to verify sig",
						zap.Error(err),
					)
					return
				}
			}
			w.AddTxListener(tx, c)

			// Submit will remove from [txWaiters] if it is not added
			txID := tx.ID()
			if err := vm.Submit(ctx, false, []*chain.Transaction{tx})[0]; err != nil {
				log.Error("failed to submit tx",
					zap.Stringer("txID", txID),
					zap.Error(err),
				)
				return
			}
			log.Debug("submitted tx", zap.Stringer("id", txID))
		default:
			log.Error("unexpected message type",
				zap.Int("len", len(msgBytes)),
				zap.Uint8("mode", msgBytes[0]),
			)
		}
	}
}
