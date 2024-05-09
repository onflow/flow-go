package alsp

import "github.com/onflow/flow-go/network"

const (
	// StaleMessage is a misbehavior that is reported when an engine receives a message that is deemed stale based on the
	// local view of the engine. The decision to consider a message stale is up to the engine.
	StaleMessage network.Misbehavior = "misbehavior-stale-message"

	// ResourceIntensiveRequest is a misbehavior that is reported when an engine receives a request that takes an unreasonable amount
	// of resources by the engine to process, e.g., a request for a large number of blocks. The decision to consider a
	// request heavy is up to the engine.
	ResourceIntensiveRequest network.Misbehavior = "misbehavior-resource-intensive-request"

	// RedundantMessage is a misbehavior that is reported when an engine receives a message that is redundant, i.e., the
	// message is already known to the engine. The decision to consider a message redundant is up to the engine.
	RedundantMessage network.Misbehavior = "misbehavior-redundant-message"

	// UnsolicitedMessage is a misbehavior that is reported when an engine receives a message that is not solicited by the
	// engine. The decision to consider a message unsolicited is up to the engine.
	UnsolicitedMessage network.Misbehavior = "misbehavior-unsolicited-message"

	// InvalidMessage is a misbehavior that is reported when an engine receives a message that is invalid, i.e.,
	// the message is not valid according to the engine's validation logic. The decision to consider a message invalid
	// is up to the engine.
	InvalidMessage network.Misbehavior = "misbehavior-invalid-message"

	// UnExpectedValidationError is a misbehavior that is reported when a validation error is encountered during message validation before the message
	// is processed by an engine.
	UnExpectedValidationError network.Misbehavior = "unexpected-validation-error"

	// UnknownMsgType is a misbehavior that is reported when a message of unknown type is received from a peer.
	UnknownMsgType network.Misbehavior = "unknown-message-type"

	// SenderEjected is a misbehavior that is reported when a message is received from an ejected peer.
	SenderEjected network.Misbehavior = "sender-ejected"

	// UnauthorizedUnicastOnChannel is a misbehavior that is reported when a message not authorized to be sent via unicast is received via unicast.
	UnauthorizedUnicastOnChannel network.Misbehavior = "unauthorized-unicast-on-channel"

	// UnAuthorizedSender is a misbehavior that is reported when a message is sent by an unauthorized role.
	UnAuthorizedSender network.Misbehavior = "unauthorized-sender"

	// UnauthorizedPublishOnChannel is a misbehavior that is reported when a message not authorized to be sent via pubsub is received via pubsub.
	UnauthorizedPublishOnChannel network.Misbehavior = "unauthorized-pubsub-on-channel"
)

func AllMisbehaviorTypes() []network.Misbehavior {
	return []network.Misbehavior{
		StaleMessage,
		ResourceIntensiveRequest,
		RedundantMessage,
		UnsolicitedMessage,
		InvalidMessage,
		UnExpectedValidationError,
		UnknownMsgType,
		SenderEjected,
		UnauthorizedUnicastOnChannel,
		UnauthorizedPublishOnChannel,
		UnAuthorizedSender,
	}
}
