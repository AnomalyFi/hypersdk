package feemarket

import (
	"encoding/binary"
	"encoding/json"
	"sync"

	"github.com/ava-labs/avalanchego/utils/math"

	"github.com/AnomalyFi/hypersdk/consts"
	"github.com/AnomalyFi/hypersdk/window"
)

const (
	// Window = WindowSize(10) * unit64(8 bytes)
	// Price(8 bytes) | Window(80 bytes) | LastConsumed(8 bytes) | LastUsedTimestamp(8 bytes)
	utilityStateLen = consts.Uint64Len + window.WindowSliceSize + consts.Uint64Len + consts.Int64Len
)

// Market keeps track of bandwidth used by namespace in a block
type Market struct {
	NameSpaceToUtilityMap map[string][utilityStateLen]byte `json:"namespace_to_utility_map"`
	l                     sync.RWMutex
	minUnitPrice          uint64
}

func NewMarket(raw []byte, r Rules) *Market {
	if len(raw) == 0 {
		return &Market{
			NameSpaceToUtilityMap: make(map[string][utilityStateLen]byte),
			minUnitPrice:          r.GetFeeMarketMinUnitPrice(),
		}
	}
	var m Market
	_ = json.Unmarshal(raw, &m.NameSpaceToUtilityMap) // error should never occur
	m.minUnitPrice = r.GetFeeMarketMinUnitPrice()
	return &m
}

func (m *Market) Bytes() []byte {
	m.l.Lock()
	b, _ := json.Marshal(m.NameSpaceToUtilityMap)
	m.l.Unlock()
	return b
}

func (m *Market) SetUnitPrice(namespace string, price uint64) {
	m.l.Lock()
	defer m.l.Unlock()
	utility, ok := m.NameSpaceToUtilityMap[namespace]
	if !ok {
		utility = [utilityStateLen]byte{}
	}
	binary.BigEndian.PutUint64(utility[:consts.Uint64Len], price)
	m.NameSpaceToUtilityMap[namespace] = utility
}

// returns minimum unit price in case of error or a new namespace.
func (m *Market) UnitPrice(namespace string) (uint64, error) {
	m.l.RLock()
	utility, ok := m.NameSpaceToUtilityMap[namespace]
	m.l.RUnlock()
	if !ok {
		return m.minUnitPrice, ErrNamespaceNotFound
	}
	price := binary.BigEndian.Uint64(utility[:consts.Uint64Len])
	if price < m.minUnitPrice {
		price = m.minUnitPrice
	}
	return price, nil
}

func (m *Market) Window(namespace string) window.Window {
	m.l.RLock()
	utility, ok := m.NameSpaceToUtilityMap[namespace]
	m.l.RUnlock()
	if !ok {
		utility = [utilityStateLen]byte{}
	}
	return window.Window(utility[consts.Uint64Len : consts.Uint64Len+window.WindowSliceSize])
}

func (m *Market) SetLastConsumed(namespace string, consumed uint64) {
	m.l.Lock()
	defer m.l.Unlock()
	utility, ok := m.NameSpaceToUtilityMap[namespace]
	if !ok {
		utility = [utilityStateLen]byte{}
	}
	binary.BigEndian.PutUint64(utility[consts.Uint64Len+window.WindowSliceSize:consts.Uint64Len+window.WindowSliceSize+consts.Uint64Len], consumed)
	m.NameSpaceToUtilityMap[namespace] = utility
}

// returns last consumed bytes by namespace.
func (m *Market) LastConsumed(namespace string) uint64 {
	m.l.RLock()
	utility, ok := m.NameSpaceToUtilityMap[namespace]
	m.l.RUnlock()
	if !ok {
		utility = [utilityStateLen]byte{}
	}
	return binary.BigEndian.Uint64(utility[consts.Uint64Len+window.WindowSliceSize : consts.Uint64Len+window.WindowSliceSize+consts.Uint64Len])
}

func (m *Market) SetLastTimeStamp(namespace string, timeStamp int64) {
	m.l.Lock()
	defer m.l.Unlock()
	utility, ok := m.NameSpaceToUtilityMap[namespace]
	if !ok {
		utility = [utilityStateLen]byte{}
	}
	binary.BigEndian.PutUint64(utility[consts.Uint64Len+window.WindowSliceSize+consts.Uint64Len:], uint64(timeStamp))
	m.NameSpaceToUtilityMap[namespace] = utility
}

func (m *Market) LastTimeStamp(namespace string) int64 {
	m.l.RLock()
	utility, ok := m.NameSpaceToUtilityMap[namespace]
	m.l.RUnlock()
	if !ok {
		utility = [utilityStateLen]byte{}
	}
	return int64(binary.BigEndian.Uint64(utility[consts.Uint64Len+window.WindowSliceSize+consts.Uint64Len:]))
}

// Consume keeps track of total bytes used by a namespace for the block.
func (m *Market) Consume(namespace string, units uint64, timeStamp int64) {
	lastConsumed := m.LastConsumed(namespace)
	m.SetLastConsumed(namespace, lastConsumed+units)
	m.SetLastTimeStamp(namespace, timeStamp)
}

// ComputeNext gives a new market that can be used, for determining fees for next block.
func (m *Market) ComputeNext(lastTime, currTime int64, r Rules) (*Market, error) {
	m.l.Lock()
	defer m.l.Unlock()
	targetUnitsPerNS := r.GetFeeMarketWindowTargetUnits()
	unitPriceChangeDenom := r.GetFeeMarketPriceChangeDenominator()
	minUnitPrice := r.GetFeeMarketMinUnitPrice()
	for namespace, utility := range m.NameSpaceToUtilityMap {
		since := int((currTime - lastTime) / consts.MillisecondsPerSecond)
		previousConsumed := binary.BigEndian.Uint64(utility[consts.Uint64Len+window.WindowSliceSize : consts.Uint64Len+window.WindowSliceSize+consts.Uint64Len])
		previousWindow := window.Window(utility[consts.Uint64Len : consts.Uint64Len+window.WindowSliceSize])
		newRollupWindow, err := window.Roll(previousWindow, since)
		if err != nil {
			return nil, err
		}
		if since < window.WindowSize {
			slot := window.WindowSize - 1 - since
			start := slot * consts.Uint64Len
			window.Update(&newRollupWindow, start, previousConsumed)
		}
		total := window.Sum(newRollupWindow)
		prevPrice := binary.BigEndian.Uint64(utility[:consts.Uint64Len])
		nextPrice := prevPrice
		// the fee market price adjustments happens below.
		if total > targetUnitsPerNS {
			delta := nextPrice / unitPriceChangeDenom
			n, over := math.Add64(nextPrice, delta)
			if over != nil {
				nextPrice = consts.MaxUint64
			} else {
				nextPrice = n
			}
		} else if total < targetUnitsPerNS {
			delta := nextPrice / unitPriceChangeDenom
			n, under := math.Sub(nextPrice, delta)
			if under != nil {
				nextPrice = 0
			} else {
				nextPrice = n
			}
		}
		if nextPrice < minUnitPrice {
			nextPrice = minUnitPrice
		}
		lastUpdatedTimeStamp := binary.BigEndian.Uint64(utility[consts.Uint64Len+window.WindowSliceSize+consts.Uint64Len:])
		if int(uint64(currTime)-lastUpdatedTimeStamp)/consts.MillisecondsPerSecond > window.WindowSize && nextPrice == minUnitPrice {
			// if namespace has no activity for more than window size, delete namespace from the map.
			delete(m.NameSpaceToUtilityMap, namespace)
			continue
		}
		// update namespace with new price and window.
		copy(utility[:consts.Uint64Len], binary.BigEndian.AppendUint64(nil, nextPrice))
		copy(utility[consts.Uint64Len:consts.Uint64Len+window.WindowSliceSize], newRollupWindow[:])
		copy(utility[consts.Uint64Len+window.WindowSliceSize:consts.Uint64Len+window.WindowSliceSize+consts.Uint64Len], make([]byte, 8))
		m.NameSpaceToUtilityMap[namespace] = utility
	}
	return &Market{NameSpaceToUtilityMap: m.NameSpaceToUtilityMap, minUnitPrice: minUnitPrice}, nil
}

func (m *Market) Fee(namespace string, units uint64) (uint64, error) {
	unitPrice, err := m.UnitPrice(namespace)
	if err != nil {
		unitPrice = m.minUnitPrice
	}
	return math.Mul64(unitPrice, units)
}
