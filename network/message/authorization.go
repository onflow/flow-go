package message

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network/channels"
)

var AuthorizationConfigs map[string]MsgAuthConfig

// MsgAuthConfig contains authorization information for a specific flow message. The authorization
// is represented as a map from network channel -> list of all roles allowed to send the message on
// the channel.
type MsgAuthConfig struct {
	// Name is the string representation of the message type.
	Name string
	// Type is a func that returns a new instance of message type.
	Type func() interface{}
	// Config is the mapping of network channel to list of authorized flow roles.
	Config map[channels.Channel]flow.RoleList
}

// IsAuthorized checks if the specified role is authorized to send the message on the provided channel and
// asserts that the message is authorized to be sent on the channel.
// Expected error returns during normal operations:
//  * ErrUnauthorizedMessageOnChannel: the channel is not included in the message's list of authorized channels
//  * ErrUnauthorizedRole: the role is not included in the message's list of authorized roles for the provided channel
func (m MsgAuthConfig) IsAuthorized(role flow.Role, channel channels.Channel) error {
	authorizedRoles, ok := m.Config[channel]
	if !ok {
		return ErrUnauthorizedMessageOnChannel
	}

	if !authorizedRoles.Contains(role) {
		return ErrUnauthorizedRole
	}

	return nil
}

func initializeMessageAuthConfigsMap() {
	AuthorizationConfigs = make(map[string]MsgAuthConfig)

	// consensus
	AuthorizationConfigs[BlockProposal] = MsgAuthConfig{
		Name: BlockProposal,
		Type: func() interface{} {
			return new(messages.BlockProposal)
		},
		Config: map[channels.Channel]flow.RoleList{
			channels.ConsensusCommittee: {flow.RoleConsensus},
			channels.PushBlocks:         {flow.RoleConsensus}, // channel alias ReceiveBlocks = PushBlocks
		},
	}
	AuthorizationConfigs[BlockVote] = MsgAuthConfig{
		Name: BlockVote,
		Type: func() interface{} {
			return new(messages.BlockVote)
		},
		Config: map[channels.Channel]flow.RoleList{
			channels.ConsensusCommittee: {flow.RoleConsensus},
		},
	}

	// protocol state sync
	AuthorizationConfigs[SyncRequest] = MsgAuthConfig{
		Name: SyncRequest,
		Type: func() interface{} {
			return new(messages.SyncRequest)
		},
		Config: map[channels.Channel]flow.RoleList{
			channels.SyncCommittee:     flow.Roles(),
			channels.SyncClusterPrefix: {flow.RoleCollection},
		},
	}
	AuthorizationConfigs[SyncResponse] = MsgAuthConfig{
		Name: SyncResponse,
		Type: func() interface{} {
			return new(messages.SyncResponse)
		},
		Config: map[channels.Channel]flow.RoleList{
			channels.SyncCommittee:     {flow.RoleConsensus},
			channels.SyncClusterPrefix: {flow.RoleCollection},
		},
	}
	AuthorizationConfigs[RangeRequest] = MsgAuthConfig{
		Name: RangeRequest,
		Type: func() interface{} {
			return new(messages.RangeRequest)
		},
		Config: map[channels.Channel]flow.RoleList{
			channels.SyncCommittee:     flow.Roles(),
			channels.SyncClusterPrefix: {flow.RoleCollection},
		},
	}
	AuthorizationConfigs[BatchRequest] = MsgAuthConfig{
		Name: BatchRequest,
		Type: func() interface{} {
			return new(messages.BatchRequest)
		},
		Config: map[channels.Channel]flow.RoleList{
			channels.SyncCommittee:     flow.Roles(),
			channels.SyncClusterPrefix: {flow.RoleCollection},
		},
	}
	AuthorizationConfigs[BlockResponse] = MsgAuthConfig{
		Name: BlockResponse,
		Type: func() interface{} {
			return new(messages.BlockResponse)
		},
		Config: map[channels.Channel]flow.RoleList{
			channels.SyncCommittee: {flow.RoleConsensus},
		},
	}

	// cluster consensus
	AuthorizationConfigs[ClusterBlockProposal] = MsgAuthConfig{
		Name: ClusterBlockProposal,
		Type: func() interface{} {
			return new(messages.ClusterBlockProposal)
		},
		Config: map[channels.Channel]flow.RoleList{
			channels.ConsensusClusterPrefix: {flow.RoleCollection},
		},
	}
	AuthorizationConfigs[ClusterBlockVote] = MsgAuthConfig{
		Name: ClusterBlockVote,
		Type: func() interface{} {
			return new(messages.ClusterBlockVote)
		},
		Config: map[channels.Channel]flow.RoleList{
			channels.ConsensusClusterPrefix: {flow.RoleCollection},
		},
	}
	AuthorizationConfigs[ClusterBlockResponse] = MsgAuthConfig{
		Name: ClusterBlockResponse,
		Type: func() interface{} {
			return new(messages.ClusterBlockResponse)
		},
		Config: map[channels.Channel]flow.RoleList{
			channels.SyncClusterPrefix: {flow.RoleCollection},
		},
	}

	// collections, guarantees & transactions
	AuthorizationConfigs[CollectionGuarantee] = MsgAuthConfig{
		Name: CollectionGuarantee,
		Type: func() interface{} {
			return new(flow.CollectionGuarantee)
		},
		Config: map[channels.Channel]flow.RoleList{
			channels.PushGuarantees: {flow.RoleCollection}, // channel alias ReceiveGuarantees = PushGuarantees
		},
	}
	AuthorizationConfigs[TransactionBody] = MsgAuthConfig{
		Name: TransactionBody,
		Type: func() interface{} {
			return new(flow.TransactionBody)
		},
		Config: map[channels.Channel]flow.RoleList{
			channels.PushTransactions: {flow.RoleCollection}, // channel alias ReceiveTransactions = PushTransactions
		},
	}

	// core messages for execution & verification
	AuthorizationConfigs[ExecutionReceipt] = MsgAuthConfig{
		Name: ExecutionReceipt,
		Type: func() interface{} {
			return new(flow.ExecutionReceipt)
		},
		Config: map[channels.Channel]flow.RoleList{
			channels.PushReceipts: {flow.RoleExecution}, // channel alias ReceiveReceipts = PushReceipts
		},
	}
	AuthorizationConfigs[ResultApproval] = MsgAuthConfig{
		Name: ResultApproval,
		Type: func() interface{} {
			return new(flow.ResultApproval)
		},
		Config: map[channels.Channel]flow.RoleList{
			channels.PushApprovals: {flow.RoleVerification}, // channel alias ReceiveApprovals = PushApprovals
		},
	}

	// data exchange for execution of blocks
	AuthorizationConfigs[ChunkDataRequest] = MsgAuthConfig{
		Name: ChunkDataRequest,
		Type: func() interface{} {
			return new(messages.ChunkDataRequest)
		},
		Config: map[channels.Channel]flow.RoleList{
			channels.RequestChunks: {flow.RoleVerification}, // channel alias RequestChunks = ProvideChunks
		},
	}
	AuthorizationConfigs[ChunkDataResponse] = MsgAuthConfig{
		Name: ChunkDataResponse,
		Type: func() interface{} {
			return new(messages.ChunkDataResponse)
		},
		Config: map[channels.Channel]flow.RoleList{
			channels.ProvideChunks: {flow.RoleExecution}, // channel alias RequestChunks = ProvideChunks
		},
	}

	// result approvals
	AuthorizationConfigs[ApprovalRequest] = MsgAuthConfig{
		Name: ApprovalRequest,
		Type: func() interface{} {
			return new(messages.ApprovalRequest)
		},
		Config: map[channels.Channel]flow.RoleList{
			channels.RequestApprovalsByChunk: {flow.RoleConsensus}, // channel alias ProvideApprovalsByChunk  = RequestApprovalsByChunk
		},
	}
	AuthorizationConfigs[ApprovalResponse] = MsgAuthConfig{
		Name: ApprovalResponse,
		Type: func() interface{} {
			return new(messages.ApprovalResponse)
		},
		Config: map[channels.Channel]flow.RoleList{
			channels.ProvideApprovalsByChunk: {flow.RoleVerification}, // channel alias ProvideApprovalsByChunk  = RequestApprovalsByChunk

		},
	}

	// generic entity exchange engines
	AuthorizationConfigs[EntityRequest] = MsgAuthConfig{
		Name: EntityRequest,
		Type: func() interface{} {
			return new(messages.EntityRequest)
		},
		Config: map[channels.Channel]flow.RoleList{
			channels.RequestReceiptsByBlockID: {flow.RoleConsensus},
			channels.RequestCollections:       {flow.RoleAccess, flow.RoleExecution},
		},
	}
	AuthorizationConfigs[EntityResponse] = MsgAuthConfig{
		Name: EntityResponse,
		Type: func() interface{} {
			return new(messages.EntityResponse)
		},
		Config: map[channels.Channel]flow.RoleList{
			channels.ProvideApprovalsByChunk:  {flow.RoleVerification},
			channels.ProvideReceiptsByBlockID: {flow.RoleExecution},
		},
	}

	// testing
	AuthorizationConfigs[TestMessage] = MsgAuthConfig{
		Name: TestMessage,
		Type: func() interface{} {
			return new(message.TestMessage)
		},
		Config: map[channels.Channel]flow.RoleList{
			channels.TestNetworkChannel: flow.Roles(),
			channels.TestMetricsChannel: flow.Roles(),
		},
	}

	// DKG
	AuthorizationConfigs[DKGMessage] = MsgAuthConfig{
		Name: DKGMessage,
		Type: func() interface{} {
			return new(messages.DKGMessage)
		},
		Config: map[channels.Channel]flow.RoleList{
			channels.DKGCommittee: {flow.RoleConsensus},
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
		return AuthorizationConfigs[BlockProposal], nil
	case *messages.BlockVote:
		return AuthorizationConfigs[BlockVote], nil

	// protocol state sync
	case *messages.SyncRequest:
		return AuthorizationConfigs[SyncRequest], nil
	case *messages.SyncResponse:
		return AuthorizationConfigs[SyncResponse], nil
	case *messages.RangeRequest:
		return AuthorizationConfigs[RangeRequest], nil
	case *messages.BatchRequest:
		return AuthorizationConfigs[BatchRequest], nil
	case *messages.BlockResponse:
		return AuthorizationConfigs[BlockResponse], nil

	// cluster consensus
	case *messages.ClusterBlockProposal:
		return AuthorizationConfigs[ClusterBlockProposal], nil
	case *messages.ClusterBlockVote:
		return AuthorizationConfigs[ClusterBlockVote], nil
	case *messages.ClusterBlockResponse:
		return AuthorizationConfigs[ClusterBlockResponse], nil

	// collections, guarantees & transactions
	case *flow.CollectionGuarantee:
		return AuthorizationConfigs[CollectionGuarantee], nil
	case *flow.TransactionBody:
		return AuthorizationConfigs[TransactionBody], nil
	case *flow.Transaction:
		return AuthorizationConfigs[Transaction], nil

	// core messages for execution & verification
	case *flow.ExecutionReceipt:
		return AuthorizationConfigs[ExecutionReceipt], nil
	case *flow.ResultApproval:
		return AuthorizationConfigs[ResultApproval], nil

	// execution state synchronization
	case *messages.ExecutionStateSyncRequest:
		return AuthorizationConfigs[ExecutionStateSyncRequest], nil
	case *messages.ExecutionStateDelta:
		return AuthorizationConfigs[ExecutionStateDelta], nil

	// data exchange for execution of blocks
	case *messages.ChunkDataRequest:
		return AuthorizationConfigs[ChunkDataRequest], nil
	case *messages.ChunkDataResponse:
		return AuthorizationConfigs[ChunkDataResponse], nil

	// result approvals
	case *messages.ApprovalRequest:
		return AuthorizationConfigs[ApprovalRequest], nil
	case *messages.ApprovalResponse:
		return AuthorizationConfigs[ApprovalResponse], nil

	// generic entity exchange engines
	case *messages.EntityRequest:
		return AuthorizationConfigs[EntityRequest], nil
	case *messages.EntityResponse:
		return AuthorizationConfigs[EntityResponse], nil

	// testing
	case *message.TestMessage:
		return AuthorizationConfigs[TestMessage], nil

	// dkg
	case *messages.DKGMessage:
		return AuthorizationConfigs[DKGMessage], nil

	default:
		return MsgAuthConfig{}, NewUnknownMsgTypeErr(v)
	}
}
