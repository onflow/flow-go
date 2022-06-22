package network

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
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
	c.log.Warn().
		Err(err).
		Str("peer_id", peerID).
		Str("role", identity.Role.String()).
		Str("peer_node_id", identity.NodeID.String()).
		Str("message_type", msgType).
		Msg("potential slashable offense")
}

// OnUnknownMsgTypeError logs a warning for unknown message type error
func (c *SlashingViolationsConsumer) OnUnknownMsgTypeError(identity *flow.Identity, peerID, msgType string, err error) {
	c.log.Warn().
		Err(err).
		Str("peer_id", peerID).
		Str("role", identity.Role.String()).
		Str("peer_node_id", identity.NodeID.String()).
		Str("message_type", msgType).
		Msg("potential slashable offense")
}

// OnSenderEjectedError logs a warning for sender ejected error
func (c *SlashingViolationsConsumer) OnSenderEjectedError(identity *flow.Identity, peerID, msgType string, err error) {
	c.log.Warn().
		Err(err).
		Str("peer_id", peerID).
		Str("role", identity.Role.String()).
		Str("peer_node_id", identity.NodeID.String()).
		Str("message_type", msgType).
		Msg("potential slashable offense")
}
