// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"

	hactions "github.com/ava-labs/hypersdk/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/spf13/cobra"
)

var actionCmd = &cobra.Command{
	Use: "action",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

var transferCmd = &cobra.Command{
	Use: "transfer",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		_, priv, factory, cli, bcli, ws, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		// Get balance info
		balance, err := handler.GetBalance(ctx, bcli, priv.Address)
		if balance == 0 || err != nil {
			return err
		}

		// Select recipient
		recipient, err := handler.Root().PromptAddress("recipient")
		if err != nil {
			return err
		}

		// Select amount
		amount, err := handler.Root().PromptAmount("amount", consts.Decimals, balance, nil)
		if err != nil {
			return err
		}

		// Confirm action
		cont, err := handler.Root().PromptContinue()
		if !cont || err != nil {
			return err
		}

		// Generate transaction
		_, _, err = sendAndWait(ctx, nil, &actions.Transfer{
			To:    recipient,
			Value: amount,
		}, cli, bcli, ws, factory, true)
		return err
	},
}

var anchorCmd = &cobra.Command{
	Use: "anchor",
	RunE: func(*cobra.Command, []string) error {
		ctx := context.Background()
		_, _, factory, cli, bcli, ws, err := handler.DefaultActor()
		if err != nil {
			return err
		}

		namespaceStr, err := handler.Root().PromptString("namespace", 0, 8)
		if err != nil {
			return err
		}
		namespace := []byte(namespaceStr)
		feeRecipient, err := handler.Root().PromptAddress("feeRecipient")
		if err != nil {
			return err
		}

		op, err := handler.Root().PromptChoice("(0)create (1)delete (2)update", 3)
		if err != nil {
			return err
		}

		info := hactions.AnchorInfo{
			FeeRecipient: feeRecipient,
			Namespace:    namespace,
		}

		// Generate transaction
		_, _, err = sendAndWait(ctx, nil, &actions.AnchorRegister{
			Namespace: namespace,
			Info:      info,
			OpCode:    op,
		}, cli, bcli, ws, factory, true)
		return err
	},
}
