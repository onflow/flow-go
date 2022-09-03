package slashing

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/utils/logging"
)

const (
	unAuthorizedSenderViolation = "unauthorized_sender"
	unknownMsgTypeViolation     = "unknown_message_type"
	invalidMsgViolation         = "invalid_message"
	senderEjectedViolation      = "sender_ejected"
)

// Consumer is a struct that logs a message for any slashable offences.
// This struct will be updated in the future when slashing is implemented.
type Consumer struct {
	log zerolog.Logger
}

// NewSlashingViolationsConsumer returns a new Consumer
func NewSlashingViolationsConsumer(log zerolog.Logger) *Consumer {
	return &Consumer{log.With().Str("module", "network_slashing_consumer").Logger()}
}

func (c *Consumer) logOffense(networkOffense string, violation *Violation) {
	e := c.log.Error().
		Str("peer_id", violation.PeerID).
		Str("networking_offense", networkOffense).
		Str("message_type", violation.MsgType).
		Str("channel", violation.Channel.String()).
		Bool("unicast_message", violation.IsUnicast)

	if violation.Identity != nil {
		e = e.Str("role", violation.Identity.Role.String()).Hex("sender_id", logging.ID(violation.Identity.NodeID))
	}

	e.Msg(fmt.Sprintf("potential slashable offense: %s", violation.Err))
}

// OnUnAuthorizedSenderError logs an error for unauthorized sender error
func (c *Consumer) OnUnAuthorizedSenderError(violation *Violation) {
	c.logOffense(unAuthorizedSenderViolation, violation)
}

// OnUnknownMsgTypeError logs an error for unknown message type error
func (c *Consumer) OnUnknownMsgTypeError(violation *Violation) {
	c.logOffense(unknownMsgTypeViolation, violation)
}

// OnInvalidMsgError logs an error for messages that contained payloads that could not
// be unmarshalled into the message type denoted by message code byte.
func (c *Consumer) OnInvalidMsgError(violation *Violation) {
	c.logOffense(invalidMsgViolation, violation)
}

// OnSenderEjectedError logs an error for sender ejected error
func (c *Consumer) OnSenderEjectedError(violation *Violation) {
	c.logOffense(senderEjectedViolation, violation)
}
