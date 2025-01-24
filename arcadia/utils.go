package arcadia

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/bits-and-blooms/bitset"

	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/emap"
)

var _ emap.Item = (*ArcadiaToSEQChunkMessage)(nil)

func (chunk *ArcadiaToSEQChunkMessage) Initialize(parser chain.Parser) error {
	actionReg, authReg := parser.Registry()
	chunk.sTxs = make([]*chain.Transaction, 0)
	chunk.authCounts = make(map[uint8]int)
	if chunk.Chunk.ToB != nil {
		for _, bundle := range chunk.Chunk.ToB.Bundles {
			err := bundle.Initialize(actionReg, authReg)
			if err != nil {
				return fmt.Errorf("unable to initialize bundle: %s", bundle.BundleHash)
			}
			chunk.sTxs = append(chunk.sTxs, bundle.seqTx)
			chunk.authCounts[bundle.authType] += 1
		}
		bs := bitset.New(uint(len(chunk.sTxs)))
		buf := bytes.NewBuffer(chunk.RemovedBitSet)
		_, err := bs.ReadFrom(buf)
		if err != nil {
			return fmt.Errorf("unable to parse remove bitset, raw: %+v, err: %s", chunk.removedBitSet, err)
		}
		chunk.removedBitSet = *bs
		return nil
	}
	if chunk.Chunk.RoB != nil {
		for _, tx := range chunk.Chunk.RoB.Txs {
			_, stx, err := chain.UnmarshalTxs(tx, 1, actionReg, authReg)
			if err != nil {
				return fmt.Errorf("unable to unmarshal txs")
			}
			if len(stx) == 0 {
				return ErrNoTxsInRoB
			}
			if len(stx) != 1 {
				return fmt.Errorf("expected 1 tx per rob chunk tx, got %d", len(stx))
			}
			chunk.sTxs = append(chunk.sTxs, stx[0])
			chunk.authCounts[stx[0].Auth.GetTypeID()] += 1
		}
		bs := bitset.New(uint(len(chunk.sTxs)))
		buf := bytes.NewBuffer(chunk.RemovedBitSet)
		_, err := bs.ReadFrom(buf)
		if err != nil {
			return fmt.Errorf("unable to parse remove bitset, raw: %+v, err: %s", chunk.removedBitSet, err)
		}
		chunk.removedBitSet = *bs
	}
	return nil
}

func (chunk *ArcadiaToSEQChunkMessage) Expiry() int64 {
	return int64(chunk.Epoch + 1)
}

func (chunk *ArcadiaToSEQChunkMessage) ID() ids.ID {
	return chunk.ChunkID
}

func (b *CrossRollupBundle) Initialize(actionReg chain.ActionRegistry, authReg chain.AuthRegistry) error {
	_, seqTxs, err := chain.UnmarshalTxs(b.Txs, 1, actionReg, authReg)
	if err != nil {
		return err
	}
	if len(seqTxs) != 1 {
		return fmt.Errorf("expected 1 seq tx per bundle, got %d", len(seqTxs))
	}
	seqTx := seqTxs[0]
	b.seqTx = seqTx
	b.authType = seqTx.Auth.GetTypeID()
	return nil
}

func replaceHTTPWithWS(url string) string {
	if strings.HasPrefix(url, "http://") {
		return "ws://" + strings.TrimPrefix(url, "http://")
	} else if strings.HasPrefix(url, "https://") {
		return "wss://" + strings.TrimPrefix(url, "https://")
	}
	return url // Return as-is if no match
}
