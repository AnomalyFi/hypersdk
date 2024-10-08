// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/spf13/cobra"

	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/cli"
	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/crypto/bls"
	"github.com/AnomalyFi/hypersdk/crypto/ed25519"
	"github.com/AnomalyFi/hypersdk/crypto/secp256r1"
	"github.com/AnomalyFi/hypersdk/examples/morpheusvm/actions"
	"github.com/AnomalyFi/hypersdk/examples/morpheusvm/auth"
	"github.com/AnomalyFi/hypersdk/examples/morpheusvm/consts"
	"github.com/AnomalyFi/hypersdk/pubsub"
	"github.com/AnomalyFi/hypersdk/rpc"
	"github.com/AnomalyFi/hypersdk/utils"

	brpc "github.com/AnomalyFi/hypersdk/examples/morpheusvm/rpc"
)

func getFactory(priv *cli.PrivateKey) (chain.AuthFactory, error) {
	switch priv.Address[0] {
	case consts.ED25519ID:
		return auth.NewED25519Factory(ed25519.PrivateKey(priv.Bytes)), nil
	case consts.SECP256R1ID:
		return auth.NewSECP256R1Factory(secp256r1.PrivateKey(priv.Bytes)), nil
	case consts.BLSID:
		p, err := bls.PrivateKeyFromBytes(priv.Bytes)
		if err != nil {
			return nil, err
		}
		return auth.NewBLSFactory(p), nil
	default:
		return nil, ErrInvalidKeyType
	}
}

var spamCmd = &cobra.Command{
	Use: "spam",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var runSpamCmd = &cobra.Command{
	Use: "run [ed25519/secp256r1/bls]",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return ErrInvalidArgs
		}
		return checkKeyType(args[0])
	},
	RunE: func(_ *cobra.Command, args []string) error {
		var bclient *brpc.JSONRPCClient
		var wclient *rpc.WebSocketClient
		var maxFeeParsed *uint64
		if maxFee >= 0 {
			v := uint64(maxFee)
			maxFeeParsed = &v
		}
		return handler.Root().Spam(maxTxBacklog, maxFeeParsed, randomRecipient,
			func(uri string, networkID uint32, chainID ids.ID) error { // createClient
				bclient = brpc.NewJSONRPCClient(uri, networkID, chainID)
				ws, err := rpc.NewWebSocketClient(uri, rpc.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize)
				if err != nil {
					return err
				}
				wclient = ws
				return nil
			},
			getFactory,
			func() (*cli.PrivateKey, error) { // createAccount
				return generatePrivateKey(args[0])
			},
			func(choice int, address string) (uint64, error) { // lookupBalance
				balance, err := bclient.Balance(context.TODO(), address)
				if err != nil {
					return 0, err
				}
				utils.Outf(
					"%d) {{cyan}}address:{{/}} %s {{cyan}}balance:{{/}} %s %s\n",
					choice,
					address,
					utils.FormatBalance(balance, consts.Decimals),
					consts.Symbol,
				)
				return balance, err
			},
			func(ctx context.Context, chainID ids.ID) (chain.Parser, error) { // getParser
				return bclient.Parser(ctx)
			},
			func(addr codec.Address, amount uint64) []chain.Action { // getTransfer
				return []chain.Action{&actions.Transfer{
					To:    addr,
					Value: amount,
				}}
			},
			func(cli *rpc.JSONRPCClient, priv *cli.PrivateKey) func(context.Context, uint64) error { // submitDummy
				return func(ictx context.Context, count uint64) error {
					factory, err := getFactory(priv)
					if err != nil {
						return err
					}
					_, _, err = sendAndWait(ictx, []chain.Action{&actions.Transfer{
						To:    priv.Address,
						Value: count, // prevent duplicate txs
					}}, cli, bclient, wclient, factory, 0, false)
					return err
				}
			},
		)
	},
}
