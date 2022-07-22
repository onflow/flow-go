package slashing

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/utils/logging"

	"github.com/onflow/flow-go/model/flow"
)

const (
	unAuthorizedSenderViolation = "unauthorized_sender"
	unknownMsgTypeViolation     = "unknown_message_type"
	senderEjectedViolation      = "sender_ejected"
)

// Consumer is a struct that logs a message for any slashable offences.
// This struct will be updated in the future when slashing is implemented.
type Consumer struct {
	log zerolog.Logger
}

// NewSlashingViolationsConsumer returns a new Consumer
func NewSlashingViolationsConsumer(log zerolog.Logger) *Consumer {
	return &Consumer{log}
}

func (c *Consumer) logEvent(identity *flow.Identity, peerID, msgType, offense string, err error) {
	e := c.log.Error().
		Str("peer_id", peerID).
		Str("offense", offense)

	if msgType != "" {
		e.Str("message_type", msgType)
	}

	if identity != nil {
		e.Str("role", identity.Role.String()).Hex("sender_id", logging.ID(identity.NodeID))
	}

	e.Msg(fmt.Sprintf("potential slashable offense: %s", err))
}

// OnUnAuthorizedSenderError logs an error for unauthorized sender error
func (c *Consumer) OnUnAuthorizedSenderError(identity *flow.Identity, peerID, msgType string, err error) {
	c.logEvent(identity, peerID, msgType, unAuthorizedSenderViolation, err)
}

// OnUnknownMsgTypeError logs an error for unknown message type error
func (c *Consumer) OnUnknownMsgTypeError(identity *flow.Identity, peerID, msgType string, err error) {
	c.logEvent(identity, peerID, msgType, unknownMsgTypeViolation, err)
}

// OnSenderEjectedError logs an error for sender ejected error
func (c *Consumer) OnSenderEjectedError(identity *flow.Identity, peerID, msgType string, err error) {
	c.logEvent(identity, peerID, msgType, senderEjectedViolation, err)
}

// SetLogger overrides the configured logger allowing the user to add more log context
func (c *Consumer) SetLogger(logger zerolog.Logger) {
	c.log = logger
}
