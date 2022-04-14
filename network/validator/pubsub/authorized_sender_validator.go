package validator

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/onflow/flow-go/model/flow"
	cborcodec "github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/message"
	"github.com/rs/zerolog"
)

func init() {
	// initialize the authorized roles map the first time this package is imported.
	initializeAuthorizedRolesMap()
}

// authorizedRolesMap is a mapping of message type to a list of roles authorized to send them.
var authorizedRolesMap map[uint8]flow.RoleList

// initializeAuthorizedRolesMap initializes authorizedRolesMap.
func initializeAuthorizedRolesMap() {
	authorizedRolesMap = make(map[uint8]flow.RoleList)

	authorizedRolesMap[cborcodec.CodeBlockProposal] = flow.RoleList{flow.RoleConsensus}
}

// AuthorizedSenderValidator using the getIdentity func will check if the role of the sender
// is part of the authorized roles list for the type of message being sent. A node is considered
// to be authorized to send a message if all of the following are true.
// 1. The message type is a known message type (initialized in the authorizedRolesMap).
// 2. The authorized roles list for the message type contains the senders role.
// 3. The node has a weight > 0 and is not ejected
func AuthorizedSenderValidator(log zerolog.Logger, getIdentity func(peer.ID) (*flow.Identity, bool)) MessageValidator {
	log = log.With().
		Str("component", "authorized_sender_validator").
		Logger()

	return func(ctx context.Context, from peer.ID, msg *message.Message) pubsub.ValidationResult {
		identity, ok := getIdentity(from)
		if !ok {
			log.Warn().Str("peer_id", from.String()).Msg("could not get identity of sender")
			return pubsub.ValidationReject
		}

		msgType := msg.Payload[0]

		if err := isAuthorizedNodeRole(identity.Role, msgType); err != nil {
			log.Warn().
				Err(err).
				Str("peer_id", from.String()).
				Str("role", identity.Role.String()).
				Uint8("message_type", msgType).
				Msg("rejecting message")

			return pubsub.ValidationReject
		}

		if err := isActiveNode(identity); err != nil {
			log.Warn().
				Err(err).
				Str("peer_id", from.String()).
				Str("role", identity.Role.String()).
				Uint8("message_type", msgType).
				Msg("rejecting message")
		}

		return pubsub.ValidationAccept
	}
}

// isAuthorizedNodeRole checks if a role is authorized to send message type
func isAuthorizedNodeRole(role flow.Role, msgType uint8) error {
	roleList, ok := authorizedRolesMap[msgType]
	if !ok {
		return fmt.Errorf("unknown message type does not match any code from the cbor codec")
	}

	if !roleList.Contains(role) {
		return fmt.Errorf("sender is not authorized to send this message type")
	}

	return nil
}

// isActiveNode checks that the node has a weight > 0 and is not ejected
func isActiveNode(identity flow.Identity) error {
	if identity.Weight == 0 {
		return fmt.Errorf("node %s has a weight of 0 is not an active node", identity.NodeID)
	}

	if identity.Ejected {
		return fmt.Errorf("node %s is an ejected node", identity.NodeID)
	}
	
	return nil
}
