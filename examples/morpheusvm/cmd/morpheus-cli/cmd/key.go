// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"

	"github.com/AnomalyFi/hypersdk/crypto/ed25519"
	"github.com/AnomalyFi/hypersdk/examples/morpheusvm/consts"
	brpc "github.com/AnomalyFi/hypersdk/examples/morpheusvm/rpc"
	"github.com/AnomalyFi/hypersdk/utils"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"
)

var keyCmd = &cobra.Command{
	Use: "key",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var genKeyCmd = &cobra.Command{
	Use: "generate",
	RunE: func(*cobra.Command, []string) error {
		return handler.Root().GenerateKey()
	},
}

var importKeyCmd = &cobra.Command{
	Use: "import [path]",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return ErrInvalidArgs
		}
		return nil
	},
	RunE: func(_ *cobra.Command, args []string) error {
		return handler.Root().ImportKey(args[0])
	},
}

func lookupSetKeyBalance(choice int, address string, uri string, networkID uint32, chainID ids.ID) error {
	// TODO: just load once
	cli := brpc.NewJSONRPCClient(uri, networkID, chainID)
	balance, err := cli.Balance(context.TODO(), address)
	if err != nil {
		return err
	}
	utils.Outf(
		"%d) {{cyan}}address:{{/}} %s {{cyan}}balance:{{/}} %s %s\n",
		choice,
		address,
		utils.FormatBalance(balance, consts.Decimals),
		consts.Symbol,
	)
	return nil
}

var setKeyCmd = &cobra.Command{
	Use: "set",
	RunE: func(*cobra.Command, []string) error {
		return handler.Root().SetKey(lookupSetKeyBalance)
	},
}

func lookupKeyBalance(pk ed25519.PublicKey, uri string, networkID uint32, chainID ids.ID, _ ids.ID) error {
	_, err := handler.GetBalance(context.TODO(), brpc.NewJSONRPCClient(uri, networkID, chainID), pk)
	return err
}

var balanceKeyCmd = &cobra.Command{
	Use: "balance",
	RunE: func(*cobra.Command, []string) error {
		return handler.Root().Balance(checkAllChains, false, lookupKeyBalance)
	},
}
