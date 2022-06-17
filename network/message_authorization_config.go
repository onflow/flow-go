package network

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/model/messages"
)

// MsgAuthConfig contains authorization information for a specific flow message. The authorization
// is represented as a map from network channel -> list of all roles allowed to send the message on
// the channel.
type MsgAuthConfig struct {
	String    string
	Interface interface{}
	Config    map[Channel]flow.RoleList
}

// IsAuthorized checks if the specified role is authorized to send the message on channel and
// asserts that the message is authorized to be sent on channel.
// Expected error returns during normal operations:
//  * ErrUnauthorizedMessageOnChannel: if channel does not exist in message config
//  * ErrUnauthorizedRole: if list of authorized roles for message config does not include role
func (m MsgAuthConfig) IsAuthorized(role flow.Role, channel Channel) error {
	authorizedRoles, ok := m.Config[channel]
	if !ok {
		return ErrUnauthorizedMessageOnChannel
	}

	if !authorizedRoles.Contains(role) {
		return ErrUnauthorizedRole
	}

	return nil
}

var (
	ErrUnknownMsgType               = errors.New("could not get authorization Config for unknown message type")
	ErrUnauthorizedMessageOnChannel = errors.New("message is not authorized to be sent on channel")
	ErrUnauthorizedRole             = errors.New("sender with role (%s) is not authorized to send message (%s) on channel (%s)")

	// consensus
	blockProposal = MsgAuthConfig{
		String:    "BlockProposal",
		Interface: &messages.BlockProposal{},
		Config: map[Channel]flow.RoleList{
			ConsensusCommittee: {flow.RoleConsensus},
			PushBlocks:         {flow.RoleConsensus}, // channel alias ReceiveBlocks = PushBlocks
		},
	}
	blockVote = MsgAuthConfig{
		String:    "BlockVote",
		Interface: &messages.BlockVote{},
		Config: map[Channel]flow.RoleList{
			ConsensusCommittee: {flow.RoleConsensus},
		},
	}

	// protocol state sync
	syncRequest = MsgAuthConfig{
		String:    "SyncRequest",
		Interface: &messages.SyncRequest{},
		Config: map[Channel]flow.RoleList{
			SyncCommittee:     flow.Roles(),
			SyncClusterPrefix: flow.Roles(),
		},
	}
	syncResponse = MsgAuthConfig{
		String:    "SyncResponse",
		Interface: &messages.SyncResponse{},
		Config: map[Channel]flow.RoleList{
			SyncCommittee:     flow.Roles(),
			SyncClusterPrefix: flow.Roles(),
		},
	}
	rangeRequest = MsgAuthConfig{
		String:    "RangeRequest",
		Interface: &messages.RangeRequest{},
		Config: map[Channel]flow.RoleList{
			SyncCommittee:     flow.Roles(),
			SyncClusterPrefix: flow.Roles(),
		},
	}
	batchRequest = MsgAuthConfig{
		String:    "BatchRequest",
		Interface: &messages.BatchRequest{},
		Config: map[Channel]flow.RoleList{
			SyncCommittee:     flow.Roles(),
			SyncClusterPrefix: flow.Roles(),
		},
	}
	blockResponse = MsgAuthConfig{
		String:    "BlockResponse",
		Interface: &messages.BlockResponse{},
		Config: map[Channel]flow.RoleList{
			SyncCommittee:     flow.Roles(),
			SyncClusterPrefix: flow.Roles(),
		},
	}

	// cluster consensus
	clusterBlockProposal = MsgAuthConfig{
		String:    "ClusterBlockProposal",
		Interface: &messages.ClusterBlockProposal{},
		Config: map[Channel]flow.RoleList{
			ConsensusClusterPrefix: {flow.RoleCollection},
		},
	}
	clusterBlockVote = MsgAuthConfig{
		String:    "ClusterBlockVote",
		Interface: &messages.ClusterBlockVote{},
		Config: map[Channel]flow.RoleList{
			ConsensusClusterPrefix: {flow.RoleCollection},
		},
	}
	clusterBlockResponse = MsgAuthConfig{
		String:    "ClusterBlockResponse",
		Interface: &messages.ClusterBlockResponse{},
		Config: map[Channel]flow.RoleList{
			ConsensusClusterPrefix: {flow.RoleCollection},
		},
	}

	// collections, guarantees & transactions
	collectionGuarantee = MsgAuthConfig{
		String:    "CollectionGuarantee",
		Interface: &flow.CollectionGuarantee{},
		Config: map[Channel]flow.RoleList{
			PushGuarantees: {flow.RoleCollection}, // channel alias ReceiveGuarantees = PushGuarantees
		},
	}
	transaction = MsgAuthConfig{
		String:    "Transaction",
		Interface: &flow.Transaction{},
		Config: map[Channel]flow.RoleList{
			PushTransactions: {flow.RoleCollection}, // channel alias ReceiveTransactions = PushTransactions
		},
	}
	transactionBody = MsgAuthConfig{
		String:    "TransactionBody",
		Interface: &flow.TransactionBody{},
		Config: map[Channel]flow.RoleList{
			PushTransactions: {flow.RoleCollection}, // channel alias ReceiveTransactions = PushTransactions
		},
	}

	// core messages for execution & verification
	executionReceipt = MsgAuthConfig{
		String:    "ExecutionReceipt",
		Interface: &flow.ExecutionReceipt{},
		Config: map[Channel]flow.RoleList{
			PushReceipts: {flow.RoleExecution}, // channel alias ReceiveReceipts = PushReceipts
		},
	}
	resultApproval = MsgAuthConfig{
		String:    "ResultApproval",
		Interface: &flow.ResultApproval{},
		Config: map[Channel]flow.RoleList{
			PushApprovals: {flow.RoleVerification}, // channel alias ReceiveApprovals = PushApprovals
		},
	}

	// [deprecated] execution state synchronization
	executionStateSyncRequest = MsgAuthConfig{
		String: "ExecutionStateSyncRequest",
		Config: nil,
	}
	executionStateDelta = MsgAuthConfig{
		String: "ExecutionStateDelta",
		Config: nil,
	}

	// data exchange for execution of blocks
	chunkDataRequest = MsgAuthConfig{
		String:    "ChunkDataRequest",
		Interface: &messages.ChunkDataRequest{},
		Config: map[Channel]flow.RoleList{
			ProvideChunks:            {flow.RoleVerification}, // channel alias RequestChunks = ProvideChunks
			RequestCollections:       {flow.RoleVerification},
			RequestApprovalsByChunk:  {flow.RoleVerification},
			RequestReceiptsByBlockID: {flow.RoleVerification},
		},
	}
	chunkDataResponse = MsgAuthConfig{
		String:    "ChunkDataResponse",
		Interface: &messages.ChunkDataResponse{},
		Config: map[Channel]flow.RoleList{
			ProvideChunks:            {flow.RoleExecution}, // channel alias RequestChunks = ProvideChunks
			RequestCollections:       {flow.RoleExecution},
			RequestApprovalsByChunk:  {flow.RoleExecution},
			RequestReceiptsByBlockID: {flow.RoleExecution},
		},
	}

	// result approvals
	approvalRequest = MsgAuthConfig{
		String:    "ApprovalRequest",
		Interface: &messages.ApprovalRequest{},
		Config: map[Channel]flow.RoleList{
			ProvideApprovalsByChunk: {flow.RoleConsensus},
		},
	}
	approvalResponse = MsgAuthConfig{
		String:    "ApprovalResponse",
		Interface: &messages.ApprovalResponse{},
		Config: map[Channel]flow.RoleList{
			ProvideApprovalsByChunk: {flow.RoleVerification},
		},
	}

	// generic entity exchange engines
	entityRequest = MsgAuthConfig{
		String:    "EntityRequest",
		Interface: &messages.EntityRequest{},
		Config: map[Channel]flow.RoleList{
			RequestChunks:            {flow.RoleAccess, flow.RoleConsensus, flow.RoleCollection},
			RequestCollections:       {flow.RoleAccess, flow.RoleConsensus, flow.RoleCollection},
			RequestApprovalsByChunk:  {flow.RoleAccess, flow.RoleConsensus, flow.RoleCollection},
			RequestReceiptsByBlockID: {flow.RoleAccess, flow.RoleConsensus, flow.RoleCollection},
		},
	}
	entityResponse = MsgAuthConfig{
		String:    "EntityResponse",
		Interface: &messages.EntityResponse{},
		Config: map[Channel]flow.RoleList{
			RequestChunks:            {flow.RoleCollection, flow.RoleExecution},
			RequestCollections:       {flow.RoleCollection, flow.RoleExecution},
			RequestApprovalsByChunk:  {flow.RoleCollection, flow.RoleExecution},
			RequestReceiptsByBlockID: {flow.RoleCollection, flow.RoleExecution},
		},
	}

	// testing
	echo = MsgAuthConfig{
		String:    "echo",
		Interface: &message.TestMessage{},
		Config: map[Channel]flow.RoleList{
			TestNetworkChannel: flow.Roles(),
			TestMetricsChannel: flow.Roles(),
		},
	}

	// DKG
	dkgMessage = MsgAuthConfig{
		String:    "DKGMessage",
		Interface: &messages.DKGMessage{},
		Config: map[Channel]flow.RoleList{
			DKGCommittee: {flow.RoleConsensus},
		},
	}
)

// GetMessageAuthConfig checks the underlying type and returns the correct
// message auth Config.
// Expected error returns during normal operations:
//  * ErrUnknownMsgType : if underlying type of v does  not match any of the known message types
func GetMessageAuthConfig(v interface{}) (MsgAuthConfig, error) {
	switch v.(type) {
	// consensus
	case *messages.BlockProposal:
		return blockProposal, nil
	case *messages.BlockVote:
		return blockVote, nil

	// protocol state sync
	case *messages.SyncRequest:
		return syncRequest, nil
	case *messages.SyncResponse:
		return syncResponse, nil
	case *messages.RangeRequest:
		return rangeRequest, nil
	case *messages.BatchRequest:
		return batchRequest, nil
	case *messages.BlockResponse:
		return blockResponse, nil

	// cluster consensus
	case *messages.ClusterBlockProposal:
		return clusterBlockProposal, nil
	case *messages.ClusterBlockVote:
		return clusterBlockVote, nil
	case *messages.ClusterBlockResponse:
		return clusterBlockResponse, nil

	// collections, guarantees & transactions
	case *flow.CollectionGuarantee:
		return collectionGuarantee, nil
	case *flow.TransactionBody:
		return transactionBody, nil
	case *flow.Transaction:
		return transaction, nil

	// core messages for execution & verification
	case *flow.ExecutionReceipt:
		return executionReceipt, nil
	case *flow.ResultApproval:
		return resultApproval, nil

	// execution state synchronization
	case *messages.ExecutionStateSyncRequest:
		return executionStateSyncRequest, nil
	case *messages.ExecutionStateDelta:
		return executionStateDelta, nil

	// data exchange for execution of blocks
	case *messages.ChunkDataRequest:
		return chunkDataRequest, nil
	case *messages.ChunkDataResponse:
		return chunkDataResponse, nil

	// result approvals
	case *messages.ApprovalRequest:
		return approvalRequest, nil
	case *messages.ApprovalResponse:
		return approvalResponse, nil

	// generic entity exchange engines
	case *messages.EntityRequest:
		return entityRequest, nil
	case *messages.EntityResponse:
		return entityResponse, nil

	// testing
	case *message.TestMessage:
		return echo, nil

	// dkg
	case *messages.DKGMessage:
		return dkgMessage, nil

	default:
		return MsgAuthConfig{}, fmt.Errorf("%w (%T)", ErrUnknownMsgType, v)
	}
}

// GetAllMessageAuthConfigs returns a list with all message auth configurations
func GetAllMessageAuthConfigs() []MsgAuthConfig {
	return []MsgAuthConfig{
		blockProposal, blockVote, syncRequest, syncResponse, rangeRequest, batchRequest,
		blockResponse, clusterBlockProposal, clusterBlockVote, clusterBlockResponse, collectionGuarantee,
		transaction, transactionBody, executionReceipt, resultApproval,
		chunkDataRequest, chunkDataResponse, approvalRequest, approvalResponse, entityRequest, entityResponse, echo, dkgMessage,
	}
}
