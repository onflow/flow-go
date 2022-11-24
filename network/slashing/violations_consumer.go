package slashing

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/channels"
)

type ViolationsConsumer interface {
	// OnUnAuthorizedSenderError logs an error for unauthorized sender error
	OnUnAuthorizedSenderError(violation *Violation)

	// OnUnknownMsgTypeError logs an error for unknown message type error
	OnUnknownMsgTypeError(violation *Violation)

	// OnInvalidMsgError logs an error for messages that contained payloads that could not
	// be unmarshalled into the message type denoted by message code byte.
	OnInvalidMsgError(violation *Violation)

	// OnSenderEjectedError logs an error for sender ejected error
	OnSenderEjectedError(violation *Violation)

	// OnUnauthorizedUnicastOnChannel logs an error for messages unauthorized to be sent via unicast
	OnUnauthorizedUnicastOnChannel(violation *Violation)

	// OnUnexpectedError logs an error for unknown errors
	OnUnexpectedError(violation *Violation)
}

type Violation struct {
	Identity  *flow.Identity
	PeerID    string
	MsgType   string
	Channel   channels.Channel
	IsUnicast bool
	Err       error
}
