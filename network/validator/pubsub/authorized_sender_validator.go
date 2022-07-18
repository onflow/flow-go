// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package validator

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network"

	"github.com/onflow/flow-go/model/flow"
)

// AuthorizedSenderValidator using the getIdentity func will check if the role of the sender
// is part of the authorized roles list for the channel being communicated on. A node is considered
// to be authorized to send a message if the following are true.
// 1. The node is staked.
// 2. The node is not ejected.
// 3. The message type is a known message type (can be decoded with network codec).
// 4. The message is authorized to be sent on channel.
// 4. The sender role is authorized to send message channel.
// 5. The sender role is authorized to participate on channel.
func AuthorizedSenderValidator(log zerolog.Logger, channel network.Channel, getIdentity func(peer.ID) (*flow.Identity, bool)) MessageValidator {
	log = log.With().
		Str("component", "authorized_sender_validator").
		Str("network_channel", channel.String()).
		Logger()

	return func(ctx context.Context, from peer.ID, msg interface{}) pubsub.ValidationResult {
		identity, ok := getIdentity(from)
		if !ok {
			log.Warn().Str("peer_id", from.String()).Msg("could not verify identity of sender")
			return pubsub.ValidationReject
		}

		// redundant check if the node is ejected so that we can fail fast before decoding
		if identity.Ejected {
			log.Warn().
				Err(fmt.Errorf("sender %s is an ejected node", identity.NodeID)).
				Str("peer_id", from.String()).
				Str("role", identity.Role.String()).
				Str("node_id", identity.NodeID.String()).
				Msg("rejecting message")
			return pubsub.ValidationReject
		}

		if what, err := IsAuthorizedSender(identity, channel, msg); err != nil {
			log.Warn().
				Err(err).
				Str("peer_id", from.String()).
				Str("role", identity.Role.String()).
				Str("node_id", identity.NodeID.String()).
				Str("message_type", what).
				Msg("sender is not authorized, rejecting message")

			return pubsub.ValidationReject
		}

		return pubsub.ValidationAccept
	}
}

// IsAuthorizedSender checks if node is an authorized role.
func IsAuthorizedSender(identity *flow.Identity, channel network.Channel, msg interface{}) (string, error) {
	if identity.Ejected {
		return "", fmt.Errorf("sender %s is an ejected node", identity.NodeID)
	}

	// get message code configuration
	conf, err := network.GetMessageAuthConfig(msg)
	if err != nil {
		return "", fmt.Errorf("failed to get message auth config: %w", err)
	}

	// handle special case for cluster prefixed channels
	if prefix, ok := network.ClusterChannelPrefix(channel); ok {
		channel = network.Channel(prefix)
	}

	if err := conf.IsAuthorized(identity.Role, channel); err != nil {
		return conf.String, err
	}

	return conf.String, nil
}
