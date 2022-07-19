// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package validator

import (
	"context"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
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

type validateFunc func(ctx context.Context, from peer.ID, msg interface{}) (string, error)

// AuthorizedSenderValidator returns a MessageValidator that will check if the sender of a message is authorized to send the message.
// The MessageValidator returned will use the getIdentity to get the flow identity for the sender, asserting that the sender is a staked node and not ejected. Otherwise, the message is rejected.
// The message is also authorized by checking that the sender is allowed to send the message on the channel.
// If validation fails the message is rejected, and if the validation error is an expected error, slashing data is also collected.
// Authorization config is defined in message.MsgAuthConfig
func AuthorizedSenderValidator(log zerolog.Logger, channel channels.Channel, getIdentity func(peer.ID) (*flow.Identity, bool)) validateFunc {
	log = log.With().
		Str("component", "authorized_sender_validator").
		Str("network_channel", channel.String()).
		Logger()

	slashingViolationsConsumer := slashing.NewSlashingViolationsConsumer(log)

	return func(ctx context.Context, from peer.ID, msg interface{}) (string, error) {
		// NOTE: messages from unstaked nodes should be reject by the libP2P node topic validator
		// before they reach message validators. If a message from a unstaked gets to this point
		// something terrible went wrong.
		identity, ok := getIdentity(from)
		if !ok {
			log.Error().Str("peer_id", from.String()).Msg(fmt.Sprintf("rejecting message: %s", ErrIdentityUnverified))
			return "", ErrIdentityUnverified
		}

		msgType, err := isAuthorizedSender(identity, channel, msg)
		switch {
		case err == nil:
			return msgType, nil
		case message.IsUnknownMsgTypeErr(err):
			slashingViolationsConsumer.OnUnknownMsgTypeError(identity, from.String(), msgType, err)
			return msgType, err
		case errors.Is(err, message.ErrUnauthorizedMessageOnChannel) || errors.Is(err, message.ErrUnauthorizedRole):
			slashingViolationsConsumer.OnUnAuthorizedSenderError(identity, from.String(), msgType, err)
			return msgType, err
		case errors.Is(err, ErrSenderEjected):
			slashingViolationsConsumer.OnSenderEjectedError(identity, from.String(), msgType, err)
			return msgType, ErrSenderEjected
		default:
			log.Error().
				Err(err).
				Str("peer_id", from.String()).
				Str("role", identity.Role.String()).
				Str("peer_node_id", identity.NodeID.String()).
				Str("message_type", msgType).
				Msg("unexpected error during message validation")
			return msgType, err
		}
	}
}

// AuthorizedSenderMessageValidator wraps the callback returned by AuthorizedSenderValidator and returns
// MessageValidator callback that returns pubsub.ValidationReject if validation fails and pubsub.ValidationAccept if validation passes.
func AuthorizedSenderMessageValidator(log zerolog.Logger, channel channels.Channel, getIdentity func(peer.ID) (*flow.Identity, bool)) MessageValidator {
	return func(ctx context.Context, from peer.ID, msg interface{}) pubsub.ValidationResult {
		validate := AuthorizedSenderValidator(log, channel, getIdentity)

		_, err := validate(ctx, from, msg)
		if err != nil {
			return pubsub.ValidationReject
		}

		return pubsub.ValidationAccept
	}
}

// isAuthorizedSender performs network authorization validation. This func will assert the following;
// 1. The node is not ejected.
// 2. Using the message auth config
//  A. The message is authorized to be sent on channel.
//  B. The sender role is authorized to send message on channel.
// Expected error returns during normal operations:
//  * ErrSenderEjected: if identity of sender is ejected from the network
//  * message.ErrUnknownMsgType if message auth config us not found for the msg
//  * message.ErrUnauthorizedMessageOnChannel if msg is not authorized to be sent on channel
//  * message.ErrUnauthorizedRole if sender role is not authorized to send msg
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
