package arcadia

import "errors"

// httpErrorResp error type.
type httpErrorResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

var (
	ErrUnexpectedMsgType = errors.New("unexpected message type")
	ErrUnexpectedMsgSize = errors.New("unexpected message size")
)
