package slashing

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
)

type ViolationsConsumer interface {
	// OnUnAuthorizedSenderError logs an error for unauthorized sender error
	OnUnAuthorizedSenderError(identity *flow.Identity, peerID, msgType string, err error)

	// OnUnknownMsgTypeError logs an error for unknown message type error
	OnUnknownMsgTypeError(identity *flow.Identity, peerID, msgType string, err error)

	// OnSenderEjectedError logs an error for sender ejected error
	OnSenderEjectedError(identity *flow.Identity, peerID, msgType string, err error)

	// SetLogger overrides the configured logger allowing the user to add more log context
	SetLogger(logger zerolog.Logger)
}
