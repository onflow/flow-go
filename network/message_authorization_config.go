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
	Interface func() interface{}
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
	ErrUnauthorizedRole             = errors.New("sender role not authorized to send message on channel")
	MessageAuthConfigs              map[string]MsgAuthConfig
)

// init is called first time this package is imported.
// It creates and initializes channelRoleMap and clusterChannelPrefixRoleMap.
func init() {
	initializeMessageAuthConfigsMap()
}

func initializeMessageAuthConfigsMap() {
	MessageAuthConfigs = make(map[string]MsgAuthConfig)
	
	// consensus
	MessageAuthConfigs["BlockProposal"] = MsgAuthConfig{
		String: "BlockProposal",
		Interface: func() interface{} {
			return new(messages.BlockProposal)
		},
		Config: map[Channel]flow.RoleList{
			ConsensusCommittee: {flow.RoleConsensus},
			PushBlocks:         {flow.RoleConsensus}, // channel alias ReceiveBlocks = PushBlocks
		},
	}
	MessageAuthConfigs["BlockVote"] = MsgAuthConfig{
		String: "BlockVote",
		Interface: func() interface{} {
			return new(messages.BlockVote)
		},
		Config: map[Channel]flow.RoleList{
			ConsensusCommittee: {flow.RoleConsensus},
		},
	}

	// protocol state sync
	MessageAuthConfigs["SyncRequest"] = MsgAuthConfig{
		String: "SyncRequest",
		Interface: func() interface{} {
			return new(messages.SyncRequest)
		},
		Config: map[Channel]flow.RoleList{
			SyncCommittee:     flow.Roles(),
			SyncClusterPrefix: flow.Roles(),
		},
	}
	MessageAuthConfigs["SyncResponse"] = MsgAuthConfig{
		String: "SyncResponse",
		Interface: func() interface{} {
			return new(messages.SyncResponse)
		},
		Config: map[Channel]flow.RoleList{
			SyncCommittee:     flow.Roles(),
			SyncClusterPrefix: flow.Roles(),
		},
	}
	MessageAuthConfigs["RangeRequest"] = MsgAuthConfig{
		String: "RangeRequest",
		Interface: func() interface{} {
			return new(messages.RangeRequest)
		},
		Config: map[Channel]flow.RoleList{
			SyncCommittee:     flow.Roles(),
			SyncClusterPrefix: flow.Roles(),
		},
	}
	MessageAuthConfigs["BatchRequest"] = MsgAuthConfig{
		String: "BatchRequest",
		Interface: func() interface{} {
			return new(messages.BatchRequest)
		},
		Config: map[Channel]flow.RoleList{
			SyncCommittee:     flow.Roles(),
			SyncClusterPrefix: flow.Roles(),
		},
	}
	MessageAuthConfigs["BlockResponse"] = MsgAuthConfig{
		String: "BlockResponse",
		Interface: func() interface{} {
			return new(messages.BlockResponse)
		},
		Config: map[Channel]flow.RoleList{
			SyncCommittee:     flow.Roles(),
			SyncClusterPrefix: flow.Roles(),
		},
	}

	// cluster consensus
	MessageAuthConfigs["ClusterBlockProposal"] = MsgAuthConfig{
		String: "ClusterBlockProposal",
		Interface: func() interface{} {
			return new(messages.ClusterBlockProposal)
		},
		Config: map[Channel]flow.RoleList{
			ConsensusClusterPrefix: {flow.RoleCollection},
		},
	}
	MessageAuthConfigs["ClusterBlockVote"] = MsgAuthConfig{
		String: "ClusterBlockVote",
		Interface: func() interface{} {
			return new(messages.ClusterBlockVote)
		},
		Config: map[Channel]flow.RoleList{
			ConsensusClusterPrefix: {flow.RoleCollection},
		},
	}
	MessageAuthConfigs["ClusterBlockResponse"] = MsgAuthConfig{
		String: "ClusterBlockResponse",
		Interface: func() interface{} {
			return new(messages.ClusterBlockResponse)
		},
		Config: map[Channel]flow.RoleList{
			ConsensusClusterPrefix: {flow.RoleCollection},
		},
	}

	// collections, guarantees & transactions
	MessageAuthConfigs["CollectionGuarantee"] = MsgAuthConfig{
		String: "CollectionGuarantee",
		Interface: func() interface{} {
			return new(flow.CollectionGuarantee)
		},
		Config: map[Channel]flow.RoleList{
			PushGuarantees: {flow.RoleCollection}, // channel alias ReceiveGuarantees = PushGuarantees
		},
	}
	MessageAuthConfigs["Transaction"] = MsgAuthConfig{
		String: "Transaction",
		Interface: func() interface{} {
			return new(flow.Transaction)
		},
		Config: map[Channel]flow.RoleList{
			PushTransactions: {flow.RoleCollection}, // channel alias ReceiveTransactions = PushTransactions
		},
	}
	MessageAuthConfigs["TransactionBody"] = MsgAuthConfig{
		String: "TransactionBody",
		Interface: func() interface{} {
			return new(flow.TransactionBody)
		},
		Config: map[Channel]flow.RoleList{
			PushTransactions: {flow.RoleCollection}, // channel alias ReceiveTransactions = PushTransactions
		},
	}

	// core messages for execution & verification
	MessageAuthConfigs["ExecutionReceipt"] = MsgAuthConfig{
		String: "ExecutionReceipt",
		Interface: func() interface{} {
			return new(flow.ExecutionReceipt)
		},
		Config: map[Channel]flow.RoleList{
			PushReceipts: {flow.RoleExecution}, // channel alias ReceiveReceipts = PushReceipts
		},
	}
	MessageAuthConfigs["ResultApproval"] = MsgAuthConfig{
		String: "ResultApproval",
		Interface: func() interface{} {
			return new(flow.ResultApproval)
		},
		Config: map[Channel]flow.RoleList{
			PushApprovals: {flow.RoleVerification}, // channel alias ReceiveApprovals = PushApprovals
		},
	}

	// [deprecated] execution state synchronization
	MessageAuthConfigs["ExecutionStateSyncRequest"] = MsgAuthConfig{
		String: "ExecutionStateSyncRequest",
		Config: nil,
	}
	MessageAuthConfigs["ExecutionStateDelta"] = MsgAuthConfig{
		String: "ExecutionStateDelta",
		Config: nil,
	}

	// data exchange for execution of blocks
	MessageAuthConfigs["ChunkDataRequest"] = MsgAuthConfig{
		String: "ChunkDataRequest",
		Interface: func() interface{} {
			return new(messages.ChunkDataRequest)
		},
		Config: map[Channel]flow.RoleList{
			ProvideChunks:            {flow.RoleVerification}, // channel alias RequestChunks = ProvideChunks
			RequestCollections:       {flow.RoleVerification},
			RequestApprovalsByChunk:  {flow.RoleVerification},
			RequestReceiptsByBlockID: {flow.RoleVerification},
		},
	}
	MessageAuthConfigs["ChunkDataResponse"] = MsgAuthConfig{
		String: "ChunkDataResponse",
		Interface: func() interface{} {
			return new(messages.ChunkDataResponse)
		},
		Config: map[Channel]flow.RoleList{
			ProvideChunks:            {flow.RoleExecution}, // channel alias RequestChunks = ProvideChunks
			RequestCollections:       {flow.RoleExecution},
			RequestApprovalsByChunk:  {flow.RoleExecution},
			RequestReceiptsByBlockID: {flow.RoleExecution},
		},
	}

	// result approvals
	MessageAuthConfigs["ApprovalRequest"] = MsgAuthConfig{
		String: "ApprovalRequest",
		Interface: func() interface{} {
			return new(messages.ApprovalRequest)
		},
		Config: map[Channel]flow.RoleList{
			ProvideApprovalsByChunk: {flow.RoleConsensus},
		},
	}
	MessageAuthConfigs["ApprovalResponse"] = MsgAuthConfig{
		String: "ApprovalResponse",
		Interface: func() interface{} {
			return new(messages.ApprovalResponse)
		},
		Config: map[Channel]flow.RoleList{
			ProvideApprovalsByChunk: {flow.RoleVerification},
		},
	}

	// generic entity exchange engines
	MessageAuthConfigs["EntityRequest"] = MsgAuthConfig{
		String: "EntityRequest",
		Interface: func() interface{} {
			return new(messages.EntityRequest)
		},
		Config: map[Channel]flow.RoleList{
			RequestChunks:            {flow.RoleAccess, flow.RoleConsensus, flow.RoleCollection},
			RequestCollections:       {flow.RoleAccess, flow.RoleConsensus, flow.RoleCollection},
			RequestApprovalsByChunk:  {flow.RoleAccess, flow.RoleConsensus, flow.RoleCollection},
			RequestReceiptsByBlockID: {flow.RoleAccess, flow.RoleConsensus, flow.RoleCollection},
		},
	}
	MessageAuthConfigs["EntityResponse"] = MsgAuthConfig{
		String: "EntityResponse",
		Interface: func() interface{} {
			return new(messages.EntityResponse)
		},
		Config: map[Channel]flow.RoleList{
			RequestChunks:            {flow.RoleCollection, flow.RoleExecution},
			RequestCollections:       {flow.RoleCollection, flow.RoleExecution},
			RequestApprovalsByChunk:  {flow.RoleCollection, flow.RoleExecution},
			RequestReceiptsByBlockID: {flow.RoleCollection, flow.RoleExecution},
		},
	}

	// testing
	MessageAuthConfigs["Echo"] = MsgAuthConfig{
		String: "echo",
		Interface: func() interface{} {
			return new(message.TestMessage)
		},
		Config: map[Channel]flow.RoleList{
			TestNetworkChannel: flow.Roles(),
			TestMetricsChannel: flow.Roles(),
		},
	}

	// DKG
	MessageAuthConfigs["DKGMessage"] = MsgAuthConfig{
		String: "DKGMessage",
		Interface: func() interface{} {
			return new(messages.DKGMessage)
		},
		Config: map[Channel]flow.RoleList{
			DKGCommittee: {flow.RoleConsensus},
		},
	}
}

// GetMessageAuthConfig checks the underlying type and returns the correct
// message auth Config.
// Expected error returns during normal operations:
//  * ErrUnknownMsgType : if underlying type of v does  not match any of the known message types
func GetMessageAuthConfig(v interface{}) (MsgAuthConfig, error) {
	switch v.(type) {
	// consensus
	case *messages.BlockProposal:
		return MessageAuthConfigs["BlockProposal"], nil
	case *messages.BlockVote:
		return MessageAuthConfigs["BlockVote"], nil

	// protocol state sync
	case *messages.SyncRequest:
		return MessageAuthConfigs["SyncRequest"], nil
	case *messages.SyncResponse:
		return MessageAuthConfigs["SyncResponse"], nil
	case *messages.RangeRequest:
		return MessageAuthConfigs["RangeRequest"], nil
	case *messages.BatchRequest:
		return MessageAuthConfigs["BatchRequest"], nil
	case *messages.BlockResponse:
		return MessageAuthConfigs["BlockResponse"], nil

	// cluster consensus
	case *messages.ClusterBlockProposal:
		return MessageAuthConfigs["ClusterBlockProposal"], nil
	case *messages.ClusterBlockVote:
		return MessageAuthConfigs["ClusterBlockVote"], nil
	case *messages.ClusterBlockResponse:
		return MessageAuthConfigs["ClusterBlockResponse"], nil

	// collections, guarantees & transactions
	case *flow.CollectionGuarantee:
		return MessageAuthConfigs["CollectionGuarantee"], nil
	case *flow.TransactionBody:
		return MessageAuthConfigs["TransactionBody"], nil
	case *flow.Transaction:
		return MessageAuthConfigs["Transaction"], nil

	// core messages for execution & verification
	case *flow.ExecutionReceipt:
		return MessageAuthConfigs["ExecutionReceipt"], nil
	case *flow.ResultApproval:
		return MessageAuthConfigs["ResultApproval"], nil

	// execution state synchronization
	case *messages.ExecutionStateSyncRequest:
		return MessageAuthConfigs["ExecutionStateSyncRequest"], nil
	case *messages.ExecutionStateDelta:
		return MessageAuthConfigs["ExecutionStateDelta"], nil

	// data exchange for execution of blocks
	case *messages.ChunkDataRequest:
		return MessageAuthConfigs["ChunkDataRequest"], nil
	case *messages.ChunkDataResponse:
		return MessageAuthConfigs["ChunkDataResponse"], nil

	// result approvals
	case *messages.ApprovalRequest:
		return MessageAuthConfigs["ApprovalRequest"], nil
	case *messages.ApprovalResponse:
		return MessageAuthConfigs["ApprovalResponse"], nil

	// generic entity exchange engines
	case *messages.EntityRequest:
		return MessageAuthConfigs["EntityRequest"], nil
	case *messages.EntityResponse:
		return MessageAuthConfigs["EntityResponse"], nil

	// testing
	case *message.TestMessage:
		return MessageAuthConfigs["TestMessage"], nil

	// dkg
	case *messages.DKGMessage:
		return MessageAuthConfigs["DKGMessage"], nil

	default:
		return MsgAuthConfig{}, fmt.Errorf("%w (%T)", ErrUnknownMsgType, v)
	}
}
