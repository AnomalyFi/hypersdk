package arcadia

import (
	"bytes"
	"strings"

	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/consts"
	"github.com/AnomalyFi/hypersdk/emap"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/bits-and-blooms/bitset"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

var _ ChunkInterface = (*ArcadiaToBChunk)(nil)
var _ ChunkInterface = (*ArcadiaRoBChunk)(nil)
var _ emap.Item = (*ArcadiaChunk)(nil)

// Note: Before calling, sanity check rollupIDs match RollupIDs in RollupIDToBlockNumber mapping.
func (tob *ArcadiaToBChunk) Marshal() ([]byte, error) {
	rollupIDsLen := len(tob.RollupIDs)
	var rollupIDsBytes [][]byte
	var rollupIdsArrSize int
	for _, rollupID := range tob.RollupIDs {
		rollupIDByte, err := hexutil.Decode(rollupID)
		if err != nil {
			return nil, err
		}
		rollupIDsBytes = append(rollupIDsBytes, rollupIDByte)
		rollupIdsArrSize += len(rollupIDByte)
	}

	size := (rollupIDsLen+1)*consts.Uint64Len + codec.CummSize(tob.sTxs) + rollupIdsArrSize
	p := codec.NewWriter(size, consts.NetworkSizeLimit)

	p.PackInt(rollupIDsLen)
	for i, rollupID := range tob.RollupIDs {
		rollupIDBytes := rollupIDsBytes[i]
		p.PackBytes(rollupIDBytes)
		p.PackUint64(tob.RollupIDToBlockNumber[rollupID])
	}
	p.PackInt(len(tob.Txs))
	for _, tx := range tob.sTxs {
		if err := tx.Marshal(p); err != nil {
			return nil, err
		}
	}
	p.PackUint64(tob.Nonce)
	bytes := p.Bytes()
	if err := p.Err(); err != nil {
		return nil, err
	}
	return bytes, nil
}

func (tob *ArcadiaToBChunk) Transactions() []*chain.Transaction {
	return tob.sTxs
}

func (rob *ArcadiaRoBChunk) Marshal() ([]byte, error) {
	rollupIDBytes, err := hexutil.Decode(rob.RollupID)
	if err != nil {
		return nil, err
	}

	size := len(rollupIDBytes) + 2*consts.Uint64Len + consts.IntLen + codec.CummSize(rob.sTxs)
	p := codec.NewWriter(size, consts.NetworkSizeLimit)

	p.PackBytes(rollupIDBytes)
	p.PackUint64(rob.BlockNumber)
	p.PackInt(len(rob.Txs))
	for _, tx := range rob.sTxs {
		if err := tx.Marshal(p); err != nil {
			return nil, err
		}
	}
	p.PackUint64(rob.Nonce)
	bytes := p.Bytes()
	if err := p.Err(); err != nil {
		return nil, err
	}
	return bytes, nil
}

func (chunk *ArcadiaChunk) Initialize(parser chain.Parser) error {
	actionReg, authReg := parser.Registry()
	if chunk.ToBChunk != nil {
		ac, txs, err := chain.UnmarshalTxs(chunk.ToBChunk.Txs, 1, actionReg, authReg)
		if err != nil {
			return err
		}
		chunk.ToBChunk.sTxs = txs
		bs := bitset.New(uint(len(txs)))
		buf := bytes.NewBuffer(chunk.ToBChunk.RemovedBitSet)
		bs.ReadFrom(buf)
		chunk.authCounts = ac
		return nil
	}
	if chunk.RoBChunk != nil {
		ac, txs, err := chain.UnmarshalTxs(chunk.RoBChunk.Txs, 1, actionReg, authReg)
		if err != nil {
			return err
		}
		chunk.RoBChunk.sTxs = txs
		bs := bitset.New(uint(len(txs)))
		buf := bytes.NewBuffer(chunk.ToBChunk.RemovedBitSet)
		bs.ReadFrom(buf)
		chunk.RoBChunk.bs = bs
		chunk.authCounts = ac
	}
	return nil
}

func (chunk *ArcadiaChunk) Expiry() int64 {
	return int64(chunk.Epoch + 1)
}

func (chunk *ArcadiaChunk) ID() ids.ID {
	return chunk.ChunkID
}

func (rob *ArcadiaRoBChunk) Transactions() []*chain.Transaction {
	return rob.sTxs
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
