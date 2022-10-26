package codec

import (
	"errors"
	"fmt"
)

// ErrInvalidEncoding is returned when attempting to decoded a message with an invalid encoding.
var ErrInvalidEncoding = errors.New("invalid encoding")

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

// ErrMsgUnmarshal indicates that the message could not be unmarshalled.
type ErrMsgUnmarshal struct {
	code    uint8
	msgType string
	err     string
}

func (e ErrMsgUnmarshal) Error() string {
	return fmt.Sprintf("failed to unmarshal message payload with message type %s and message code %d: %s", e.msgType, e.code, e.err)
}

// NewMsgUnmarshalErr returns a new ErrMsgUnmarshal
func NewMsgUnmarshalErr(code uint8, msgType string, err error) ErrMsgUnmarshal {
	return ErrMsgUnmarshal{code: code, msgType: msgType, err: err.Error()}
}

// IsErrMsgUnmarshal returns true if an error is ErrMsgUnmarshal
func IsErrMsgUnmarshal(err error) bool {
	var e ErrMsgUnmarshal
	return errors.As(err, &e)
}
