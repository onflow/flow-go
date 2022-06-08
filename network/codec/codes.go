// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package codec

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/model/messages"
)

const (
	CodeMin Code = iota + 1

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

type Code uint8

// String returns the string name of the message type the Code belongs to.
func (code Code) String() (string, error) {
	var s string

	switch code {

	// consensus
	case CodeBlockProposal:
		s = "CodeBlockProposal"
	case CodeBlockVote:
		s = "CodeBlockVote"

	// cluster consensus
	case CodeClusterBlockProposal:
		s = "CodeClusterBlockProposal"
	case CodeClusterBlockVote:
		s = "CodeClusterBlockVote"
	case CodeClusterBlockResponse:
		s = "CodeClusterBlockResponse"

	// protocol state sync
	case CodeSyncRequest:
		s = "CodeSyncRequest"
	case CodeSyncResponse:
		s = "CodeSyncResponse"
	case CodeRangeRequest:
		s = "CodeRangeRequest"
	case CodeBatchRequest:
		s = "CodeBatchRequest"
	case CodeBlockResponse:
		s = "CodeBlockResponse"

	// collections, guarantees & transactions
	case CodeCollectionGuarantee:
		s = "CodeCollectionGuarantee"
	case CodeTransactionBody:
		s = "CodeTransactionBody"
	case CodeTransaction:
		s = "CodeTransaction"

	// core messages for execution & verification
	case CodeExecutionReceipt:
		s = "CodeExecutionReceipt"
	case CodeResultApproval:
		s = "CodeResultApproval"

	// execution state synchronization
	case CodeExecutionStateSyncRequest:
		s = "CodeExecutionStateSyncRequest"
	case CodeExecutionStateDelta:
		s = "CodeExecutionStateDelta"

	// data exchange for execution of blocks
	case CodeChunkDataRequest:
		s = "CodeChunkDataRequest"
	case CodeChunkDataResponse:
		s = "CodeChunkDataResponse"

	case CodeApprovalRequest:
		s = "CodeApprovalRequest"
	case CodeApprovalResponse:
		s = "CodeApprovalResponse"

	// generic entity exchange engines
	case CodeEntityRequest:
		s = "CodeEntityRequest"
	case CodeEntityResponse:
		s = "CodeEntityResponse"

	// testing
	case CodeEcho:
		s = "CodeEcho"

	// dkg
	case CodeDKGMessage:
		s = "CodeDKGMessage"

	default:
		return "", fmt.Errorf("invalid message code (%d)", code)
	}

	return s, nil
}

// Interface returns an interface{} where it's underlying type is the flow message the Code belongs to.
func (code Code) Interface() (interface{}, error) {
	var v interface{}

	switch code {

	// consensus
	case CodeBlockProposal:
		v = &messages.BlockProposal{}
	case CodeBlockVote:
		v = &messages.BlockVote{}

	// cluster consensus
	case CodeClusterBlockProposal:
		v = &messages.ClusterBlockProposal{}
	case CodeClusterBlockVote:
		v = &messages.ClusterBlockVote{}
	case CodeClusterBlockResponse:
		v = &messages.ClusterBlockResponse{}

	// protocol state sync
	case CodeSyncRequest:
		v = &messages.SyncRequest{}
	case CodeSyncResponse:
		v = &messages.SyncResponse{}
	case CodeRangeRequest:
		v = &messages.RangeRequest{}
	case CodeBatchRequest:
		v = &messages.BatchRequest{}
	case CodeBlockResponse:
		v = &messages.BlockResponse{}

	// collections, guarantees & transactions
	case CodeCollectionGuarantee:
		v = &flow.CollectionGuarantee{}
	case CodeTransactionBody:
		v = &flow.TransactionBody{}
	case CodeTransaction:
		v = &flow.Transaction{}

	// core messages for execution & verification
	case CodeExecutionReceipt:
		v = &flow.ExecutionReceipt{}
	case CodeResultApproval:
		v = &flow.ResultApproval{}

	// execution state synchronization
	case CodeExecutionStateSyncRequest:
		v = &messages.ExecutionStateSyncRequest{}
	case CodeExecutionStateDelta:
		v = &messages.ExecutionStateDelta{}

	// data exchange for execution of blocks
	case CodeChunkDataRequest:
		v = &messages.ChunkDataRequest{}
	case CodeChunkDataResponse:
		v = &messages.ChunkDataResponse{}

	case CodeApprovalRequest:
		v = &messages.ApprovalRequest{}
	case CodeApprovalResponse:
		v = &messages.ApprovalResponse{}

	// generic entity exchange engines
	case CodeEntityRequest:
		v = &messages.EntityRequest{}
	case CodeEntityResponse:
		v = &messages.EntityResponse{}

	// testing
	case CodeEcho:
		v = &message.TestMessage{}

	// dkg
	case CodeDKGMessage:
		v = &messages.DKGMessage{}

	default:
		return nil, fmt.Errorf("invalid message code (%d)", code)
	}

	return v, nil
}

// Byte is a helper func that returns the Code as a byte type.
func (code Code) Byte() byte {
	return byte(code)
}

// Message is wrapper that calls both String and Interface
func (code Code) Message() (string, interface{}, error) {
	what, err := code.String()
	if err != nil {
		return "", nil, err
	}

	v, err := code.Interface()
	if err != nil {
		return "", nil, err
	}

	return what, v, nil
}
