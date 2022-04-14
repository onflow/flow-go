package validator

import (
	"context"

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
var authorizedRolesMap map[cborcodec.MessageType]flow.RoleList

// initializeAuthorizedRolesMap initializes authorizedRolesMap.
func initializeAuthorizedRolesMap() {
	authorizedRolesMap = make(map[cborcodec.MessageType]flow.RoleList)

	authorizedRolesMap[cborcodec.CodeBlockProposal] = flow.RoleList{flow.RoleConsensus}
}

// AuthorizedSenderValidator using the getIdentity func will check if the role of the sender
// is part of the authorized roles list for the type of message being sent.
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

		msgType := cborcodec.MessageType(msg.Payload[0])
		roleList, ok := authorizedRolesMap[msgType]
		if !ok {
			// unknown message type
			log.Warn().
				Str("peer_id", from.String()).
				Str("role", identity.Role.String()).
				Int("message_type", int(msgType)).
				Msg("unknown message type does not match any code from the cbor codec")

			return pubsub.ValidationReject
		}

		if !roleList.Contains(identity.Role) {
			log.Warn().
				Str("peer_id", from.String()).
				Str("role", identity.Role.String()).
				Int("message_type", int(msgType)).
				Msg("sender is not authorized to send this message type")

			return pubsub.ValidationReject
		}

		return pubsub.ValidationAccept
	}
}
