package validator

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	cborcodec "github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/message"
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

	// consensus
	authorizedRolesMap[cborcodec.CodeBlockProposal] = flow.RoleList{flow.RoleConsensus}
	authorizedRolesMap[cborcodec.CodeBlockVote] = flow.RoleList{flow.RoleConsensus}

	// protocol state sync
	authorizedRolesMap[cborcodec.CodeSyncRequest] = flow.Roles()
	authorizedRolesMap[cborcodec.CodeSyncResponse] = flow.Roles()
	authorizedRolesMap[cborcodec.CodeRangeRequest] = flow.Roles()
	authorizedRolesMap[cborcodec.CodeBatchRequest] = flow.Roles()
	authorizedRolesMap[cborcodec.CodeBlockResponse] = flow.Roles()

	// cluster consensus
	authorizedRolesMap[cborcodec.CodeClusterBlockProposal] = flow.RoleList{flow.RoleCollection}
	authorizedRolesMap[cborcodec.CodeClusterBlockVote] = flow.RoleList{flow.RoleCollection}
	authorizedRolesMap[cborcodec.CodeClusterBlockResponse] = flow.RoleList{flow.RoleCollection}

	// collections, guarantees & transactions
	authorizedRolesMap[cborcodec.CodeCollectionGuarantee] = flow.RoleList{flow.RoleCollection}
	authorizedRolesMap[cborcodec.CodeTransactionBody] = flow.RoleList{flow.RoleCollection}
	authorizedRolesMap[cborcodec.CodeTransaction] = flow.RoleList{flow.RoleCollection}

	// core messages for execution & verification
	authorizedRolesMap[cborcodec.CodeExecutionReceipt] = flow.RoleList{flow.RoleExecution}
	authorizedRolesMap[cborcodec.CodeResultApproval] = flow.RoleList{flow.RoleVerification}

	// execution state synchronization
	// NOTE: these messages have been deprecated
	authorizedRolesMap[cborcodec.CodeExecutionStateSyncRequest] = flow.RoleList{}
	authorizedRolesMap[cborcodec.CodeExecutionStateDelta] = flow.RoleList{}

	// data exchange for execution of blocks
	authorizedRolesMap[cborcodec.CodeChunkDataRequest] = flow.RoleList{flow.RoleVerification}
	authorizedRolesMap[cborcodec.CodeChunkDataResponse] = flow.RoleList{flow.RoleExecution}

	// result approvals
	authorizedRolesMap[cborcodec.CodeApprovalRequest] = flow.RoleList{flow.RoleConsensus}
	authorizedRolesMap[cborcodec.CodeApprovalResponse] = flow.RoleList{flow.RoleVerification}

	// generic entity exchange engines
	authorizedRolesMap[cborcodec.CodeEntityRequest] = flow.RoleList{flow.RoleAccess, flow.RoleConsensus, flow.RoleCollection} // only staked access nodes
	authorizedRolesMap[cborcodec.CodeEntityResponse] = flow.RoleList{flow.RoleCollection, flow.RoleExecution}

	// testing
	authorizedRolesMap[cborcodec.CodeEcho] = flow.Roles()

	// dkg
	authorizedRolesMap[cborcodec.CodeDKGMessage] = flow.RoleList{flow.RoleConsensus} // sn nodes for next epoch
}

// AuthorizedSenderValidator using the getIdentity func will check if the role of the sender
// is part of the authorized roles list for the type of message being sent. A node is considered
// to be authorized to send a message if all of the following are true.
// 1. The message type is a known message type (initialized in the authorizedRolesMap).
// 2. The authorized roles list for the message type contains the senders role.
// 3. The node is not ejected
func AuthorizedSenderValidator(log zerolog.Logger, getIdentity func(peer.ID) (*flow.Identity, bool)) MessageValidator {
	log = log.With().
		Str("component", "authorized_sender_validator").
		Logger()

	// use cbor codec to add explicit dependency on cbor encoded messages adding the message type
	// to the first byte of the message payload.
	codec := cborcodec.NewCodec()

	return func(ctx context.Context, from peer.ID, msg *message.Message) pubsub.ValidationResult {
		identity, ok := getIdentity(from)
		if !ok {
			log.Warn().Str("peer_id", from.String()).Msg("could not get identity of sender")
			return pubsub.ValidationReject
		}

		// attempt to decode the flow message type from encoded payload
		msgTypeCode, what, err := codec.DecodeMsgType(msg.Payload)
		if err != nil {
			log.Warn().
				Err(err).
				Str("peer_id", from.String()).
				Str("role", identity.Role.String()).
				Msg("rejecting message")
			return pubsub.ValidationReject
		}

		if err := isAuthorizedSender(identity, msgTypeCode); err != nil {
			log.Warn().
				Err(err).
				Str("peer_id", from.String()).
				Str("role", identity.Role.String()).
				Str("message_type", what).
				Msg("rejecting message")

			return pubsub.ValidationReject
		}

		return pubsub.ValidationAccept
	}
}

// isAuthorizedSender checks if node is an authorized role and is not ejected
func isAuthorizedSender(identity *flow.Identity, msgTypeCode uint8) error {
	roleList, ok := authorizedRolesMap[msgTypeCode]
	if !ok {
		return fmt.Errorf("unknown message type does not match any code from the cbor codec")
	}

	if !roleList.Contains(identity.Role) {
		return fmt.Errorf("sender is not authorized to send this message type")
	}

	if identity.Ejected {
		return fmt.Errorf("node %s is an ejected node", identity.NodeID)
	}

	return nil
}
