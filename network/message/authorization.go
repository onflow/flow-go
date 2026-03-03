package message

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network/channels"
)

type ChannelAuthConfig struct {
	// AuthorizedRoles list of roles authorized to send this message on the channel.
	AuthorizedRoles flow.RoleList

	// AllowedProtocols list of protocols the message is allowed to be sent on. Currently AllowedProtocols is expected to have
	// exactly one element in the list. This is due to the fact that currently there are no messages that are used with both protocols aside from TestMessage.
	AllowedProtocols Protocols
}

var authorizationConfigs map[string]MsgAuthConfig

// MsgAuthConfig contains authorization information for a specific flow message. The authorization
// is represented as a map from network channel -> list of all roles allowed to send the message on
// the channel.
type MsgAuthConfig struct {
	// Name is the string representation of the message type.
	Name string
	// Type is a func that returns a new instance of message type.
	Type func() any
	// Config is the mapping of network channel to list of authorized flow roles.
	Config map[channels.Channel]ChannelAuthConfig
}

// EnsureAuthorized checks if the specified role is authorized to send the message on the provided channel and
// asserts that the message is authorized to be sent on the channel.
// Expected error returns during normal operations:
//   - ErrUnauthorizedMessageOnChannel: the channel is not included in the message's list of authorized channels
//   - ErrUnauthorizedRole: the role is not included in the message's list of authorized roles for the provided channel
//   - ErrUnauthorizedUnicastOnChannel: the message is not authorized to be sent via unicast protocol.
//   - ErrUnauthorizedPublishOnChannel: the message is not authorized to be sent via publish protocol.
func (m MsgAuthConfig) EnsureAuthorized(role flow.Role, channel channels.Channel, protocol ProtocolType) error {
	authConfig, ok := m.Config[channel]
	if !ok {
		return ErrUnauthorizedMessageOnChannel
	}

	if !authConfig.AuthorizedRoles.Contains(role) {
		return ErrUnauthorizedRole
	}

	if !authConfig.AllowedProtocols.Contains(protocol) {
		return NewUnauthorizedProtocolError(protocol)
	}

	return nil
}

func initializeMessageAuthConfigsMap() {
	authorizationConfigs = make(map[string]MsgAuthConfig)

	// consensus
	authorizationConfigs[BlockProposal] = MsgAuthConfig{
		Name: BlockProposal,
		Type: func() any {
			return new(messages.Proposal)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.ConsensusCommittee: {
				AuthorizedRoles:  flow.RoleList{flow.RoleConsensus},
				AllowedProtocols: Protocols{ProtocolTypePubSub},
			},
			channels.PushBlocks: {
				AuthorizedRoles:  flow.RoleList{flow.RoleConsensus},
				AllowedProtocols: Protocols{ProtocolTypePubSub},
			}, // channel alias ReceiveBlocks = PushBlocks
		},
	}
	authorizationConfigs[BlockVote] = MsgAuthConfig{
		Name: BlockVote,
		Type: func() any {
			return new(messages.BlockVote)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.ConsensusCommittee: {
				AuthorizedRoles:  flow.RoleList{flow.RoleConsensus},
				AllowedProtocols: Protocols{ProtocolTypeUnicast},
			},
		},
	}
	authorizationConfigs[TimeoutObject] = MsgAuthConfig{
		Name: TimeoutObject,
		Type: func() any {
			return new(messages.TimeoutObject)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.ConsensusCommittee: {
				AuthorizedRoles:  flow.RoleList{flow.RoleConsensus},
				AllowedProtocols: Protocols{ProtocolTypePubSub},
			},
		},
	}

	// protocol state sync
	authorizationConfigs[SyncRequest] = MsgAuthConfig{
		Name: SyncRequest,
		Type: func() any {
			return new(messages.SyncRequest)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.SyncCommittee: {
				AuthorizedRoles:  flow.Roles(),
				AllowedProtocols: Protocols{ProtocolTypePubSub},
			},
			channels.SyncClusterPrefix: {
				AuthorizedRoles:  flow.RoleList{flow.RoleCollection},
				AllowedProtocols: Protocols{ProtocolTypePubSub},
			},
		},
	}
	authorizationConfigs[SyncResponse] = MsgAuthConfig{
		Name: SyncResponse,
		Type: func() any {
			return new(messages.SyncResponse)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.SyncCommittee: {
				AuthorizedRoles:  flow.RoleList{flow.RoleConsensus},
				AllowedProtocols: Protocols{ProtocolTypeUnicast},
			},
			channels.SyncClusterPrefix: {
				AuthorizedRoles:  flow.RoleList{flow.RoleCollection},
				AllowedProtocols: Protocols{ProtocolTypeUnicast},
			},
		},
	}
	authorizationConfigs[RangeRequest] = MsgAuthConfig{
		Name: RangeRequest,
		Type: func() any {
			return new(messages.RangeRequest)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.SyncCommittee: {
				AuthorizedRoles:  flow.Roles(),
				AllowedProtocols: Protocols{ProtocolTypePubSub},
			},
			channels.SyncClusterPrefix: {
				AuthorizedRoles:  flow.RoleList{flow.RoleCollection},
				AllowedProtocols: Protocols{ProtocolTypePubSub},
			},
		},
	}
	authorizationConfigs[BatchRequest] = MsgAuthConfig{
		Name: BatchRequest,
		Type: func() any {
			return new(messages.BatchRequest)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.SyncCommittee: {
				AuthorizedRoles:  flow.Roles(),
				AllowedProtocols: Protocols{ProtocolTypePubSub},
			},
			channels.SyncClusterPrefix: {
				AuthorizedRoles:  flow.RoleList{flow.RoleCollection},
				AllowedProtocols: Protocols{ProtocolTypePubSub},
			},
		},
	}
	authorizationConfigs[BlockResponse] = MsgAuthConfig{
		Name: BlockResponse,
		Type: func() any {
			return new(messages.BlockResponse)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.SyncCommittee: {
				AuthorizedRoles:  flow.RoleList{flow.RoleConsensus},
				AllowedProtocols: Protocols{ProtocolTypeUnicast},
			},
		},
	}

	// cluster consensus
	authorizationConfigs[ClusterBlockProposal] = MsgAuthConfig{
		Name: ClusterBlockProposal,
		Type: func() any {
			return new(messages.ClusterProposal)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.ConsensusClusterPrefix: {
				AuthorizedRoles:  flow.RoleList{flow.RoleCollection},
				AllowedProtocols: Protocols{ProtocolTypePubSub},
			},
		},
	}
	authorizationConfigs[ClusterBlockVote] = MsgAuthConfig{
		Name: ClusterBlockVote,
		Type: func() any {
			return new(messages.ClusterBlockVote)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.ConsensusClusterPrefix: {
				AuthorizedRoles:  flow.RoleList{flow.RoleCollection},
				AllowedProtocols: Protocols{ProtocolTypeUnicast},
			},
		},
	}
	authorizationConfigs[ClusterTimeoutObject] = MsgAuthConfig{
		Name: ClusterTimeoutObject,
		Type: func() any {
			return new(messages.ClusterTimeoutObject)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.ConsensusClusterPrefix: {
				AuthorizedRoles:  flow.RoleList{flow.RoleCollection},
				AllowedProtocols: Protocols{ProtocolTypePubSub},
			},
		},
	}
	authorizationConfigs[ClusterBlockResponse] = MsgAuthConfig{
		Name: ClusterBlockResponse,
		Type: func() any {
			return new(messages.ClusterBlockResponse)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.SyncClusterPrefix: {
				AuthorizedRoles:  flow.RoleList{flow.RoleCollection},
				AllowedProtocols: Protocols{ProtocolTypeUnicast},
			},
		},
	}

	// collections, guarantees & transactions
	authorizationConfigs[CollectionGuarantee] = MsgAuthConfig{
		Name: CollectionGuarantee,
		Type: func() any {
			return new(messages.CollectionGuarantee)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.PushGuarantees: {
				AuthorizedRoles:  flow.RoleList{flow.RoleCollection},
				AllowedProtocols: Protocols{ProtocolTypePubSub},
			}, // channel alias ReceiveGuarantees = PushGuarantees
		},
	}
	authorizationConfigs[TransactionBody] = MsgAuthConfig{
		Name: TransactionBody,
		Type: func() any {
			return new(messages.TransactionBody)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.PushTransactions: {
				AuthorizedRoles:  flow.RoleList{flow.RoleCollection},
				AllowedProtocols: Protocols{ProtocolTypePubSub},
			}, // channel alias ReceiveTransactions = PushTransactions
		},
	}

	// core messages for execution & verification
	authorizationConfigs[ExecutionReceipt] = MsgAuthConfig{
		Name: ExecutionReceipt,
		Type: func() any {
			return new(messages.ExecutionReceipt)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.PushReceipts: {
				AuthorizedRoles:  flow.RoleList{flow.RoleExecution},
				AllowedProtocols: Protocols{ProtocolTypePubSub},
			}, // channel alias ReceiveReceipts = PushReceipts
		},
	}
	authorizationConfigs[ResultApproval] = MsgAuthConfig{
		Name: ResultApproval,
		Type: func() any {
			return new(messages.ResultApproval)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.PushApprovals: {
				AuthorizedRoles:  flow.RoleList{flow.RoleVerification},
				AllowedProtocols: Protocols{ProtocolTypePubSub},
			}, // channel alias ReceiveApprovals = PushApprovals
		},
	}

	// data exchange for execution of blocks
	authorizationConfigs[ChunkDataRequest] = MsgAuthConfig{
		Name: ChunkDataRequest,
		Type: func() any {
			return new(messages.ChunkDataRequest)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.RequestChunks: {
				AuthorizedRoles:  flow.RoleList{flow.RoleVerification},
				AllowedProtocols: Protocols{ProtocolTypePubSub},
			}, // channel alias RequestChunks = ProvideChunks
		},
	}
	authorizationConfigs[ChunkDataResponse] = MsgAuthConfig{
		Name: ChunkDataResponse,
		Type: func() any {
			return new(messages.ChunkDataResponse)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.ProvideChunks: {
				AuthorizedRoles:  flow.RoleList{flow.RoleExecution},
				AllowedProtocols: Protocols{ProtocolTypeUnicast},
			}, // channel alias RequestChunks = ProvideChunks
		},
	}

	// result approvals
	authorizationConfigs[ApprovalRequest] = MsgAuthConfig{
		Name: ApprovalRequest,
		Type: func() any {
			return new(messages.ApprovalRequest)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.RequestApprovalsByChunk: {
				AuthorizedRoles:  flow.RoleList{flow.RoleConsensus},
				AllowedProtocols: Protocols{ProtocolTypePubSub},
			}, // channel alias ProvideApprovalsByChunk  = RequestApprovalsByChunk
		},
	}
	authorizationConfigs[ApprovalResponse] = MsgAuthConfig{
		Name: ApprovalResponse,
		Type: func() any {
			return new(messages.ApprovalResponse)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.ProvideApprovalsByChunk: {
				AuthorizedRoles:  flow.RoleList{flow.RoleVerification},
				AllowedProtocols: Protocols{ProtocolTypeUnicast},
			}, // channel alias ProvideApprovalsByChunk  = RequestApprovalsByChunk
		},
	}

	// generic entity exchange engines
	authorizationConfigs[EntityRequest] = MsgAuthConfig{
		Name: EntityRequest,
		Type: func() any {
			return new(messages.EntityRequest)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.RequestReceiptsByBlockID: {
				AuthorizedRoles:  flow.RoleList{flow.RoleConsensus},
				AllowedProtocols: Protocols{ProtocolTypeUnicast},
			},
			channels.RequestCollections: {
				AuthorizedRoles:  flow.RoleList{flow.RoleAccess, flow.RoleExecution},
				AllowedProtocols: Protocols{ProtocolTypeUnicast},
			},
		},
	}
	authorizationConfigs[EntityResponse] = MsgAuthConfig{
		Name: EntityResponse,
		Type: func() any {
			return new(messages.EntityResponse)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.ProvideReceiptsByBlockID: {
				AuthorizedRoles:  flow.RoleList{flow.RoleExecution},
				AllowedProtocols: Protocols{ProtocolTypeUnicast},
			},
			channels.ProvideCollections: {
				AuthorizedRoles:  flow.RoleList{flow.RoleCollection},
				AllowedProtocols: Protocols{ProtocolTypeUnicast},
			},
		},
	}

	// testing
	authorizationConfigs[TestMessage] = MsgAuthConfig{
		Name: TestMessage,
		Type: func() any {
			return new(message.TestMessage)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.TestNetworkChannel: {
				AuthorizedRoles:  flow.Roles(),
				AllowedProtocols: Protocols{ProtocolTypePubSub, ProtocolTypeUnicast},
			},
			channels.TestMetricsChannel: {
				AuthorizedRoles:  flow.Roles(),
				AllowedProtocols: Protocols{ProtocolTypePubSub, ProtocolTypeUnicast},
			},
		},
	}

	// DKG
	authorizationConfigs[DKGMessage] = MsgAuthConfig{
		Name: DKGMessage,
		Type: func() any {
			return new(messages.DKGMessage)
		},
		Config: map[channels.Channel]ChannelAuthConfig{
			channels.DKGCommittee: {
				AuthorizedRoles:  flow.RoleList{flow.RoleConsensus},
				AllowedProtocols: Protocols{ProtocolTypeUnicast},
			},
		},
	}
}

// GetMessageAuthConfig checks the underlying type and returns the correct
// message auth Config.
// Expected error returns during normal operations:
//   - ErrUnknownMsgType : if underlying type of v does  not match any of the known message types
func GetMessageAuthConfig(v any) (MsgAuthConfig, error) {
	switch v.(type) {
	// consensus
	case *messages.Proposal:
		return authorizationConfigs[BlockProposal], nil
	case *messages.BlockVote:
		return authorizationConfigs[BlockVote], nil
	case *messages.TimeoutObject:
		return authorizationConfigs[TimeoutObject], nil

	// protocol state sync
	case *messages.SyncRequest:
		return authorizationConfigs[SyncRequest], nil
	case *messages.SyncResponse:
		return authorizationConfigs[SyncResponse], nil
	case *messages.RangeRequest:
		return authorizationConfigs[RangeRequest], nil
	case *messages.BatchRequest:
		return authorizationConfigs[BatchRequest], nil
	case *messages.BlockResponse:
		return authorizationConfigs[BlockResponse], nil

	// cluster consensus
	case *messages.ClusterProposal:
		return authorizationConfigs[ClusterBlockProposal], nil
	case *messages.ClusterBlockVote:
		return authorizationConfigs[ClusterBlockVote], nil
	case *messages.ClusterTimeoutObject:
		return authorizationConfigs[ClusterTimeoutObject], nil
	case *messages.ClusterBlockResponse:
		return authorizationConfigs[ClusterBlockResponse], nil

	// collections, guarantees & transactions
	case *messages.CollectionGuarantee:
		return authorizationConfigs[CollectionGuarantee], nil
	case *messages.TransactionBody:
		return authorizationConfigs[TransactionBody], nil

	// core messages for execution & verification
	case *messages.ExecutionReceipt:
		return authorizationConfigs[ExecutionReceipt], nil
	case *messages.ResultApproval:
		return authorizationConfigs[ResultApproval], nil

	// data exchange for execution of blocks
	case *messages.ChunkDataRequest:
		return authorizationConfigs[ChunkDataRequest], nil
	case *messages.ChunkDataResponse:
		return authorizationConfigs[ChunkDataResponse], nil

	// result approvals
	case *messages.ApprovalRequest:
		return authorizationConfigs[ApprovalRequest], nil
	case *messages.ApprovalResponse:
		return authorizationConfigs[ApprovalResponse], nil

	// generic entity exchange engines
	case *messages.EntityRequest:
		return authorizationConfigs[EntityRequest], nil
	case *messages.EntityResponse:
		return authorizationConfigs[EntityResponse], nil

	// testing
	case *message.TestMessage:
		return authorizationConfigs[TestMessage], nil

	// dkg
	case *messages.DKGMessage:
		return authorizationConfigs[DKGMessage], nil

	default:
		return MsgAuthConfig{}, NewUnknownMsgTypeErr(v)
	}
}

// GetAllMessageAuthConfigs returns all the configured message auth configurations.
func GetAllMessageAuthConfigs() []MsgAuthConfig {
	configs := make([]MsgAuthConfig, len(authorizationConfigs))

	i := 0
	for _, config := range authorizationConfigs {
		configs[i] = config
		i++
	}

	return configs
}
