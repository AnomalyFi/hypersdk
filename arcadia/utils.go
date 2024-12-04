package arcadia

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/emap"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/bits-and-blooms/bitset"
)

// var _ ChunkInterface = (*ArcadiaToBChunk)(nil)
// var _ ChunkInterface = (*ArcadiaRoBChunk)(nil)
var _ emap.Item = (*ArcadiaToSEQChunkMessage)(nil)

// Note: Before calling, sanity check rollupIDs match RollupIDs in RollupIDToBlockNumber mapping.
// func (tob *ArcadiaToBChunk) Marshal() ([]byte, error) {
// 	rollupIDsLen := len(tob.RollupIDs)
// 	var rollupIDsBytes [][]byte
// 	var rollupIdsArrSize int
// 	for _, rollupID := range tob.RollupIDs {
// 		rollupIDByte, err := hexutil.Decode(rollupID)
// 		if err != nil {
// 			return nil, err
// 		}
// 		rollupIDsBytes = append(rollupIDsBytes, rollupIDByte)
// 		rollupIdsArrSize += len(rollupIDByte)
// 	}

// 	size := (rollupIDsLen+1)*consts.Uint64Len + codec.CummSize(tob.sTxs) + rollupIdsArrSize
// 	p := codec.NewWriter(size, consts.NetworkSizeLimit)

// 	p.PackInt(rollupIDsLen)
// 	for i, rollupID := range tob.RollupIDs {
// 		rollupIDBytes := rollupIDsBytes[i]
// 		p.PackBytes(rollupIDBytes)
// 		p.PackUint64(tob.RollupIDToBlockNumber[rollupID])
// 	}
// 	p.PackInt(len(tob.Txs))
// 	for _, tx := range tob.sTxs {
// 		if err := tx.Marshal(p); err != nil {
// 			return nil, err
// 		}
// 	}
// 	p.PackUint64(tob.Nonce)
// 	bytes := p.Bytes()
// 	if err := p.Err(); err != nil {
// 		return nil, err
// 	}
// 	return bytes, nil
// }

// func (tob *ArcadiaToBChunk) Transactions() []*chain.Transaction {
// 	return tob.sTxs
// }

// func (rob *ArcadiaRoBChunk) Marshal() ([]byte, error) {
// 	rollupIDBytes, err := hexutil.Decode(rob.RollupID)
// 	if err != nil {
// 		return nil, err
// 	}

// 	size := len(rollupIDBytes) + 2*consts.Uint64Len + consts.IntLen + codec.CummSize(rob.sTxs)
// 	p := codec.NewWriter(size, consts.NetworkSizeLimit)

// 	p.PackBytes(rollupIDBytes)
// 	p.PackUint64(rob.BlockNumber)
// 	p.PackInt(len(rob.Txs))
// 	for _, tx := range rob.sTxs {
// 		if err := tx.Marshal(p); err != nil {
// 			return nil, err
// 		}
// 	}
// 	p.PackUint64(rob.Nonce)
// 	bytes := p.Bytes()
// 	if err := p.Err(); err != nil {
// 		return nil, err
// 	}
// 	return bytes, nil
// }

func (chunk *ArcadiaToSEQChunkMessage) Initialize(parser chain.Parser) error {
	actionReg, authReg := parser.Registry()
	chunk.sTxs = make([]*chain.Transaction, 0)
	chunk.authCounts = make(map[uint8]int)
	if chunk.Chunk.ToB != nil {
		for _, bundle := range chunk.Chunk.ToB.Bundles {
			err := bundle.Initialize(actionReg, authReg)
			if err != nil {
				return err
			}
			chunk.sTxs = append(chunk.sTxs, bundle.seqTx)
			chunk.authCounts[bundle.authType] += 1
		}
		bs := bitset.New(uint(len(chunk.sTxs)))
		buf := bytes.NewBuffer(chunk.RemovedBitSet)
		bs.ReadFrom(buf)
		chunk.removedBitSet = *bs
		return nil
	}
	if chunk.Chunk.RoB != nil {
		for _, tx := range chunk.Chunk.RoB.Txs {
			_, stx, err := chain.UnmarshalTxs(tx, 1, actionReg, authReg)
			if err != nil {
				return err
			}
			if len(stx) == 0 {
				return fmt.Errorf("no txs found in rob chunk tx")
			}
			if len(stx) != 1 {
				return fmt.Errorf("expected 1 tx per rob chunk tx, got %d", len(stx))
			}
			chunk.sTxs = append(chunk.sTxs, stx[0])
			chunk.authCounts[stx[0].Auth.GetTypeID()] += 1
		}
		bs := bitset.New(uint(len(chunk.sTxs)))
		buf := bytes.NewBuffer(chunk.RemovedBitSet)
		bs.ReadFrom(buf)
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

// func (rob *ArcadiaRoBChunk) Transactions() []*chain.Transaction {
// 	return rob.sTxs
// }

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

func isContainsInMapping(sarr []string, m map[string]uint64) bool {
	for _, s := range sarr {
		_, ok := m[s]
		if !ok {
			return false
		}
	}
	return true
}

func replaceHTTPWithWS(url string) string {
	if strings.HasPrefix(url, "http://") {
		return "ws://" + strings.TrimPrefix(url, "http://")
	} else if strings.HasPrefix(url, "https://") {
		return "wss://" + strings.TrimPrefix(url, "https://")
	}
	return url // Return as-is if no match
}
