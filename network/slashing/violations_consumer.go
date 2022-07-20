package slashing

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/utils/logging"

	"github.com/onflow/flow-go/model/flow"
)

const (
	unAuthorizedSenderViolation        = "unauthorized_sender"
	unAuthorizedSenderUnicastViolation = "unauthorized_sender_unicast"
	unknownMsgTypeViolation            = "unknown_message_type"
	senderEjectedViolation             = "sender_ejected"
)

// SlashingViolationsConsumer is a struct that logs a message for any slashable offences.
// This struct will be updated in the future when slashing is implemented.
type SlashingViolationsConsumer struct {
	log zerolog.Logger
}

// NewSlashingViolationsConsumer returns a new SlashingViolationsConsumer
func NewSlashingViolationsConsumer(log zerolog.Logger) *SlashingViolationsConsumer {
	return &SlashingViolationsConsumer{log}
}

// OnUnAuthorizedSenderError logs an error for unauthorized sender error
func (c *SlashingViolationsConsumer) OnUnAuthorizedSenderError(identity *flow.Identity, peerID, msgType string, err error) {
	c.log.Error().
		Str("peer_id", peerID).
		Str("role", identity.Role.String()).
		Hex("sender_id", logging.ID(identity.NodeID)).
		Str("message_type", msgType).
		Str("offense", unAuthorizedSenderViolation).
		Msg(fmt.Sprintf("potential slashable offense: %s", err))
}

// OnUnknownMsgTypeError logs an error for unknown message type error
func (c *SlashingViolationsConsumer) OnUnknownMsgTypeError(identity *flow.Identity, peerID, msgType string, err error) {
	c.log.Error().
		Str("peer_id", peerID).
		Str("role", identity.Role.String()).
		Hex("sender_id", logging.ID(identity.NodeID)).
		Str("message_type", msgType).
		Str("offense", unknownMsgTypeViolation).
		Msg(fmt.Sprintf("potential slashable offense: %s", err))
}

// OnSenderEjectedError logs an error for sender ejected error
func (c *SlashingViolationsConsumer) OnSenderEjectedError(identity *flow.Identity, peerID, msgType string, err error) {
	c.log.Error().
		Str("peer_id", peerID).
		Str("role", identity.Role.String()).
		Hex("sender_id", logging.ID(identity.NodeID)).
		Str("message_type", msgType).
		Str("offense", senderEjectedViolation).
		Msg(fmt.Sprintf("potential slashable offense: %s", err))
}

// OnUnauthorizedUnicastError logs an error for an unauthorized message sent via unicast
func (c *SlashingViolationsConsumer) OnUnauthorizedUnicastError(identity *flow.Identity, peerID, msgType string, err error) {
	c.log.Error().
		Str("peer_id", peerID).
		Str("role", identity.Role.String()).
		Hex("sender_id", logging.ID(identity.NodeID)).
		Str("message_type", msgType).
		Str("offense", unAuthorizedSenderUnicastViolation).
		Msg(fmt.Sprintf("potential slashable offense: %s", err))
}
