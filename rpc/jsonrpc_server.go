// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/consts"
	"github.com/AnomalyFi/hypersdk/crypto/bls"
	feemarket "github.com/AnomalyFi/hypersdk/fee_market"
	"github.com/AnomalyFi/hypersdk/fees"
)

type JSONRPCServer struct {
	vm VM
}

func NewJSONRPCServer(vm VM) *JSONRPCServer {
	return &JSONRPCServer{vm}
}

type PingReply struct {
	Success bool `json:"success"`
}

func (j *JSONRPCServer) Ping(_ *http.Request, _ *struct{}, reply *PingReply) (err error) {
	j.vm.Logger().Info("ping")
	reply.Success = true
	return nil
}

type NetworkReply struct {
	NetworkID uint32 `json:"networkId"`
	SubnetID  ids.ID `json:"subnetId"`
	ChainID   ids.ID `json:"chainId"`
}

func (j *JSONRPCServer) Network(_ *http.Request, _ *struct{}, reply *NetworkReply) (err error) {
	reply.NetworkID = j.vm.NetworkID()
	reply.SubnetID = j.vm.SubnetID()
	reply.ChainID = j.vm.ChainID()
	return nil
}

type SubmitTxArgs struct {
	Tx []byte `json:"tx"`
}

type SubmitTxReply struct {
	TxID ids.ID `json:"txId"`
}

func (j *JSONRPCServer) SubmitTx(
	req *http.Request,
	args *SubmitTxArgs,
	reply *SubmitTxReply,
) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "JSONRPCServer.SubmitTx")
	defer span.End()

	actionRegistry, authRegistry := j.vm.Registry()
	rtx := codec.NewReader(args.Tx, consts.NetworkSizeLimit) // will likely be much smaller than this
	tx, err := chain.UnmarshalTx(rtx, actionRegistry, authRegistry)
	if err != nil {
		return fmt.Errorf("%w: unable to unmarshal on public service", err)
	}
	if !rtx.Empty() {
		return errors.New("tx has extra bytes")
	}
	msg, err := tx.Digest()
	if err != nil {
		// Should never occur because populated during unmarshal
		return err
	}
	if err := tx.Auth.Verify(ctx, msg); err != nil {
		return err
	}
	txID := tx.ID()
	reply.TxID = txID
	return j.vm.Submit(ctx, false, []*chain.Transaction{tx})[0]
}

type LastAcceptedReply struct {
	Height    uint64 `json:"height"`
	BlockID   ids.ID `json:"blockId"`
	Timestamp int64  `json:"timestamp"`
}

func (j *JSONRPCServer) LastAccepted(_ *http.Request, _ *struct{}, reply *LastAcceptedReply) error {
	blk := j.vm.LastAcceptedBlock()
	reply.Height = blk.Hght
	reply.BlockID = blk.ID()
	reply.Timestamp = blk.Tmstmp
	return nil
}

type UnitPricesReply struct {
	UnitPrices fees.Dimensions `json:"unitPrices"`
}

func (j *JSONRPCServer) UnitPrices(
	req *http.Request,
	_ *struct{},
	reply *UnitPricesReply,
) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "JSONRPCServer.UnitPrices")
	defer span.End()

	unitPrices, err := j.vm.UnitPrices(ctx)
	if err != nil {
		return err
	}
	reply.UnitPrices = unitPrices
	return nil
}

type NameSpacePriceArgs struct {
	NameSpace string `json:"namespace"`
}

type NameSpacePriceReply struct {
	Price uint64 `json:"price"`
}

func (j *JSONRPCServer) NameSpacePrice(
	req *http.Request,
	args *NameSpacePriceArgs,
	reply *NameSpacePriceReply,
) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "JSONRPCServer.NameSpacePrice")
	defer span.End()

	price, err := j.vm.NameSpacePrice(ctx, args.NameSpace)
	reply.Price = price
	if err != nil && err != feemarket.ErrNamespaceNotFound {
		return err
	}
	return nil
}

type GetProposerArgs struct {
	PBlockHeight uint64 `json:"pBlockHeight"`
	BlockHeight  uint64 `json:"blockHeight"`
	MaxWindows   int    `json:"maxWindows"`
}
type GetProposerReply struct {
	Proposers *[]ids.NodeID `json:"proposers"`
}

func (j *JSONRPCServer) GetProposer(
	req *http.Request,
	args *GetProposerArgs,
	reply *GetProposerReply,
) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "JSONRPCServer.GetProposer")
	defer span.End()
	proposers, err := j.vm.GetProposer(ctx, args.BlockHeight, args.PBlockHeight, args.MaxWindows)
	if err != nil {
		return err
	}
	reply.Proposers = &proposers
	return nil
}

type GetValidatorsReply struct {
	Validators map[ids.NodeID][]byte `json:"validators"`
}

func (j *JSONRPCServer) GetValidators(
	req *http.Request,
	_ *struct{},
	rep *GetValidatorsReply,
) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "JSONRPCServer.GetValidators")
	defer span.End()
	val, _ := j.vm.CurrentValidators(ctx)
	newVal := make(map[ids.NodeID][]byte)
	for _, v := range val {
		newVal[v.NodeID] = bls.PublicKeyToBytes(v.PublicKey)
	}
	rep.Validators = newVal
	return nil
}
