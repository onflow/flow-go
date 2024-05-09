package slashing

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/alsp"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	unknown = "unknown"
)

// Consumer is a struct that logs a message for any slashable offenses.
// This struct will be updated in the future when slashing is implemented.
type Consumer struct {
	log                       zerolog.Logger
	metrics                   module.NetworkSecurityMetrics
	misbehaviorReportConsumer network.MisbehaviorReportConsumer
}

// NewSlashingViolationsConsumer returns a new Consumer.
func NewSlashingViolationsConsumer(log zerolog.Logger, metrics module.NetworkSecurityMetrics, misbehaviorReportConsumer network.MisbehaviorReportConsumer) *Consumer {
	return &Consumer{
		log:                       log.With().Str("module", "network_slashing_consumer").Logger(),
		metrics:                   metrics,
		misbehaviorReportConsumer: misbehaviorReportConsumer,
	}
}

// logOffense logs the slashing violation with details.
func (c *Consumer) logOffense(misbehavior network.Misbehavior, violation *network.Violation) {
	// if violation fails before the message is decoded the violation.MsgType will be unknown
	if len(violation.MsgType) == 0 {
		violation.MsgType = unknown
	}

	// if violation fails for an unknown peer violation.Identity will be nil
	role := unknown
	nodeID := flow.ZeroID
	if violation.Identity != nil {
		role = violation.Identity.Role.String()
		nodeID = violation.Identity.NodeID
	}

	e := c.log.Error().
		Str("peer_id", violation.PeerID).
		Str("misbehavior", misbehavior.String()).
		Str("message_type", violation.MsgType).
		Str("channel", violation.Channel.String()).
		Str("protocol", violation.Protocol.String()).
		Bool(logging.KeySuspicious, true).
		Str("role", role).
		Hex("sender_id", logging.ID(nodeID))

	e.Msg(fmt.Sprintf("potential slashable offense: %s", violation.Err))

	// capture unauthorized message count metric
	c.metrics.OnUnauthorizedMessage(role, violation.MsgType, violation.Channel.String(), misbehavior.String())
}

// reportMisbehavior reports the slashing violation to the alsp misbehavior report manager. When violation identity
// is nil this indicates the misbehavior occurred either on a public network and the identity of the sender is unknown
// we can skip reporting the misbehavior.
// Args:
// - misbehavior: the network misbehavior.
// - violation: the slashing violation.
// Any error encountered while creating the misbehavior report is considered irrecoverable and will result in a fatal log.
func (c *Consumer) reportMisbehavior(misbehavior network.Misbehavior, violation *network.Violation) {
	if violation.Identity == nil {
		c.log.Debug().
			Bool(logging.KeySuspicious, true).
			Str("peerID", violation.PeerID).
			Msg("violation identity unknown (or public) skipping misbehavior reporting")
		c.metrics.OnViolationReportSkipped()
		return
	}
	report, err := alsp.NewMisbehaviorReport(violation.Identity.NodeID, misbehavior)
	if err != nil {
		// failing to create the misbehavior report is unlikely. If an error is encountered while
		// creating the misbehavior report it indicates a bug and processing can not proceed.
		c.log.Fatal().
			Err(err).
			Str("peerID", violation.PeerID).
			Msg("failed to create misbehavior report")
	}
	c.misbehaviorReportConsumer.ReportMisbehaviorOnChannel(violation.Channel, report)
}

// OnUnAuthorizedSenderError logs an error for unauthorized sender error and reports a misbehavior to alsp misbehavior report manager.
func (c *Consumer) OnUnAuthorizedSenderError(violation *network.Violation) {
	c.logOffense(alsp.UnAuthorizedSender, violation)
	c.reportMisbehavior(alsp.UnAuthorizedSender, violation)
}

// OnUnknownMsgTypeError logs an error for unknown message type error and reports a misbehavior to alsp misbehavior report manager.
func (c *Consumer) OnUnknownMsgTypeError(violation *network.Violation) {
	c.logOffense(alsp.UnknownMsgType, violation)
	c.reportMisbehavior(alsp.UnknownMsgType, violation)
}

// OnInvalidMsgError logs an error for messages that contained payloads that could not
// be unmarshalled into the message type denoted by message code byte and reports a misbehavior to alsp misbehavior report manager.
func (c *Consumer) OnInvalidMsgError(violation *network.Violation) {
	c.logOffense(alsp.InvalidMessage, violation)
	c.reportMisbehavior(alsp.InvalidMessage, violation)
}

// OnSenderEjectedError logs an error for sender ejected error and reports a misbehavior to alsp misbehavior report manager.
func (c *Consumer) OnSenderEjectedError(violation *network.Violation) {
	c.logOffense(alsp.SenderEjected, violation)
	c.reportMisbehavior(alsp.SenderEjected, violation)
}

// OnUnauthorizedUnicastOnChannel logs an error for messages unauthorized to be sent via unicast and reports a misbehavior to alsp misbehavior report manager.
func (c *Consumer) OnUnauthorizedUnicastOnChannel(violation *network.Violation) {
	c.logOffense(alsp.UnauthorizedUnicastOnChannel, violation)
	c.reportMisbehavior(alsp.UnauthorizedUnicastOnChannel, violation)
}

// OnUnauthorizedPublishOnChannel logs an error for messages unauthorized to be sent via pubsub.
func (c *Consumer) OnUnauthorizedPublishOnChannel(violation *network.Violation) {
	c.logOffense(alsp.UnauthorizedPublishOnChannel, violation)
	c.reportMisbehavior(alsp.UnauthorizedPublishOnChannel, violation)
}

// OnUnexpectedError logs an error for unexpected errors. This indicates message validation
// has failed for an unknown reason and could potentially be n slashable offense and reports a misbehavior to alsp misbehavior report manager.
func (c *Consumer) OnUnexpectedError(violation *network.Violation) {
	c.logOffense(alsp.UnExpectedValidationError, violation)
	c.reportMisbehavior(alsp.UnExpectedValidationError, violation)
}
