package anchor

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/stretchr/testify/require"
)

const ANCHOR_URL = "http://localhost:18550"

type MockVM struct{}

func TestAnchorFlow(t *testing.T) {
	slot := int64(1)
	cli := NewAnchorClient(ANCHOR_URL)
	header, err := cli.GetHeaderV2(slot)
	require.NoError(t, err)
	fmt.Printf("%+v\n", header)

	_, err = cli.GetPayloadV2(slot)
	require.NoError(t, err)
}

// Avalanchego also use BLS12-381 scheme as go-eth2-client
func TestBLSSigning(t *testing.T) {
	chainID := ids.GenerateTestID()
	networkID := 1337
	sk, err := bls.NewSecretKey()
	require.NoError(t, err)
	pubkey := bls.PublicFromSecretKey(sk)

	payload := make([]byte, 20)
	_, err = rand.Read(payload)
	require.NoError(t, err)

	warpSigner := warp.NewSigner(sk, 1337, chainID)
	uwm, err := warp.NewUnsignedMessage(uint32(networkID), chainID, payload)
	require.NoError(t, err)
	sig, err := warpSigner.Sign(uwm)
	require.NoError(t, err)

	fmt.Printf("compressed pubkey length: %d\n", len(pubkey.Compress()))
	fmt.Printf("pubkey length: %d\n", len(pubkey.Serialize()))
	fmt.Printf("sig length: %d\n", len(sig))
}
