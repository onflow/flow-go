package network

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/model/messages"
)

// MsgAuthConfig contains authorization information for a specific flow message. The authorization
// is represented as a map from network channel -> list of all roles allowed to send the message on
// the channel.
type MsgAuthConfig struct {
	String string
	config map[Channel]flow.RoleList
}

// IsAuthorized checks if the specified role is authorized to send the message on channel and
// asserts that the message is authorized to be sent on channel.
func (m MsgAuthConfig) IsAuthorized(role flow.Role, channel Channel) error {
	authorizedRoles, ok := m.config[channel]
	if !ok {
		return fmt.Errorf("message (%s) is not authorized to be sent on channel (%s)", m.String, channel)
	}

	if !authorizedRoles.Contains(role) {
		return fmt.Errorf("sender with role (%s) is not authorized to send message (%s) on channel (%s)", role, m.String, channel)
	}

	return nil
}

var (
	// consensus
	blockProposal = MsgAuthConfig{
		String: "BlockProposal",
		config: map[Channel]flow.RoleList{
			ConsensusCommittee: {flow.RoleConsensus},
			PushBlocks:         {flow.RoleConsensus}, // channel alias ReceiveBlocks = PushBlocks
		},
	}
	blockVote = MsgAuthConfig{
		String: "BlockVote",
		config: map[Channel]flow.RoleList{
			ConsensusCommittee: {flow.RoleConsensus},
		},
	}

	// protocol state sync
	syncRequest = MsgAuthConfig{
		String: "SyncRequest",
		config: map[Channel]flow.RoleList{
			SyncCommittee:     flow.Roles(),
			SyncClusterPrefix: flow.Roles(),
		},
	}
	syncResponse = MsgAuthConfig{
		String: "SyncResponse",
		config: map[Channel]flow.RoleList{
			SyncCommittee:     flow.Roles(),
			SyncClusterPrefix: flow.Roles(),
		},
	}
	rangeRequest = MsgAuthConfig{
		String: "RangeRequest",
		config: map[Channel]flow.RoleList{
			SyncCommittee:     flow.Roles(),
			SyncClusterPrefix: flow.Roles(),
		},
	}
	batchRequest = MsgAuthConfig{
		String: "BatchRequest",
		config: map[Channel]flow.RoleList{
			SyncCommittee:     flow.Roles(),
			SyncClusterPrefix: flow.Roles(),
		},
	}
	blockResponse = MsgAuthConfig{
		String: "BlockResponse",
		config: map[Channel]flow.RoleList{
			SyncCommittee:     flow.Roles(),
			SyncClusterPrefix: flow.Roles(),
		},
	}

	// cluster consensus
	clusterBlockProposal = MsgAuthConfig{
		String: "ClusterBlockProposal",
		config: map[Channel]flow.RoleList{
			ConsensusClusterPrefix: {flow.RoleCollection},
		},
	}
	clusterBlockVote = MsgAuthConfig{
		String: "ClusterBlockVote",
		config: map[Channel]flow.RoleList{
			ConsensusClusterPrefix: {flow.RoleCollection},
		},
	}
	clusterBlockResponse = MsgAuthConfig{
		String: "ClusterBlockResponse",
		config: map[Channel]flow.RoleList{
			ConsensusClusterPrefix: {flow.RoleCollection},
		},
	}

	// collections, guarantees & transactions
	collectionGuarantee = MsgAuthConfig{
		String: "CollectionGuarantee",
		config: map[Channel]flow.RoleList{
			PushGuarantees: {flow.RoleCollection}, // channel alias ReceiveGuarantees = PushGuarantees
		},
	}
	transaction = MsgAuthConfig{
		String: "Transaction",
		config: map[Channel]flow.RoleList{
			PushTransactions: {flow.RoleCollection}, // channel alias ReceiveTransactions = PushTransactions
		},
	}
	transactionBody = MsgAuthConfig{
		String: "TransactionBody",
		config: map[Channel]flow.RoleList{
			PushTransactions: {flow.RoleCollection}, // channel alias ReceiveTransactions = PushTransactions
		},
	}

	// core messages for execution & verification
	executionReceipt = MsgAuthConfig{
		String: "ExecutionReceipt",
		config: map[Channel]flow.RoleList{
			PushReceipts: {flow.RoleExecution}, // channel alias ReceiveReceipts = PushReceipts
		},
	}
	resultApproval = MsgAuthConfig{
		String: "ResultApproval",
		config: map[Channel]flow.RoleList{
			PushApprovals: {flow.RoleVerification}, // channel alias ReceiveApprovals = PushApprovals
		},
	}

	// [deprecated] execution state synchronization
	executionStateSyncRequest = MsgAuthConfig{
		String: "ExecutionStateSyncRequest",
		config: nil,
	}
	executionStateDelta = MsgAuthConfig{
		String: "ExecutionStateDelta",
		config: nil,
	}

	// data exchange for execution of blocks
	chunkDataRequest = MsgAuthConfig{
		String: "ChunkDataRequest",
		config: map[Channel]flow.RoleList{
			ProvideChunks:            {flow.RoleVerification}, // channel alias RequestChunks = ProvideChunks
			RequestCollections:       {flow.RoleVerification},
			RequestApprovalsByChunk:  {flow.RoleVerification},
			RequestReceiptsByBlockID: {flow.RoleVerification},
		},
	}
	chunkDataResponse = MsgAuthConfig{
		String: "ChunkDataResponse",
		config: map[Channel]flow.RoleList{
			ProvideChunks:            {flow.RoleExecution}, // channel alias RequestChunks = ProvideChunks
			RequestCollections:       {flow.RoleExecution},
			RequestApprovalsByChunk:  {flow.RoleExecution},
			RequestReceiptsByBlockID: {flow.RoleExecution},
		},
	}

	// result approvals
	approvalRequest = MsgAuthConfig{
		String: "ApprovalRequest",
		config: map[Channel]flow.RoleList{
			ProvideApprovalsByChunk: {flow.RoleConsensus},
		},
	}
	approvalResponse = MsgAuthConfig{
		String: "ApprovalResponse",
		config: map[Channel]flow.RoleList{
			ProvideApprovalsByChunk: {flow.RoleVerification},
		},
	}

	// generic entity exchange engines
	entityRequest = MsgAuthConfig{
		String: "EntityRequest",
		config: map[Channel]flow.RoleList{
			RequestChunks:            {flow.RoleAccess, flow.RoleConsensus, flow.RoleCollection},
			RequestCollections:       {flow.RoleAccess, flow.RoleConsensus, flow.RoleCollection},
			RequestApprovalsByChunk:  {flow.RoleAccess, flow.RoleConsensus, flow.RoleCollection},
			RequestReceiptsByBlockID: {flow.RoleAccess, flow.RoleConsensus, flow.RoleCollection},
		},
	}
	entityResponse = MsgAuthConfig{
		String: "EntityResponse",
		config: map[Channel]flow.RoleList{
			RequestChunks:            {flow.RoleCollection, flow.RoleExecution},
			RequestCollections:       {flow.RoleCollection, flow.RoleExecution},
			RequestApprovalsByChunk:  {flow.RoleCollection, flow.RoleExecution},
			RequestReceiptsByBlockID: {flow.RoleCollection, flow.RoleExecution},
		},
	}

	// testing
	echo = MsgAuthConfig{
		String: "echo",
		config: map[Channel]flow.RoleList{
			TestNetworkChannel: flow.Roles(),
			TestMetricsChannel: flow.Roles(),
		},
	}

	// DKG
	dkgMessage = MsgAuthConfig{
		String: "DKGMessage",
		config: map[Channel]flow.RoleList{
			DKGCommittee: {flow.RoleConsensus},
		},
	}
)

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
		return MsgAuthConfig{}, fmt.Errorf("could not get authorization config for message with type (%T)", v)
	}
}
