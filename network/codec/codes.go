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
		return CodeBlockProposal, "messages.CodeBlockProposal", nil
	case *messages.BlockVote:
		return CodeBlockVote, "messages.CodeBlockVote", nil

	// cluster consensus
	case *messages.ClusterBlockProposal:
		return CodeClusterBlockProposal, "messages.CodeClusterBlockProposal", nil
	case *messages.ClusterBlockVote:
		return CodeClusterBlockVote, "messages.CodeClusterBlockVote", nil
	case *messages.ClusterBlockResponse:
		return CodeClusterBlockResponse, "messages.CodeClusterBlockResponse", nil

	// protocol state sync
	case *messages.SyncRequest:
		return CodeSyncRequest, "messages.CodeSyncRequest", nil
	case *messages.SyncResponse:
		return CodeSyncResponse, "messages.CodeSyncResponse", nil
	case *messages.RangeRequest:
		return CodeRangeRequest, "messages.CodeRangeRequest", nil
	case *messages.BatchRequest:
		return CodeBatchRequest, "messages.CodeBatchRequest", nil
	case *messages.BlockResponse:
		return CodeBlockResponse, "messages.CodeBlockResponse", nil

	// collections, guarantees & transactions
	case *flow.CollectionGuarantee:
		return CodeCollectionGuarantee, "flow.CodeCollectionGuarantee", nil
	case *flow.TransactionBody:
		return CodeTransactionBody, "flow.CodeTransactionBody", nil
	case *flow.Transaction:
		return CodeTransaction, "flow.CodeTransaction", nil

	// core messages for execution & verification
	case *flow.ExecutionReceipt:
		return CodeExecutionReceipt, "flow.CodeExecutionReceipt", nil
	case *flow.ResultApproval:
		return CodeResultApproval, "flow.CodeResultApproval", nil

	// execution state synchronization
	case *messages.ExecutionStateSyncRequest:
		return CodeExecutionStateSyncRequest, "messages.CodeExecutionStateSyncRequest", nil
	case *messages.ExecutionStateDelta:
		return CodeExecutionStateDelta, "messages.CodeExecutionStateDelta", nil

	// data exchange for execution of blocks
	case *messages.ChunkDataRequest:
		return CodeChunkDataRequest, "messages.CodeChunkDataRequest", nil
	case *messages.ChunkDataResponse:
		return CodeChunkDataResponse, "messages.CodeChunkDataResponse", nil

	// result approvals
	case *messages.ApprovalRequest:
		return CodeApprovalRequest, "messages.CodeApprovalRequest", nil
	case *messages.ApprovalResponse:
		return CodeApprovalResponse, "messages.CodeApprovalResponse", nil

	// generic entity exchange engines
	case *messages.EntityRequest:
		return CodeEntityRequest, "messages.CodeEntityRequest", nil
	case *messages.EntityResponse:
		return CodeEntityResponse, "messages.CodeEntityResponse", nil

	// testing
	case *message.TestMessage:
		return CodeEcho, "message.CodeEcho", nil

	// dkg
	case *messages.DKGMessage:
		return CodeDKGMessage, "messages.CodeDKGMessage", nil

	default:
		return 0, "", fmt.Errorf("invalid encode type (%T)", v)
	}
}

// InterfaceFromMessageCode returns an interface with the correct underlying go type
// of the message code represents.
// Expected error returns during normal operations:
//   - ErrUnknownMsgCode if message code does not match any of the configured message codes above.
func InterfaceFromMessageCode(code uint8) (interface{}, string, error) {
	switch code {
	// consensus
	case CodeBlockProposal:
		return &messages.BlockProposal{}, "messages.BlockProposal", nil
	case CodeBlockVote:
		return &messages.BlockVote{}, "messages.BlockVote", nil

	// cluster consensus
	case CodeClusterBlockProposal:
		return &messages.ClusterBlockProposal{}, "messages.ClusterBlockProposal", nil
	case CodeClusterBlockVote:
		return &messages.ClusterBlockVote{}, "messages.ClusterBlockVote", nil
	case CodeClusterBlockResponse:
		return &messages.ClusterBlockResponse{}, "messages.ClusterBlockResponse", nil

	// protocol state sync
	case CodeSyncRequest:
		return &messages.SyncRequest{}, "messages.SyncRequest", nil
	case CodeSyncResponse:
		return &messages.SyncResponse{}, "messages.SyncResponse", nil
	case CodeRangeRequest:
		return &messages.RangeRequest{}, "messages.RangeRequest", nil
	case CodeBatchRequest:
		return &messages.BatchRequest{}, "messages.BatchRequest", nil
	case CodeBlockResponse:
		return &messages.BlockResponse{}, "messages.BlockResponse", nil

	// collections, guarantees & transactions
	case CodeCollectionGuarantee:
		return &flow.CollectionGuarantee{}, "flow.CollectionGuarantee", nil
	case CodeTransactionBody:
		return &flow.TransactionBody{}, "flow.TransactionBody", nil
	case CodeTransaction:
		return &flow.Transaction{}, "flow.Transaction", nil

	// core messages for execution & verification
	case CodeExecutionReceipt:
		return &flow.ExecutionReceipt{}, "flow.ExecutionReceipt", nil
	case CodeResultApproval:
		return &flow.ResultApproval{}, "flow.ResultApproval", nil

	// execution state synchronization
	case CodeExecutionStateSyncRequest:
		return &messages.ExecutionStateSyncRequest{}, "messages.ExecutionStateSyncRequest", nil
	case CodeExecutionStateDelta:
		return &messages.ExecutionStateDelta{}, "messages.ExecutionStateDelta", nil

	// data exchange for execution of blocks
	case CodeChunkDataRequest:
		return &messages.ChunkDataRequest{}, "messages.ChunkDataRequest", nil
	case CodeChunkDataResponse:
		return &messages.ChunkDataResponse{}, "messages.ChunkDataResponse", nil

	// result approvals
	case CodeApprovalRequest:
		return &messages.ApprovalRequest{}, "messages.ApprovalRequest", nil
	case CodeApprovalResponse:
		return &messages.ApprovalResponse{}, "messages.ApprovalResponse", nil

	// generic entity exchange engines
	case CodeEntityRequest:
		return &messages.EntityRequest{}, "messages.EntityRequest", nil
	case CodeEntityResponse:
		return &messages.EntityResponse{}, "messages.EntityResponse", nil

	// dkg
	case CodeDKGMessage:
		return &messages.DKGMessage{}, "messages.DKGMessage", nil

	// test messages
	case CodeEcho:
		return &message.TestMessage{}, "message.TestMessage", nil

	default:
		return nil, "", NewUnknownMsgCodeErr(code)
	}
}
