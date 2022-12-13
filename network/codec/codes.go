// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package codec

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/model/messages"
)

const (
	CodeMin uint8 = iota + 1

	// consensus
	CodeBlockProposal
	CodeBlockVote

	// protocol state sync
	CodeSyncRequest
	CodeSyncResponse
	CodeRangeRequest
	CodeBatchRequest
	CodeBlockResponse

	// cluster consensus
	CodeClusterBlockProposal
	CodeClusterBlockVote
	CodeClusterBlockResponse

	// collections, guarantees & transactions
	CodeCollectionGuarantee
	CodeTransaction
	CodeTransactionBody

	// core messages for execution & verification
	CodeExecutionReceipt
	CodeResultApproval

	// execution state synchronization
	CodeExecutionStateSyncRequest
	CodeExecutionStateDelta

	// data exchange for execution of blocks
	CodeChunkDataRequest
	CodeChunkDataResponse

	// result approvals
	CodeApprovalRequest
	CodeApprovalResponse

	// generic entity exchange engines
	CodeEntityRequest
	CodeEntityResponse

	// testing
	CodeEcho

	// DKG
	CodeDKGMessage

	CodeMax
)

// MessageCodeFromInterface returns the correct Code based on the underlying type of message v.
func MessageCodeFromInterface(v interface{}) (uint8, string, error) {
	switch v.(type) {
	// consensus
	case *messages.BlockProposal:
		return CodeBlockProposal, "CodeBlockProposal", nil
	case *messages.BlockVote:
		return CodeBlockVote, "CodeBlockVote", nil

	// cluster consensus
	case *messages.ClusterBlockProposal:
		return CodeClusterBlockProposal, "CodeClusterBlockProposal", nil
	case *messages.ClusterBlockVote:
		return CodeClusterBlockVote, "CodeClusterBlockVote", nil
	case *messages.ClusterBlockResponse:
		return CodeClusterBlockResponse, "CodeClusterBlockResponse", nil

	// protocol state sync
	case *messages.SyncRequest:
		return CodeSyncRequest, "CodeSyncRequest", nil
	case *messages.SyncResponse:
		return CodeSyncResponse, "CodeSyncResponse", nil
	case *messages.RangeRequest:
		return CodeRangeRequest, "CodeRangeRequest", nil
	case *messages.BatchRequest:
		return CodeBatchRequest, "CodeBatchRequest", nil
	case *messages.BlockResponse:
		return CodeBlockResponse, "CodeBlockResponse", nil

	// collections, guarantees & transactions
	case *flow.CollectionGuarantee:
		return CodeCollectionGuarantee, "CodeCollectionGuarantee", nil
	case *flow.TransactionBody:
		return CodeTransactionBody, "CodeTransactionBody", nil
	case *flow.Transaction:
		return CodeTransaction, "CodeTransaction", nil

	// core messages for execution & verification
	case *flow.ExecutionReceipt:
		return CodeExecutionReceipt, "CodeExecutionReceipt", nil
	case *flow.ResultApproval:
		return CodeResultApproval, "CodeResultApproval", nil

	// execution state synchronization
	case *messages.ExecutionStateSyncRequest:
		return CodeExecutionStateSyncRequest, "CodeExecutionStateSyncRequest", nil
	case *messages.ExecutionStateDelta:
		return CodeExecutionStateDelta, "CodeExecutionStateDelta", nil

	// data exchange for execution of blocks
	case *messages.ChunkDataRequest:
		return CodeChunkDataRequest, "CodeChunkDataRequest", nil
	case *messages.ChunkDataResponse:
		return CodeChunkDataResponse, "CodeChunkDataResponse", nil

	// result approvals
	case *messages.ApprovalRequest:
		return CodeApprovalRequest, "CodeApprovalRequest", nil
	case *messages.ApprovalResponse:
		return CodeApprovalResponse, "CodeApprovalResponse", nil

	// generic entity exchange engines
	case *messages.EntityRequest:
		return CodeEntityRequest, "CodeEntityRequest", nil
	case *messages.EntityResponse:
		return CodeEntityResponse, "CodeEntityResponse", nil

	// testing
	case *message.TestMessage:
		return CodeEcho, "CodeEcho", nil

	// dkg
	case *messages.DKGMessage:
		return CodeDKGMessage, "CodeDKGMessage", nil

	default:
		return 0, "", fmt.Errorf("invalid encode type (%T)", v)
	}
}

// InterfaceFromMessageCode returns an interface with the correct underlying go type
// of the message code represents.
func InterfaceFromMessageCode(code uint8) (interface{}, string, error) {
	switch code {
	// consensus
	case CodeBlockProposal:
		return &messages.BlockProposal{}, "BlockProposal", nil
	case CodeBlockVote:
		return &messages.BlockVote{}, "BlockVote", nil

	// cluster consensus
	case CodeClusterBlockProposal:
		return &messages.ClusterBlockProposal{}, "ClusterBlockProposal", nil
	case CodeClusterBlockVote:
		return &messages.ClusterBlockVote{}, "ClusterBlockVote", nil
	case CodeClusterBlockResponse:
		return &messages.ClusterBlockResponse{}, "ClusterBlockResponse", nil

	// protocol state sync
	case CodeSyncRequest:
		return &messages.SyncRequest{}, "SyncRequest", nil
	case CodeSyncResponse:
		return &messages.SyncResponse{}, "SyncResponse", nil
	case CodeRangeRequest:
		return &messages.RangeRequest{}, "RangeRequest", nil
	case CodeBatchRequest:
		return &messages.BatchRequest{}, "BatchRequest", nil
	case CodeBlockResponse:
		return &messages.BlockResponse{}, "BlockResponse", nil

	// collections, guarantees & transactions
	case CodeCollectionGuarantee:
		return &flow.CollectionGuarantee{}, "CollectionGuarantee", nil
	case CodeTransactionBody:
		return &flow.TransactionBody{}, "TransactionBody", nil
	case CodeTransaction:
		return &flow.Transaction{}, "Transaction", nil

	// core messages for execution & verification
	case CodeExecutionReceipt:
		return &flow.ExecutionReceipt{}, "ExecutionReceipt", nil
	case CodeResultApproval:
		return &flow.ResultApproval{}, "ResultApproval", nil

	// execution state synchronization
	case CodeExecutionStateSyncRequest:
		return &messages.ExecutionStateSyncRequest{}, "ExecutionStateSyncRequest", nil
	case CodeExecutionStateDelta:
		return &messages.ExecutionStateDelta{}, "ExecutionStateDelta", nil

	// data exchange for execution of blocks
	case CodeChunkDataRequest:
		return &messages.ChunkDataRequest{}, "ChunkDataRequest", nil
	case CodeChunkDataResponse:
		return &messages.ChunkDataResponse{}, "ChunkDataResponse", nil

	// result approvals
	case CodeApprovalRequest:
		return &messages.ApprovalRequest{}, "ApprovalRequest", nil
	case CodeApprovalResponse:
		return &messages.ApprovalResponse{}, "ApprovalResponse", nil

	// generic entity exchange engines
	case CodeEntityRequest:
		return &messages.EntityRequest{}, "EntityRequest", nil
	case CodeEntityResponse:
		return &messages.EntityResponse{}, "EntityResponse", nil

	// dkg
	case CodeDKGMessage:
		return &messages.DKGMessage{}, "DKGMessage", nil

	// test messages
	case CodeEcho:
		return &message.TestMessage{}, "TestMessage", nil

	default:
		return nil, "", fmt.Errorf("invalid message code (%d)", code)
	}
}
