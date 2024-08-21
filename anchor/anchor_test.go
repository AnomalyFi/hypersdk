package anchor

import (
	"fmt"
	"testing"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

const ANCHOR_URL = "http://localhost:18550"

type MockVM struct{}

func TestAnchorFlow(t *testing.T) {
	slot := int64(1)
	cli := NewAnchorClient(nil, ANCHOR_URL)
	header, err := cli.GetHeaderV2(slot)
	require.NoError(t, err)
	fmt.Printf("%+v\n", header)

	payload, err := cli.GetPayloadV2(slot)
	require.NoError(t, err)

	tobPayload := payload.ToBPayload
	txs := tobPayload.Transactions
	fmt.Printf("ToB txs: ")
	for _, txRaw := range txs {
		var tx ethtypes.Transaction
		err := tx.UnmarshalBinary(txRaw)
		require.NoError(t, err)
		chainID := tx.ChainId()
		fmt.Printf("txHash: %s\tchainID: %d\n", tx.Hash().Hex(), chainID.Int64())
	}

	for chainID, robPayload := range payload.RoBPayloads {
		fmt.Printf("RoB-%s txs\n", chainID)
		txs := robPayload.Transactions
		for _, txRaw := range txs {
			var tx ethtypes.Transaction
			err := tx.UnmarshalBinary(txRaw)
			require.NoError(t, err)
			chainID := tx.ChainId()
			fmt.Printf("txHash: %s\tchainID: %d\n", tx.Hash().Hex(), chainID.Int64())
		}
	}
}
