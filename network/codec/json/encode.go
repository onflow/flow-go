// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package json

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	"github.com/onflow/flow-go/binstat"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/model/messages"
)

func encode(v interface{}) (*Envelope, error) {

	// determine the message type
	var code uint8
	var what string
	switch v.(type) {

	// consensus
	case *messages.BlockProposal:
		what = "~4CodeBlockProposal"
		code = CodeBlockProposal
	case *messages.BlockVote:
		what = "~4CodeBlockVote"
		code = CodeBlockVote

	// protocol state sync
	case *messages.SyncRequest:
		what = "~4CodeSyncRequest"
		code = CodeSyncRequest
	case *messages.SyncResponse:
		what = "~4CodeSyncResponse"
		code = CodeSyncResponse
	case *messages.RangeRequest:
		what = "~4CodeRangeRequest"
		code = CodeRangeRequest
	case *messages.BatchRequest:
		what = "~4CodeBatchRequest"
		code = CodeBatchRequest
	case *messages.BlockResponse:
		what = "~4CodeBatchRequest"
		code = CodeBlockResponse

	// cluster consensus
	case *messages.ClusterBlockProposal:
		what = "~4CodeClusterBlockProposal"
		code = CodeClusterBlockProposal
	case *messages.ClusterBlockVote:
		what = "~4CodeClusterBlockVote"
		code = CodeClusterBlockVote
	case *messages.ClusterBlockResponse:
		what = "~4CodeClusterBlockResponse"
		code = CodeClusterBlockResponse

	// collections, guarantees & transactions
	case *flow.CollectionGuarantee:
		what = "~4CodeCollectionGuarantee"
		code = CodeCollectionGuarantee
	case *flow.TransactionBody:
		what = "~4CodeTransactionBody"
		code = CodeTransactionBody
	case *flow.Transaction:
		what = "~4CodeTransaction"
		code = CodeTransaction

	// core messages for execution & verification
	case *flow.ExecutionReceipt:
		what = "~4CodeExecutionReceipt"
		code = CodeExecutionReceipt
	case *flow.ResultApproval:
		what = "~4CodeResultApproval"
		code = CodeResultApproval

	// execution state synchronization
	case *messages.ExecutionStateSyncRequest:
		what = "~4CodeExecutionStateSyncRequest"
		code = CodeExecutionStateSyncRequest
	case *messages.ExecutionStateDelta:
		what = "~4CodeExecutionStateDelta"
		code = CodeExecutionStateDelta

	// data exchange for execution of blocks
	case *messages.ChunkDataRequest:
		what = "~4CodeChunkDataRequest"
		code = CodeChunkDataRequest
	case *messages.ChunkDataResponse:
		what = "~4CodeChunkDataResponse"
		code = CodeChunkDataResponse

	// result approvals
	case *messages.ApprovalRequest:
		what = "~4CodeApprovalRequest"
		code = CodeApprovalRequest
	case *messages.ApprovalResponse:
		what = "~4CodeApprovalResponse"
		code = CodeApprovalResponse

	// generic entity exchange engines
	case *messages.EntityRequest:
		what = "~4CodeEntityRequest"
		code = CodeEntityRequest
	case *messages.EntityResponse:
		what = "~4CodeEntityResponse"
		code = CodeEntityResponse

	// testing
	case *message.TestMessage:
		what = "~4CodeEcho"
		code = CodeEcho

	default:
		return nil, errors.Errorf("invalid encode type (%T)", v)
	}

	// encode the payload
	p := binstat.NewTime(fmt.Sprintf("%s:%d", what, code), "")
	data, err := json.Marshal(v)
	binstat.EndVal(p, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("could not encode payload: %w", err)
	}

	env := Envelope{
		Code: code,
		Data: data,
	}

	return &env, nil
}
