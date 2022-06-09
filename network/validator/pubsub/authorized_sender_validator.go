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
	"github.com/onflow/flow-go/network/message"
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
func AuthorizedSenderValidator(log zerolog.Logger, channel network.Channel, c network.Codec, getIdentity func(peer.ID) (*flow.Identity, bool)) MessageValidator {
	log = log.With().
		Str("component", "authorized_sender_validator").
		Str("network_channel", channel.String()).
		Logger()

	return func(ctx context.Context, from peer.ID, msg *message.Message) pubsub.ValidationResult {
		identity, ok := getIdentity(from)
		if !ok {
			log.Warn().Str("peer_id", from.String()).Msg("could not verify identity of sender")
			return pubsub.ValidationReject
		}

		if identity.Ejected {
			log.Warn().
				Err(fmt.Errorf("sender %s is an ejected node", identity.NodeID)).
				Str("peer_id", from.String()).
				Str("role", identity.Role.String()).
				Str("node_id", identity.NodeID.String()).
				Msg("rejecting message")
			return pubsub.ValidationReject
		}

		// attempt to decode the flow message type from encoded payload
		code, err := c.DecodeMsgType(msg.Payload)
		if err != nil {
			log.Warn().
				Err(err).
				Str("peer_id", from.String()).
				Str("role", identity.Role.String()).
				Str("node_id", identity.NodeID.String()).
				Msg("rejecting message")
			return pubsub.ValidationReject
		}

		if err := isAuthorizedSender(identity, channel, code); err != nil {
			what, _ := code.Code.String()
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

// isAuthorizedSender checks if node is an authorized role.
func isAuthorizedSender(identity *flow.Identity, channel network.Channel, code network.MessageCode) error {
	var authorizedRolesByChannel flow.RoleList

	// handle cluster prefixed channels and check and get authorized roles list
	if prefix, ok := network.ClusterChannelPrefix(channel); ok {
		authorizedRolesByChannel = code.AuthorizedRolesByChannel(network.Channel(prefix))
	} else {
		authorizedRolesByChannel = code.AuthorizedRolesByChannel(channel)
	}

	// check if role is authorized to send message on channel
	if !authorizedRolesByChannel.Contains(identity.Role) {
		return fmt.Errorf("sender role is not authorized to send this message type on channel")
	}

	return nil
}
