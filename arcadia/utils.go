package arcadia

import (
	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/consts"
	"github.com/AnomalyFi/hypersdk/emap"
	"github.com/ava-labs/avalanchego/ids"
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

	size := (rollupIDsLen+1)*consts.Uint64Len + codec.CummSize(tob.Txs) + rollupIdsArrSize
	p := codec.NewWriter(size, consts.NetworkSizeLimit)

	p.PackInt(rollupIDsLen)
	for i, rollupID := range tob.RollupIDs {
		rollupIDBytes := rollupIDsBytes[i]
		p.PackBytes(rollupIDBytes)
		p.PackUint64(tob.RollupIDToBlockNumber[rollupID])
	}
	p.PackInt(len(tob.Txs))
	for _, tx := range tob.Txs {
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
	return tob.Txs
}

func (rob *ArcadiaRoBChunk) Marshal() ([]byte, error) {
	rollupIDBytes, err := hexutil.Decode(rob.RollupID)
	if err != nil {
		return nil, err
	}

	size := len(rollupIDBytes) + 2*consts.Uint64Len + consts.IntLen + codec.CummSize(rob.Txs)
	p := codec.NewWriter(size, consts.NetworkSizeLimit)

	p.PackBytes(rollupIDBytes)
	p.PackUint64(rob.BlockNumber)
	p.PackInt(len(rob.Txs))
	for _, tx := range rob.Txs {
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

func (chunk *ArcadiaChunk) Expiry() int64 {
	return int64(chunk.Epoch + 1)
}

func (chunk *ArcadiaChunk) ID() ids.ID {
	return chunk.ChunkID
}

func (rob *ArcadiaRoBChunk) Transactions() []*chain.Transaction {
	return rob.Txs
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
