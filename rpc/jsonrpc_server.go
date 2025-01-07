// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpc

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"go.uber.org/zap"

	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/consts"
	"github.com/AnomalyFi/hypersdk/fees"

	feemarket "github.com/AnomalyFi/hypersdk/fee_market"
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

type NameSpacesPriceArgs struct {
	NameSpaces []string `json:"namespaces"`
}

type NameSpacesPriceReply struct {
	Price []uint64 `json:"price"`
}

func (j *JSONRPCServer) NameSpacesPrice(
	req *http.Request,
	args *NameSpacesPriceArgs,
	reply *NameSpacesPriceReply,
) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "JSONRPCServer.NameSpacesPrice")
	defer span.End()
	price, err := j.vm.NameSpacesPrice(ctx, args.NameSpaces)
	reply.Price = price
	if err != nil && err != feemarket.ErrNamespaceNotFound {
		return err
	}

	return nil
}

func (j *JSONRPCServer) GetCurrentEpoch(_ *http.Request, _ *struct{}, reply *uint64) error {
	*reply = j.vm.GetCurrentEpoch()
	return nil
}

type Validator struct {
	NodeID    ids.NodeID `json:"nodeID"`
	PublicKey []byte     `json:"publicKey"`
	Weight    uint64     `json:"weight"`
}

type NextProposerArgs struct {
	Height uint64 `json:"height"`
}
type NextProposerReply struct {
	PublicKey []byte     `json:"publicKey"`
	NodeID    ids.NodeID `json:"nodeID"`
}

// NextProposer returns the proposer at the given height.
func (j *JSONRPCServer) NextProposer(req *http.Request, args *NextProposerArgs, reply *Validator) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "JSONRPCServer.NextProposer")
	defer span.End()
	validators, _ := j.vm.CurrentValidators(ctx)
	nextProposer, err := j.vm.ProposerAtHeight(ctx, args.Height)
	if err != nil {
		return err
	}

	if _, ok := validators[nextProposer]; !ok {
		j.vm.Logger().Debug("validator set not containing proposer")
		return ErrValSetNot
	}

	reply.NodeID = nextProposer
	v := validators[nextProposer]
	reply.PublicKey = v.PublicKey.Compress()
	reply.Weight = v.Weight

	j.vm.Logger().Debug("proposer info returned", zap.Uint64("height", args.Height), zap.String("nodeID", reply.NodeID.String()), zap.String("pubkey", hexutil.Encode(reply.PublicKey)))
	return nil
}

type GetCurrentValidatorsReply struct {
	Validators []*Validator `json:"validators"`
}

func (j *JSONRPCServer) GetCurrentValidators(req *http.Request, _ *struct{}, reply *GetCurrentValidatorsReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "JSONRPCServer.GetCurrentValidators")
	defer span.End()
	validators, _ := j.vm.CurrentValidators(ctx)

	wValidators := make([]*Validator, 0, len(validators))
	for _, validator := range validators {
		wVal := new(Validator)
		wVal.NodeID = validator.NodeID
		wVal.PublicKey = validator.PublicKey.Compress()
		wVal.Weight = validator.Weight
		wValidators = append(wValidators, wVal)
	}
	reply.Validators = wValidators

	return nil
}

type GetProposerArgs struct {
	Diff  int `json:"diff"`
	Depth int `json:"depth"`
}

type GetProposerReply struct {
	NodeIDs []*ids.NodeID `json:"nodeIDs"`
}

func (j *JSONRPCServer) GetProposer(req *http.Request, args *GetProposerArgs, reply *GetProposerReply) error {
	ctx, span := j.vm.Tracer().Start(req.Context(), "JSONRPCServer.GetProposer")
	defer span.End()
	proposers, err := j.vm.Proposers(ctx, args.Diff, args.Depth)
	if err != nil {
		return err
	}

	for proposer := range proposers {
		reply.NodeIDs = append(reply.NodeIDs, &proposer) //nolint:exportloopref
	}

	return nil
}
