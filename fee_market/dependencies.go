package feemarket

type Rules interface {
	GetFeeMarketPriceChangeDenominator() uint64
	GetFeeMarketWindowTargetUnits() uint64
	GetFeeMarketMinUnitPrice() uint64
}
