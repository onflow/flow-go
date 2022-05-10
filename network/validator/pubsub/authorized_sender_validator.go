package validator

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"

	channels "github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/network"

	"github.com/onflow/flow-go/model/flow"
	cborcodec "github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/message"
)

// AuthorizedSenderValidator using the getIdentity func will check if the role of the sender
// is part of the authorized roles list for the channel being communicated on. A node is considered
// to be authorized to send a message if all of the following are true.
// 1. The node is authorized.
// 2. The message type is a known message type (can be decoded with cbor codec).
// 3. The authorized roles list for the channel contains the senders role.
// 4. The node is not ejected
func AuthorizedSenderValidator(log zerolog.Logger, channel network.Channel, getIdentity func(peer.ID) (*flow.Identity, bool)) MessageValidator {
	log = log.With().
		Str("component", "authorized_sender_validator").
		Str("network_channel", channel.String()).
		Logger()

	// use cbor codec to add explicit dependency on cbor encoded messages adding the message type
	// to the first byte of the message payload, this adds safety against changing codec without updating this validator
	codec := cborcodec.NewCodec()

	return func(ctx context.Context, from peer.ID, msg *message.Message) pubsub.ValidationResult {
		identity, ok := getIdentity(from)
		if !ok {
			log.Warn().Str("peer_id", from.String()).Msg("could not verify identity of sender")
			return pubsub.ValidationReject
		}

		if identity.Ejected {
			log.Warn().
				Err(fmt.Errorf("node %s is an ejected node", identity.NodeID)).
				Str("peer_id", from.String()).
				Str("role", identity.Role.String()).
				Msg("rejecting message")
			return pubsub.ValidationReject
		}

		// attempt to decode the flow message type from encoded payload
		code, what, err := codec.DecodeMsgType(msg.Payload)
		if err != nil {
			log.Warn().
				Err(err).
				Str("peer_id", from.String()).
				Str("role", identity.Role.String()).
				Msg("rejecting message")
			return pubsub.ValidationReject
		}

		if err := isAuthorizedSender(identity, channel, code); err != nil {
			log.Warn().
				Err(err).
				Str("peer_id", from.String()).
				Str("role", identity.Role.String()).
				Str("message_type", what).
				Str("network_channel", channel.String()).
				Msg("sender is not authorized, rejecting message")

			return pubsub.ValidationReject
		}

		return pubsub.ValidationAccept
	}
}

// isAuthorizedSender checks if node is an authorized role and is not ejected
func isAuthorizedSender(identity *flow.Identity, channel network.Channel, code uint8) error {
	// get authorized roles list
	roles, err := getRoles(channel, code)
	if err != nil {
		return err
	}

	if !roles.Contains(identity.Role) {
		return fmt.Errorf("sender is not authorized to send this message type")
	}

	return nil
}

// getRoles returns list of authorized roles for the channel associated with the message code provided
func getRoles(channel network.Channel, msgTypeCode uint8) (flow.RoleList, error) {
	// echo messages can be sent by anyone
	if msgTypeCode == cborcodec.CodeEcho {
		return flow.Roles(), nil
	}

	// cluster channels have a dynamic channel name
	if msgTypeCode == cborcodec.CodeClusterBlockProposal || msgTypeCode == cborcodec.CodeClusterBlockVote || msgTypeCode == cborcodec.CodeClusterBlockResponse {
		return channels.ClusterChannelRoles(channel), nil
	}

	// get message type codes for all messages communicated on the channel
	codes, ok := cborcodec.ChannelToMsgCodes[channel]
	if !ok {
		return nil, fmt.Errorf("could not get message codes for unknown channel")
	}

	// check if message type code is in list of codes corresponding to channel
	if !containsCode(codes, msgTypeCode) {
		return nil, fmt.Errorf("invalid message type being sent on channel")
	}

	// get authorized list of roles for channel
	roles, ok := channels.RolesByChannel(channel)
	if !ok {
		return nil, fmt.Errorf("could not get roles for channel")
	}

	return roles, nil
}

func containsCode(codes []uint8, code uint8) bool {
	for _, c := range codes {
		if c == code {
			return true
		}
	}

	return false
}
