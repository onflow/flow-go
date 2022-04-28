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

func init() {
	// initialize the authorized roles map the first time this package is imported.
	initializeChannelToMsgCodesMap()
}

// channelToMsgCodes is a mapping of network channels to codes of the messages communicated on them. This will be used to check the list of authorized roles associated with the channel
var channelToMsgCodes map[network.Channel][]uint8

// initializeChannelToMsgCodesMap initializes channelToMsgCodes.
func initializeChannelToMsgCodesMap() {
	channelToMsgCodes = make(map[network.Channel][]uint8)

	// consensus
	channelToMsgCodes[channels.ConsensusCommittee] = []uint8{cborcodec.CodeBlockProposal, cborcodec.CodeBlockVote}

	// protocol state sync
	channelToMsgCodes[channels.SyncCommittee] = []uint8{cborcodec.CodeSyncRequest, cborcodec.CodeSyncResponse, cborcodec.CodeRangeRequest, cborcodec.CodeBatchRequest, cborcodec.CodeBlockResponse}

	// collections, guarantees & transactions
	channelToMsgCodes[channels.PushGuarantees] = []uint8{cborcodec.CodeCollectionGuarantee}
	channelToMsgCodes[channels.ReceiveGuarantees] = channelToMsgCodes[channels.PushGuarantees]

	channelToMsgCodes[channels.PushTransactions] = []uint8{cborcodec.CodeTransactionBody, cborcodec.CodeTransaction}
	channelToMsgCodes[channels.ReceiveTransactions] = channelToMsgCodes[channels.PushTransactions]

	// core messages for execution & verification
	channelToMsgCodes[channels.PushReceipts] = []uint8{cborcodec.CodeExecutionReceipt}
	channelToMsgCodes[channels.ReceiveReceipts] = channelToMsgCodes[channels.PushReceipts]

	channelToMsgCodes[channels.PushApprovals] = []uint8{cborcodec.CodeResultApproval}
	channelToMsgCodes[channels.ReceiveApprovals] = channelToMsgCodes[channels.PushApprovals]

	channelToMsgCodes[channels.PushBlocks] = []uint8{cborcodec.CodeBlockProposal}
	channelToMsgCodes[channels.ReceiveBlocks] = channelToMsgCodes[channels.PushBlocks]

	// data exchange for execution of blocks
	channelToMsgCodes[channels.ProvideChunks] = []uint8{cborcodec.CodeChunkDataRequest, cborcodec.CodeChunkDataResponse}

	// result approvals
	channelToMsgCodes[channels.ProvideApprovalsByChunk] = []uint8{cborcodec.CodeApprovalRequest, cborcodec.CodeApprovalResponse}

	// generic entity exchange engines all use EntityRequest and EntityResponse
	channelToMsgCodes[channels.RequestChunks] = []uint8{cborcodec.CodeEntityRequest, cborcodec.CodeEntityResponse, cborcodec.CodeChunkDataRequest, cborcodec.CodeChunkDataResponse}
	channelToMsgCodes[channels.RequestCollections] = channelToMsgCodes[channels.RequestChunks]
	channelToMsgCodes[channels.RequestApprovalsByChunk] = channelToMsgCodes[channels.RequestChunks]
	channelToMsgCodes[channels.RequestReceiptsByBlockID] = channelToMsgCodes[channels.RequestChunks]

	// dkg
	channelToMsgCodes[channels.DKGCommittee] = []uint8{cborcodec.CodeDKGMessage}
}

// AuthorizedSenderValidator using the getIdentity func will check if the role of the sender
// is part of the authorized roles list for the channel being communicated on. A node is considered
// to be authorized to send a message if all of the following are true.
// 1. The node is staked
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
			log.Warn().Str("peer_id", from.String()).Msg("could not get identity of sender")
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
				Msg("rejecting message")

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

	if identity.Ejected {
		return fmt.Errorf("node %s is an ejected node", identity.NodeID)
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
	codes, ok := channelToMsgCodes[channel]
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
