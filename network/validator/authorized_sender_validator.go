package validator

import (
	"errors"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/slashing"
)

var (
	ErrSenderEjected      = errors.New("validation failed: sender is an ejected node")
	ErrIdentityUnverified = errors.New("validation failed: could not verify identity of sender")
)

type GetIdentityFunc func(peer.ID) (*flow.Identity, bool)

// AuthorizedSenderValidator returns a MessageValidator that will check if the sender of a message is authorized to send the message.
// The MessageValidator returned will use the getIdentity to get the flow identity for the sender, asserting that the sender is a staked node and not ejected. Otherwise, the message is rejected.
// The message is also authorized by checking that the sender is allowed to send the message on the channel.
// If validation fails the message is rejected, and if the validation error is an expected error, slashing data is also collected.
// Authorization config is defined in message.MsgAuthConfig.
func AuthorizedSenderValidator(log zerolog.Logger, slashingViolationsConsumer slashing.ViolationsConsumer, channel channels.Channel, isUnicast bool, getIdentity GetIdentityFunc) MessageValidator {
	return func(from peer.ID, msg interface{}) (string, error) {
		// NOTE: messages from unstaked nodes should be rejected by the libP2P node topic validator
		// before they reach message validators. If a message from a unstaked peer gets to this point
		// something terrible went wrong.
		identity, ok := getIdentity(from)
		if !ok {
			violation := &slashing.Violation{Identity: identity, PeerID: from.String(), Channel: channel, IsUnicast: isUnicast, Err: ErrIdentityUnverified}
			slashingViolationsConsumer.OnUnAuthorizedSenderError(violation)
			return "", ErrIdentityUnverified
		}

		msgType, err := isAuthorizedSender(identity, channel, msg)

		switch {
		case err == nil:
			return msgType, nil
		case message.IsUnknownMsgTypeErr(err):
			violation := &slashing.Violation{Identity: identity, PeerID: from.String(), MsgType: msgType, Channel: channel, IsUnicast: isUnicast, Err: err}
			slashingViolationsConsumer.OnUnknownMsgTypeError(violation)
			return msgType, err
		case errors.Is(err, message.ErrUnauthorizedMessageOnChannel) || errors.Is(err, message.ErrUnauthorizedRole):
			violation := &slashing.Violation{Identity: identity, PeerID: from.String(), MsgType: msgType, Channel: channel, IsUnicast: isUnicast, Err: err}
			slashingViolationsConsumer.OnUnAuthorizedSenderError(violation)
			return msgType, err
		case errors.Is(err, ErrSenderEjected):
			violation := &slashing.Violation{Identity: identity, PeerID: from.String(), MsgType: msgType, Channel: channel, IsUnicast: isUnicast, Err: err}
			slashingViolationsConsumer.OnSenderEjectedError(violation)
			return msgType, ErrSenderEjected
		default:
			// this condition should never happen and indicates there's a bug
			// don't crash as a result of external inputs since that creates a DoS vector
			log.Error().
				Err(err).
				Str("peer_id", from.String()).
				Str("role", identity.Role.String()).
				Str("peer_node_id", identity.NodeID.String()).
				Str("message_type", msgType).
				Bool("unicast_message", isUnicast).
				Msg("unexpected error during message validation")
			return msgType, err
		}
	}
}

// AuthorizedSenderMessageValidator wraps the callback returned by AuthorizedSenderValidator and returns
// MessageValidator callback that returns pubsub.ValidationReject if validation fails and pubsub.ValidationAccept if validation passes.
func AuthorizedSenderMessageValidator(log zerolog.Logger, slashingViolationsConsumer slashing.ViolationsConsumer, channel channels.Channel, getIdentity GetIdentityFunc) PubSubMessageValidator {
	return func(from peer.ID, msg interface{}) pubsub.ValidationResult {
		validate := AuthorizedSenderValidator(log, slashingViolationsConsumer, channel, false, getIdentity)

		_, err := validate(from, msg)
		if err != nil {
			return pubsub.ValidationReject
		}

		return pubsub.ValidationAccept
	}
}

// isAuthorizedSender performs network authorization validation. This func will assert the following;
//  1. The node is not ejected.
//  2. Using the message auth config
//     A. The message is authorized to be sent on channel.
//     B. The sender role is authorized to send message on channel.
//
// Expected error returns during normal operations:
//   - ErrSenderEjected: if identity of sender is ejected from the network
//   - message.ErrUnknownMsgType if message auth config us not found for the msg
//   - message.ErrUnauthorizedMessageOnChannel if msg is not authorized to be sent on channel
//   - message.ErrUnauthorizedRole if sender role is not authorized to send msg
func isAuthorizedSender(identity *flow.Identity, channel channels.Channel, msg interface{}) (string, error) {
	if identity.Ejected {
		return "", ErrSenderEjected
	}

	// get message auth config
	conf, err := message.GetMessageAuthConfig(msg)
	if err != nil {
		return "", err
	}

	// handle special case for cluster prefixed channels
	if prefix, ok := channels.ClusterChannelPrefix(channel); ok {
		channel = channels.Channel(prefix)
	}

	if err := conf.EnsureAuthorized(identity.Role, channel); err != nil {
		return conf.Name, err
	}

	return conf.Name, nil
}
