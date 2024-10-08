// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"fmt"
	"reflect"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/cli"
	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/examples/morpheusvm/actions"
	"github.com/AnomalyFi/hypersdk/examples/morpheusvm/consts"
	"github.com/AnomalyFi/hypersdk/rpc"
	"github.com/AnomalyFi/hypersdk/utils"

	brpc "github.com/AnomalyFi/hypersdk/examples/morpheusvm/rpc"
)

// sendAndWait may not be used concurrently
func sendAndWait(
	ctx context.Context, actions []chain.Action, cli *rpc.JSONRPCClient,
	bcli *brpc.JSONRPCClient, ws *rpc.WebSocketClient, factory chain.AuthFactory, priorityFee uint64, printStatus bool,
) (bool, ids.ID, error) { //nolint:unparam
	parser, err := bcli.Parser(ctx)
	if err != nil {
		return false, ids.Empty, err
	}
	_, tx, _, err := cli.GenerateTransaction(ctx, parser, actions, factory, priorityFee)
	if err != nil {
		return false, ids.Empty, err
	}
	if err := ws.RegisterTx(tx); err != nil {
		return false, ids.Empty, err
	}
	var result *chain.Result
	for {
		txID, txErr, txResult, err := ws.ListenTx(ctx)
		if err != nil {
			return false, ids.Empty, err
		}
		if txErr != nil {
			return false, ids.Empty, txErr
		}
		if txID == tx.ID() {
			result = txResult
			break
		}
		utils.Outf("{{yellow}}skipping unexpected transaction:{{/}} %s\n", tx.ID())
	}
	if printStatus {
		handler.Root().PrintStatus(tx.ID(), result.Success)
	}
	return result.Success, tx.ID(), nil
}

func handleTx(tx *chain.Transaction, result *chain.Result) {
	actor := tx.Auth.Actor()
	if !result.Success {
		utils.Outf(
			"%s {{yellow}}%s{{/}} {{yellow}}actor:{{/}} %s {{yellow}}error:{{/}} [%s] {{yellow}}fee (max %.2f%%):{{/}} %s %s {{yellow}}consumed:{{/}} [%s]\n",
			"❌",
			tx.ID(),
			codec.MustAddressBech32(consts.HRP, actor),
			result.Error,
			float64(result.Fee)/float64(tx.Base.MaxFee)*100,
			utils.FormatBalance(result.Fee, consts.Decimals),
			consts.Symbol,
			cli.ParseDimensions(result.Units),
		)
		return
	}

	for _, action := range tx.Actions {
		var summaryStr string
		switch act := action.(type) { //nolint:gocritic
		case *actions.Transfer:
			summaryStr = fmt.Sprintf("%s %s -> %s\n", utils.FormatBalance(act.Value, consts.Decimals), consts.Symbol, codec.MustAddressBech32(consts.HRP, act.To))
		}
		utils.Outf(
			"%s {{yellow}}%s{{/}} {{yellow}}actor:{{/}} %s {{yellow}}summary (%s):{{/}} [%s] {{yellow}}fee (max %.2f%%):{{/}} %s %s {{yellow}}consumed:{{/}} [%s]\n",
			"✅",
			tx.ID(),
			codec.MustAddressBech32(consts.HRP, actor),
			reflect.TypeOf(action),
			summaryStr,
			float64(result.Fee)/float64(tx.Base.MaxFee)*100,
			utils.FormatBalance(result.Fee, consts.Decimals),
			consts.Symbol,
			cli.ParseDimensions(result.Units),
		)
	}
}
