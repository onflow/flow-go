package validator

import (
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	pubsub "github.com/yhassanzadeh13/go-libp2p-pubsub"

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

// AuthorizedSenderValidator performs message authorization validation.
type AuthorizedSenderValidator struct {
	log                        zerolog.Logger
	slashingViolationsConsumer slashing.ViolationsConsumer
	getIdentity                GetIdentityFunc
}

// NewAuthorizedSenderValidator returns a new AuthorizedSenderValidator
func NewAuthorizedSenderValidator(log zerolog.Logger, slashingViolationsConsumer slashing.ViolationsConsumer, getIdentity GetIdentityFunc) *AuthorizedSenderValidator {
	return &AuthorizedSenderValidator{
		log:                        log.With().Str("component", "authorized_sender_validator").Logger(),
		slashingViolationsConsumer: slashingViolationsConsumer,
		getIdentity:                getIdentity,
	}
}

// Validate will check if the sender of a message is authorized to send the message.
// Using the getIdentity to get the flow identity for the sender, asserting that the sender is a staked node and not ejected.
// Otherwise, the message is rejected. The message is also authorized by checking that the sender is allowed to send the message on the channel.
// If validation fails the message is rejected, and if the validation error is an expected error, slashing data is also collected.
// Authorization config is defined in message.MsgAuthConfig.
func (av *AuthorizedSenderValidator) Validate(from peer.ID, msg interface{}, channel channels.Channel, isUnicast bool) (string, error) {
	// NOTE: Gossipsub messages from unstaked nodes should be rejected by the libP2P node topic validator
	// before they reach message validators. If a message from a unstaked peer gets to this point
	// something terrible went wrong.
	identity, ok := av.getIdentity(from)
	if !ok {
		violation := &slashing.Violation{Identity: identity, PeerID: from.String(), Channel: channel, IsUnicast: isUnicast, Err: ErrIdentityUnverified}
		av.slashingViolationsConsumer.OnUnAuthorizedSenderError(violation)
		return "", ErrIdentityUnverified
	}

	msgType, err := av.isAuthorizedSender(identity, channel, msg)

	switch {
	case err == nil:
		return msgType, nil
	case message.IsUnknownMsgTypeErr(err):
		violation := &slashing.Violation{Identity: identity, PeerID: from.String(), MsgType: msgType, Channel: channel, IsUnicast: isUnicast, Err: err}
		av.slashingViolationsConsumer.OnUnknownMsgTypeError(violation)
		return msgType, err
	case errors.Is(err, message.ErrUnauthorizedMessageOnChannel) || errors.Is(err, message.ErrUnauthorizedRole):
		violation := &slashing.Violation{Identity: identity, PeerID: from.String(), MsgType: msgType, Channel: channel, IsUnicast: isUnicast, Err: err}
		av.slashingViolationsConsumer.OnUnAuthorizedSenderError(violation)
		return msgType, err
	case errors.Is(err, ErrSenderEjected):
		violation := &slashing.Violation{Identity: identity, PeerID: from.String(), MsgType: msgType, Channel: channel, IsUnicast: isUnicast, Err: err}
		av.slashingViolationsConsumer.OnSenderEjectedError(violation)
		return msgType, ErrSenderEjected
	default:
		// this condition should never happen and indicates there's a bug
		// don't crash as a result of external inputs since that creates a DoS vector
		// collect slashing data because this could potentially lead to slashing
		err = fmt.Errorf("unexpected error during message validation: %w", err)
		violation := &slashing.Violation{Identity: identity, PeerID: from.String(), MsgType: msgType, Channel: channel, IsUnicast: isUnicast, Err: err}
		av.slashingViolationsConsumer.OnUnexpectedError(violation)
		return msgType, err
	}
}

// PubSubMessageValidator wraps Validate and returns PubSubMessageValidator callback that returns pubsub.ValidationReject if validation fails and pubsub.ValidationAccept if validation passes.
func (av *AuthorizedSenderValidator) PubSubMessageValidator(channel channels.Channel) PubSubMessageValidator {
	return func(from peer.ID, msg interface{}) pubsub.ValidationResult {
		_, err := av.Validate(from, msg, channel, false)
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
func (av *AuthorizedSenderValidator) isAuthorizedSender(identity *flow.Identity, channel channels.Channel, msg interface{}) (string, error) {
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
