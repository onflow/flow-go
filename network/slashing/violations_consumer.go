package slashing

import (
	"github.com/onflow/flow-go/model/flow"
	network "github.com/onflow/flow-go/network/channels"
)

type ViolationsConsumer interface {
	// OnUnAuthorizedSenderError logs an error for unauthorized sender error
	OnUnAuthorizedSenderError(violation *Violation)

	// OnUnknownMsgTypeError logs an error for unknown message type error
	OnUnknownMsgTypeError(violation *Violation)

	// OnSenderEjectedError logs an error for sender ejected error
	OnSenderEjectedError(violation *Violation)
}

type Violation struct {
	Identity  *flow.Identity
	PeerID    string
	MsgType   string
	Channel   network.Channel
	IsUnicast bool
	Err       error
}
