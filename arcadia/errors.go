package arcadia

import "errors"

// httpErrorResp error type.
type httpErrorResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

var (
	ErrUnexpectedMsgType                = errors.New("unexpected message type")
	ErrUnexpectedMsgSize                = errors.New("unexpected message size")
	ErrChunkWithBothToBAndRoB           = errors.New("received chunk with both ToB and RoB chunks")
	ErrChunkWithNoToBAndRoB             = errors.New("received chunk with no ToB or RoB chunks")
	ErrInvalidBitSetLengthMisMatch      = errors.New("txs and removed bitset length mismatch")
	ErrBuilderSignature                 = errors.New("wrong builder signature")
	ErrLessThanTwoActions               = errors.New("tx with less than 2 actions found in ToB chunk")
	ErrToBChunkWithNonAcceptableActions = errors.New("ToB chunk transaction with non-transfer or sequencer action found")
	ErrMoreThanOneAction                = errors.New("tx with more than 1 actions found in RoB chunk")
	ErrNonSequencerMessage              = errors.New("RoB chunk transaction with non-sequencer action found")
	ErrNoTxsInRoB                       = errors.New("no txs found in rob chunk tx")
)
