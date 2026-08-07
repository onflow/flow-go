package message

import (
	"errors"
	"fmt"
)

var (
	ErrUnauthorizedUnicastOnChannel = errors.New("message is not authorized to be sent on channel via unicast")
	ErrUnauthorizedPublishOnChannel = errors.New("message is not authorized to be sent on channel via publish/multicast")
	ErrUnauthorizedMessageOnChannel = errors.New("message is not authorized to be sent on channel")
	ErrUnauthorizedRole             = errors.New("sender role not authorized to send message on channel")
)

// UnknownMsgTypeErr indicates that no message auth configured for the message type v
type UnknownMsgTypeErr struct {
	MsgType any
}

func (e UnknownMsgTypeErr) Error() string {
	return fmt.Sprintf("could not get authorization config for unknown message type: %T", e.MsgType)
}

// NewUnknownMsgTypeErr returns a new ErrUnknownMsgType
func NewUnknownMsgTypeErr(msgType any) UnknownMsgTypeErr {
	return UnknownMsgTypeErr{MsgType: msgType}
}

// IsUnknownMsgTypeErr returns whether an error is UnknownMsgTypeErr
func IsUnknownMsgTypeErr(err error) bool {
	var e UnknownMsgTypeErr
	return errors.As(err, &e)
}

// NewUnauthorizedProtocolError returns ErrUnauthorizedUnicastOnChannel or ErrUnauthorizedPublishOnChannel depending on the protocol provided.
func NewUnauthorizedProtocolError(p ProtocolType) error {
	if p == ProtocolTypeUnicast {
		return ErrUnauthorizedUnicastOnChannel
	}

	return ErrUnauthorizedPublishOnChannel
}
