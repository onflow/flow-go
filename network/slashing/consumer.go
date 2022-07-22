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

func (c *Consumer) logOffense(identity *flow.Identity, peerID, msgType, channel, offense string, isUnicast bool, err error) {
	e := c.log.Error().
		Str("peer_id", peerID).
		Str("offense", offense).
		Str("message_type", msgType).
		Str("channel", channel).
		Bool("unicast_message", true)

	if identity != nil {
		e = e.Str("role", identity.Role.String()).Hex("sender_id", logging.ID(identity.NodeID))
	}

	e.Msg(fmt.Sprintf("potential slashable offense: %s", err))
}

// OnUnAuthorizedSenderError logs an error for unauthorized sender error
func (c *Consumer) OnUnAuthorizedSenderError(identity *flow.Identity, peerID, msgType, channel string, isUnicast bool, err error) {
	c.logOffense(identity, peerID, msgType, channel, unAuthorizedSenderViolation, isUnicast, err)
}

// OnUnknownMsgTypeError logs an error for unknown message type error
func (c *Consumer) OnUnknownMsgTypeError(identity *flow.Identity, peerID, msgType, channel string, isUnicast bool, err error) {
	c.logOffense(identity, peerID, msgType, channel, unknownMsgTypeViolation, isUnicast, err)
}

// OnSenderEjectedError logs an error for sender ejected error
func (c *Consumer) OnSenderEjectedError(identity *flow.Identity, peerID, msgType, channel string, isUnicast bool, err error) {
	c.logOffense(identity, peerID, msgType, channel, senderEjectedViolation, isUnicast, err)
}
