package codec

import (
	"errors"
	"fmt"
)

// ErrUnknownMsgCode indicates that the message code byte (first byte of message payload) is unknown.
type ErrUnknownMsgCode struct {
	code uint8
}

func (e ErrUnknownMsgCode) Error() string {
	return fmt.Sprintf("failed to decode message could not get interface from unknown message code: %d", e.code)
}

// NewUnknownMsgCodeErr returns a new ErrUnknownMsgCode
func NewUnknownMsgCodeErr(code uint8) ErrUnknownMsgCode {
	return ErrUnknownMsgCode{code}
}

// IsErrUnknownMsgCode returns true if an error is ErrUnknownMsgCode
func IsErrUnknownMsgCode(err error) bool {
	var e ErrUnknownMsgCode
	return errors.As(err, &e)
}
