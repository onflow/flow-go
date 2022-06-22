// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package validator

import (
	"context"
	"errors"
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network"

	"github.com/onflow/flow-go/model/flow"
)

var (
	ErrUnauthorizedSender = errors.New("validation failed: sender is not authorized to send this message type")
	ErrSenderEjected      = errors.New("validation failed: sender is an ejected node")
	ErrUnknownMessageType = errors.New("validation failed: failed to get message auth config")
	ErrIdentityUnverified = errors.New("validation failed: could not verify identity of sender")
)

// AuthorizedSenderValidator returns a MessageValidator that will check if the sender of a message is authorized to send the message.
// The MessageValidator returned will use the getIdentity to get the flow identity for the sender, asserting that the sender is a staked node.
// If the sender is an unstaked node the message is rejected. IsAuthorizedSender is used to perform further message validation. If validation
// fails the message is rejected, if the validation error is an expected error slashing data is collected before the message is rejected.
func AuthorizedSenderValidator(log zerolog.Logger, channel network.Channel, getIdentity func(peer.ID) (*flow.Identity, bool)) MessageValidator {
	slashingViolationsConsumer := network.NewSlashingViolationsConsumer(log.With().
		Str("component", "authorized_sender_validator").
		Str("network_channel", channel.String()).
		Logger())

	return func(ctx context.Context, from peer.ID, msg interface{}) pubsub.ValidationResult {
		identity, ok := getIdentity(from)
		if !ok {
			log.Warn().Err(ErrIdentityUnverified).Str("peer_id", from.String()).Msg("rejecting message")
			return pubsub.ValidationReject
		}

		msgType, err := IsAuthorizedSender(identity, channel, msg)
		if errors.Is(err, ErrUnauthorizedSender) {
			slashingViolationsConsumer.OnUnAuthorizedSenderError(identity, from.String(), msgType, err)
			return pubsub.ValidationReject
		} else if errors.Is(err, ErrUnknownMessageType) {
			slashingViolationsConsumer.OnUnknownMsgTypeError(identity, from.String(), msgType, err)
			return pubsub.ValidationReject
		} else if errors.Is(err, ErrSenderEjected) {
			slashingViolationsConsumer.OnSenderEjectedError(identity, from.String(), msgType, err)
			return pubsub.ValidationReject
		} else if err != nil {
			log.Warn().
				Err(err).
				Str("peer_id", from.String()).
				Str("role", identity.Role.String()).
				Str("peer_node_id", identity.NodeID.String()).
				Str("message_type", msgType).
				Msg("unexpected error during message validation")
			return pubsub.ValidationReject
		}

		return pubsub.ValidationAccept
	}
}

// IsAuthorizedSender performs network authorization validation. This func will assert the following;
// 1. The node is not ejected.
// 2. Using the message auth config
//  A. The message is authorized to be sent on channel.
//  B. The sender role is authorized to send message channel.
//  C. The sender role is authorized to participate on channel.
// Expected error returns during normal operations:
//  * ErrSenderEjected: if identity of sender is ejected
//  * ErrUnknownMessageType: if retrieving the message auth config for msg fails
//  * ErrUnauthorizedSender: if the message auth config validation for msg fails
func IsAuthorizedSender(identity *flow.Identity, channel network.Channel, msg interface{}) (string, error) {
	if identity.Ejected {
		return "", ErrSenderEjected
	}

	// get message auth config
	conf, err := network.GetMessageAuthConfig(msg)
	if err != nil {
		return "", fmt.Errorf("%s: %w", err, ErrUnknownMessageType)
	}

	// handle special case for cluster prefixed channels
	if prefix, ok := network.ClusterChannelPrefix(channel); ok {
		channel = network.Channel(prefix)
	}

	if err := conf.IsAuthorized(identity.Role, channel); err != nil {
		return conf.String, fmt.Errorf("%s: %w", err, ErrUnauthorizedSender)
	}

	return conf.String, nil
}
