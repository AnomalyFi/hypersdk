package feemarket

import (
	"testing"
	"time"

	"github.com/AnomalyFi/hypersdk/window"
	"github.com/stretchr/testify/require"
)

var (
	dummyNS1 = "namespace1"
	dummyNS2 = "namespace2"
)

var _ Rules = (*MRules)(nil)

type MRules struct{}

func (*MRules) GetFeeMarketPriceChangeDenominator() uint64 { return 48 }

func (*MRules) GetFeeMarketWindowTargetUnits() uint64 { return 600 /*KiB*/ }

func (*MRules) GetFeeMarketMinUnitPrice() uint64 { return 10_000 }

func TestMinUnitPriceForNewNameSpace(t *testing.T) {
	rules := new(MRules)
	market := NewMarket(nil, rules)
	price, err := market.UnitPrice(dummyNS1)
	require.Equal(t, uint64(10_000), price)
	require.ErrorIs(t, err, ErrNamespaceNotFound)
}

func TestSetUnitPriceForNameSpace(t *testing.T) {
	rules := new(MRules)
	market := NewMarket(nil, rules)
	market.SetUnitPrice(dummyNS1, 100_000)
	price, err := market.UnitPrice(dummyNS1)
	require.Equal(t, uint64(100_000), price)
	require.NoError(t, err)
}

func TestUnInitLastConsumedForNameSpace(t *testing.T) {
	rules := new(MRules)
	market := NewMarket(nil, rules)
	consumed := market.LastConsumed(dummyNS1)
	require.Equal(t, uint64(0), consumed)
}

func TestSetLastConsumedForNameSpace(t *testing.T) {
	rules := new(MRules)
	market := NewMarket(nil, rules)
	market.SetLastConsumed(dummyNS1, 100)
	consumed := market.LastConsumed(dummyNS1)
	require.Equal(t, uint64(100), consumed)
}

func TestMultipleConsumeForNameSpace(t *testing.T) {
	rules := new(MRules)
	market := NewMarket(nil, rules)
	ts := time.Now().Unix()
	market.Consume(dummyNS1, 100, ts)
	require.Equal(t, uint64(100), market.LastConsumed(dummyNS1))
	market.Consume(dummyNS1, 200, ts)
	require.Equal(t, uint64(300), market.LastConsumed(dummyNS1))
}

func TestUnInitWindowForNameSpace(t *testing.T) {
	rules := new(MRules)
	market := NewMarket(nil, rules)
	w := market.Window(dummyNS1)
	a := window.Window(make([]byte, window.WindowSliceSize))
	require.Equal(t, a, w)
}

func TestUnInitTimeStampForNameSpace(t *testing.T) {
	rules := new(MRules)
	market := NewMarket(nil, rules)
	ts := market.LastTimeStamp(dummyNS1)
	require.Equal(t, int64(0), ts)
}

func TestSetTimeStampForNameSpace(t *testing.T) {
	rules := new(MRules)
	market := NewMarket(nil, rules)
	ts := time.Now().Unix()
	market.SetLastTimeStamp(dummyNS1, ts)
	lts := market.LastTimeStamp(dummyNS1)
	require.Equal(t, ts, lts)
}

func TestComputeNext(t *testing.T) {
	rules := new(MRules)
	market := NewMarket(nil, rules)
	timeNow := time.Now()
	ts := timeNow.UnixMilli()
	// test minimum price
	market.Consume(dummyNS1, 100, ts)
	market.Consume(dummyNS2, 300, ts)
	market, err := market.ComputeNext(ts, timeNow.Add(time.Second).UnixMilli(), rules)
	require.NoError(t, err)
	price1, err := market.UnitPrice(dummyNS1)
	require.Equal(t, rules.GetFeeMarketMinUnitPrice(), price1)
	require.NoError(t, err)
	price2, err := market.UnitPrice(dummyNS2)
	require.Equal(t, rules.GetFeeMarketMinUnitPrice(), price2)
	require.NoError(t, err)
	// test price increase
	timeNow = timeNow.Add(time.Second)
	ts = timeNow.UnixMilli()
	market.Consume(dummyNS2, 400, ts)
	market, err = market.ComputeNext(ts, timeNow.Add(time.Second).UnixMilli(), rules)
	require.NoError(t, err)
	price2, err = market.UnitPrice(dummyNS2)
	newPrice := rules.GetFeeMarketMinUnitPrice() + rules.GetFeeMarketMinUnitPrice()/rules.GetFeeMarketPriceChangeDenominator()
	require.Equal(t, newPrice, price2)
	require.NoError(t, err)
	// test namespace deletion
	market, err = market.ComputeNext(ts, timeNow.Add(10*time.Second).UnixMilli(), rules)
	require.NoError(t, err)
	_, ok := market.NameSpaceToUtilityMap[dummyNS1]
	require.Equal(t, false, ok)
	// test unit price for no namespace
	price1, err = market.UnitPrice(dummyNS1)
	require.Equal(t, rules.GetFeeMarketMinUnitPrice(), price1)
	require.Error(t, ErrNamespaceNotFound, err)
}
