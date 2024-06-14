package fees

import "encoding/json"

type Namespace = []byte

type localFeeMarket struct {
	PriceMap map[string][]byte // byte consists of price + trend + start_block for trend
	RawData  []byte
}

func NewFeeMarket(raw []byte) *localFeeMarket {
	if len(raw) == 0 {
		return &localFeeMarket{
			PriceMap: make(map[string][]byte),
		}
	}
	var lfm localFeeMarket
	json.Unmarshal(raw, &lfm)
	lfm.RawData = raw
	return &lfm
}

func (l *localFeeMarket) Bytes() []byte {
	raw, _ := json.Marshal(l.PriceMap)
	l.RawData = raw
	return raw
}

//@todo have price over ride for large tx data sizes for non-default namespace and implement just in block gas price.
// Note: in SEQ transaction won't fail if maxFee is crossed by transaction.
