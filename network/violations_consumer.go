package network

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
)

type ViolationsConsumer interface {
	// OnUnAuthorizedSenderError logs an error for unauthorized sender error
	OnUnAuthorizedSenderError(violation *Violation) error

	// OnUnknownMsgTypeError logs an error for unknown message type error
	OnUnknownMsgTypeError(violation *Violation) error

	// OnInvalidMsgError logs an error for messages that contained payloads that could not
	// be unmarshalled into the message type denoted by message code byte.
	OnInvalidMsgError(violation *Violation) error

	// OnSenderEjectedError logs an error for sender ejected error
	OnSenderEjectedError(violation *Violation) error

	// OnUnauthorizedUnicastOnChannel logs an error for messages unauthorized to be sent via unicast.
	OnUnauthorizedUnicastOnChannel(violation *Violation) error

	// OnUnauthorizedPublishOnChannel logs an error for messages unauthorized to be sent via pubsub.
	OnUnauthorizedPublishOnChannel(violation *Violation) error

	// OnUnexpectedError logs an error for unknown errors
	OnUnexpectedError(violation *Violation) error
}

type Violation struct {
	Identity *flow.Identity
	PeerID   string
	OriginID flow.Identifier
	MsgType  string
	Channel  channels.Channel
	Protocol message.ProtocolType
	Err      error
}
