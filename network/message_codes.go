package network

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network/codec"
)

// codeToMessageCodes is a mapping of Code -> MessageCode wrapper that contains authorization information used for validation
type codeToMessageCodes map[codec.Code]MessageCode

// MessageCode wrapper around Code that contains channel authorization information for the message Code.
type MessageCode struct {
	// Code is the underlying message code byte
	Code codec.Code

	// authorizedChannelsMap is a mapping of channels to authorized roles allowed to send MessageCode on the channel
	authorizedChannelsMap map[Channel]flow.RoleList
}

// AuthorizedRolesByChannel returns the list of roles authorized to send this message code on the channel
func (mc MessageCode) AuthorizedRolesByChannel(channel Channel) flow.RoleList {
	return mc.authorizedChannelsMap[channel]
}

var messageCodeMap codeToMessageCodes

// initializeMessageCodeMap initializes the messageCodeMap
func initializeMessageCodeMap() {
	messageCodeMap = make(codeToMessageCodes)
	// consensus
	messageCodeMap[codec.CodeBlockProposal] = MessageCode{
		Code: codec.CodeBlockProposal,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			ConsensusCommittee: {flow.RoleConsensus},
			PushBlocks:         {flow.RoleConsensus}, // channel alias ReceiveBlocks = PushBlocks
		},
	}

	messageCodeMap[codec.CodeBlockVote] = MessageCode{
		Code: codec.CodeBlockVote,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			ConsensusCommittee: {flow.RoleConsensus},
		},
	}

	// protocol state sync
	messageCodeMap[codec.CodeSyncRequest] = MessageCode{
		Code: codec.CodeSyncRequest,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			SyncCommittee:     flow.Roles(),
			SyncClusterPrefix: flow.Roles(),
		},
	}
	messageCodeMap[codec.CodeSyncResponse] = MessageCode{
		Code: codec.CodeSyncResponse,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			SyncCommittee:     flow.Roles(),
			SyncClusterPrefix: flow.Roles(),
		},
	}
	messageCodeMap[codec.CodeRangeRequest] = MessageCode{
		Code: codec.CodeRangeRequest,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			SyncCommittee:     flow.Roles(),
			SyncClusterPrefix: flow.Roles(),
		},
	}
	messageCodeMap[codec.CodeBatchRequest] = MessageCode{
		Code: codec.CodeBatchRequest,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			SyncCommittee:     flow.Roles(),
			SyncClusterPrefix: flow.Roles(),
		},
	}
	messageCodeMap[codec.CodeBlockResponse] = MessageCode{
		Code: codec.CodeBlockResponse,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			SyncCommittee:     flow.Roles(),
			SyncClusterPrefix: flow.Roles(),
		},
	}

	// cluster consensus
	messageCodeMap[codec.CodeClusterBlockProposal] = MessageCode{
		Code: codec.CodeClusterBlockProposal,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			ConsensusClusterPrefix: {flow.RoleCollection},
		},
	}
	messageCodeMap[codec.CodeClusterBlockVote] = MessageCode{
		Code: codec.CodeClusterBlockVote,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			ConsensusClusterPrefix: {flow.RoleCollection},
		},
	}
	messageCodeMap[codec.CodeClusterBlockResponse] = MessageCode{
		Code: codec.CodeClusterBlockResponse,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			ConsensusClusterPrefix: {flow.RoleCollection},
		},
	}

	// collections, guarantees & transactions
	messageCodeMap[codec.CodeCollectionGuarantee] = MessageCode{
		Code: codec.CodeCollectionGuarantee,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			PushGuarantees: {flow.RoleCollection}, // channel alias ReceiveGuarantees = PushGuarantees
		},
	}
	messageCodeMap[codec.CodeTransactionBody] = MessageCode{
		Code: codec.CodeTransactionBody,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			PushTransactions: {flow.RoleCollection}, // channel alias ReceiveTransactions = PushTransactions
		},
	}
	messageCodeMap[codec.CodeTransaction] = MessageCode{
		Code: codec.CodeTransaction,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			PushTransactions: {flow.RoleCollection}, // channel alias ReceiveTransactions = PushTransactions
		},
	}

	// core messages for execution & verification
	messageCodeMap[codec.CodeExecutionReceipt] = MessageCode{
		Code: codec.CodeExecutionReceipt,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			PushReceipts: {flow.RoleExecution}, // channel alias ReceiveReceipts = PushReceipts
		},
	}
	messageCodeMap[codec.CodeResultApproval] = MessageCode{
		Code: codec.CodeResultApproval,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			PushApprovals: {flow.RoleVerification}, // channel alias ReceiveApprovals = PushApprovals
		},
	}

	// execution state synchronization
	// NOTE: these messages have been deprecated
	messageCodeMap[codec.CodeExecutionStateSyncRequest] = MessageCode{
		Code:                  codec.CodeExecutionStateSyncRequest,
		authorizedChannelsMap: map[Channel]flow.RoleList{},
	}
	messageCodeMap[codec.CodeExecutionStateDelta] = MessageCode{
		Code:                  codec.CodeExecutionStateDelta,
		authorizedChannelsMap: map[Channel]flow.RoleList{},
	}

	// data exchange for execution of blocks
	messageCodeMap[codec.CodeChunkDataRequest] = MessageCode{
		Code: codec.CodeChunkDataRequest,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			ProvideChunks:            {flow.RoleVerification}, // channel alias RequestChunks = ProvideChunks
			RequestCollections:       {flow.RoleVerification},
			RequestApprovalsByChunk:  {flow.RoleVerification},
			RequestReceiptsByBlockID: {flow.RoleVerification},
		},
	}
	messageCodeMap[codec.CodeChunkDataResponse] = MessageCode{
		Code: codec.CodeChunkDataResponse,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			ProvideChunks:            {flow.RoleExecution}, // channel alias RequestChunks = ProvideChunks
			RequestCollections:       {flow.RoleExecution},
			RequestApprovalsByChunk:  {flow.RoleExecution},
			RequestReceiptsByBlockID: {flow.RoleExecution},
		},
	}

	// result approvals
	messageCodeMap[codec.CodeApprovalRequest] = MessageCode{
		Code: codec.CodeApprovalRequest,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			ProvideApprovalsByChunk: {flow.RoleConsensus},
		},
	}
	messageCodeMap[codec.CodeApprovalResponse] = MessageCode{
		Code: codec.CodeApprovalResponse,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			ProvideApprovalsByChunk: {flow.RoleVerification},
		},
	}

	// generic entity exchange engines
	messageCodeMap[codec.CodeEntityRequest] = MessageCode{
		Code: codec.CodeEntityRequest,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			RequestChunks:            {flow.RoleAccess, flow.RoleConsensus, flow.RoleCollection},
			RequestCollections:       {flow.RoleAccess, flow.RoleConsensus, flow.RoleCollection},
			RequestApprovalsByChunk:  {flow.RoleAccess, flow.RoleConsensus, flow.RoleCollection},
			RequestReceiptsByBlockID: {flow.RoleAccess, flow.RoleConsensus, flow.RoleCollection},
		},
	}

	messageCodeMap[codec.CodeEntityResponse] = MessageCode{
		Code: codec.CodeEntityResponse,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			RequestChunks:            {flow.RoleCollection, flow.RoleExecution},
			RequestCollections:       {flow.RoleCollection, flow.RoleExecution},
			RequestApprovalsByChunk:  {flow.RoleCollection, flow.RoleExecution},
			RequestReceiptsByBlockID: {flow.RoleCollection, flow.RoleExecution},
		},
	}

	// testing
	messageCodeMap[codec.CodeEcho] = MessageCode{
		Code: codec.CodeEcho,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			TestNetworkChannel: flow.Roles(),
			TestMetricsChannel: flow.Roles(),
		},
	}

	// dkg
	messageCodeMap[codec.CodeDKGMessage] = MessageCode{
		Code: codec.CodeDKGMessage,
		authorizedChannelsMap: map[Channel]flow.RoleList{
			DKGCommittee: {flow.RoleConsensus},
		},
	}
}

// MessageCodeFromV returns the correct Code based on the underlying type of message v
func MessageCodeFromV(v interface{}) (MessageCode, error) {
	var code codec.Code

	switch v.(type) {
	// consensus
	case *messages.BlockProposal:
		code = codec.CodeBlockProposal
	case *messages.BlockVote:
		code = codec.CodeBlockVote

	// protocol state sync
	case *messages.SyncRequest:
		code = codec.CodeSyncRequest
	case *messages.SyncResponse:
		code = codec.CodeSyncResponse
	case *messages.RangeRequest:
		code = codec.CodeRangeRequest
	case *messages.BatchRequest:
		code = codec.CodeBatchRequest
	case *messages.BlockResponse:
		code = codec.CodeBlockResponse

	// cluster consensus
	case *messages.ClusterBlockProposal:
		code = codec.CodeClusterBlockProposal
	case *messages.ClusterBlockVote:
		code = codec.CodeClusterBlockVote
	case *messages.ClusterBlockResponse:
		code = codec.CodeClusterBlockResponse

	// collections, guarantees & transactions
	case *flow.CollectionGuarantee:
		code = codec.CodeCollectionGuarantee
	case *flow.TransactionBody:
		code = codec.CodeTransactionBody
	case *flow.Transaction:
		code = codec.CodeTransaction

	// core messages for execution & verification
	case *flow.ExecutionReceipt:
		code = codec.CodeExecutionReceipt
	case *flow.ResultApproval:
		code = codec.CodeResultApproval

	// execution state synchronization
	case *messages.ExecutionStateSyncRequest:
		code = codec.CodeExecutionStateSyncRequest
	case *messages.ExecutionStateDelta:
		code = codec.CodeExecutionStateDelta

	// data exchange for execution of blocks
	case *messages.ChunkDataRequest:
		code = codec.CodeChunkDataRequest
	case *messages.ChunkDataResponse:
		code = codec.CodeChunkDataResponse

	// result approvals
	case *messages.ApprovalRequest:
		code = codec.CodeApprovalRequest
	case *messages.ApprovalResponse:
		code = codec.CodeApprovalResponse

	// generic entity exchange engines
	case *messages.EntityRequest:
		code = codec.CodeEntityRequest
	case *messages.EntityResponse:
		code = codec.CodeEntityResponse

	// testing
	case *message.TestMessage:
		code = codec.CodeEcho

	// dkg
	case *messages.DKGMessage:
		code = codec.CodeDKGMessage

	default:
		return MessageCode{}, fmt.Errorf("invalid encode type (%T)", v)
	}

	return messageCodeMap[code], nil
}

// MessageCodeFromByte helper func that performs a sanity check before returning a byte b as a Code
func MessageCodeFromByte(b byte) (MessageCode, error) {
	c := codec.Code(b)

	code, ok := messageCodeMap[c]
	if !ok {
		return MessageCode{}, fmt.Errorf("unknown message code: %d", c)
	}

	return code, nil
}

func init() {
	initializeMessageCodeMap()
}
