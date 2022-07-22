package slashing

import (
	"github.com/onflow/flow-go/model/flow"
)

type ViolationsConsumer interface {
	// OnUnAuthorizedSenderError logs an error for unauthorized sender error
	OnUnAuthorizedSenderError(identity *flow.Identity, peerID, msgType, channel string, isUnicast bool, err error)

	// OnUnknownMsgTypeError logs an error for unknown message type error
	OnUnknownMsgTypeError(identity *flow.Identity, peerID, msgType, channel string, isUnicast bool, err error)

	// OnSenderEjectedError logs an error for sender ejected error
	OnSenderEjectedError(identity *flow.Identity, peerID, msgType, channel string, isUnicast bool, err error)
}
