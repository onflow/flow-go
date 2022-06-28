package slashing

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/utils/logging"

	"github.com/onflow/flow-go/model/flow"
)

const (
	unAuthorizedSenderViolation = "unauthorized_sender"
	unknownMsgTypeViolation     = "unknown_message_type"
	senderEjectedViolation      = "sender_ejected"
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

// OnUnAuthorizedSenderError logs a warning for unauthorized sender error
func (c *SlashingViolationsConsumer) OnUnAuthorizedSenderError(identity *flow.Identity, peerID, msgType string, err error) {
	c.log.Error().
		Err(err).
		Str("peer_id", peerID).
		Str("role", identity.Role.String()).
		Hex("sender_id", logging.ID(identity.NodeID)).
		Str("message_type", msgType).
		Str("offense", unAuthorizedSenderViolation).
		Msg("potential slashable offense")
}

// OnUnknownMsgTypeError logs a warning for unknown message type error
func (c *SlashingViolationsConsumer) OnUnknownMsgTypeError(identity *flow.Identity, peerID, msgType string, err error) {
	c.log.Error().
		Err(err).
		Str("peer_id", peerID).
		Str("role", identity.Role.String()).
		Hex("sender_id", logging.ID(identity.NodeID)).
		Str("message_type", msgType).
		Str("offense", unknownMsgTypeViolation).
		Msg("potential slashable offense")
}

// OnSenderEjectedError logs a warning for sender ejected error
func (c *SlashingViolationsConsumer) OnSenderEjectedError(identity *flow.Identity, peerID, msgType string, err error) {
	c.log.Error().
		Err(err).
		Str("peer_id", peerID).
		Str("role", identity.Role.String()).
		Hex("sender_id", logging.ID(identity.NodeID)).
		Str("message_type", msgType).
		Str("offense", senderEjectedViolation).
		Msg("potential slashable offense")
}
