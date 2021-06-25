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

func switchv2code(v interface{}) (uint8, error) {
	var code uint8

	switch v.(type) {

	// consensus
	case *messages.BlockProposal:
		code = CodeBlockProposal
	case *messages.BlockVote:
		code = CodeBlockVote

	// protocol state sync
	case *messages.SyncRequest:
		code = CodeSyncRequest
	case *messages.SyncResponse:
		code = CodeSyncResponse
	case *messages.RangeRequest:
		code = CodeRangeRequest
	case *messages.BatchRequest:
		code = CodeBatchRequest
	case *messages.BlockResponse:
		code = CodeBlockResponse

	// cluster consensus
	case *messages.ClusterBlockProposal:
		code = CodeClusterBlockProposal
	case *messages.ClusterBlockVote:
		code = CodeClusterBlockVote
	case *messages.ClusterBlockResponse:
		code = CodeClusterBlockResponse

	// collections, guarantees & transactions
	case *flow.CollectionGuarantee:
		code = CodeCollectionGuarantee
	case *flow.TransactionBody:
		code = CodeTransactionBody
	case *flow.Transaction:
		code = CodeTransaction

	// core messages for execution & verification
	case *flow.ExecutionReceipt:
		code = CodeExecutionReceipt
	case *flow.ResultApproval:
		code = CodeResultApproval

	// execution state synchronization
	case *messages.ExecutionStateSyncRequest:
		code = CodeExecutionStateSyncRequest
	case *messages.ExecutionStateDelta:
		code = CodeExecutionStateDelta

	// data exchange for execution of blocks
	case *messages.ChunkDataRequest:
		code = CodeChunkDataRequest
	case *messages.ChunkDataResponse:
		code = CodeChunkDataResponse

	// result approvals
	case *messages.ApprovalRequest:
		code = CodeApprovalRequest
	case *messages.ApprovalResponse:
		code = CodeApprovalResponse

	// generic entity exchange engines
	case *messages.EntityRequest:
		code = CodeEntityRequest
	case *messages.EntityResponse:
		code = CodeEntityResponse

	// testing
	case *message.TestMessage:
		code = CodeEcho

	default:
		return 0, errors.Errorf("invalid encode type (%T)", v)
	}

	return code, nil
}

func switchv2what(v interface{}) (string, error) {
	var what string

	switch v.(type) {

	// consensus
	case *messages.BlockProposal:
		what = "~4CodeBlockProposal"
	case *messages.BlockVote:
		what = "~4CodeBlockVote"

	// protocol state sync
	case *messages.SyncRequest:
		what = "~4CodeSyncRequest"
	case *messages.SyncResponse:
		what = "~4CodeSyncResponse"
	case *messages.RangeRequest:
		what = "~4CodeRangeRequest"
	case *messages.BatchRequest:
		what = "~4CodeBatchRequest"
	case *messages.BlockResponse:
		what = "~4CodeBatchRequest"

	// cluster consensus
	case *messages.ClusterBlockProposal:
		what = "~4CodeClusterBlockProposal"
	case *messages.ClusterBlockVote:
		what = "~4CodeClusterBlockVote"
	case *messages.ClusterBlockResponse:
		what = "~4CodeClusterBlockResponse"

	// collections, guarantees & transactions
	case *flow.CollectionGuarantee:
		what = "~4CodeCollectionGuarantee"
	case *flow.TransactionBody:
		what = "~4CodeTransactionBody"
	case *flow.Transaction:
		what = "~4CodeTransaction"

	// core messages for execution & verification
	case *flow.ExecutionReceipt:
		what = "~4CodeExecutionReceipt"
	case *flow.ResultApproval:
		what = "~4CodeResultApproval"

	// execution state synchronization
	case *messages.ExecutionStateSyncRequest:
		what = "~4CodeExecutionStateSyncRequest"
	case *messages.ExecutionStateDelta:
		what = "~4CodeExecutionStateDelta"

	// data exchange for execution of blocks
	case *messages.ChunkDataRequest:
		what = "~4CodeChunkDataRequest"
	case *messages.ChunkDataResponse:
		what = "~4CodeChunkDataResponse"

	// result approvals
	case *messages.ApprovalRequest:
		what = "~4CodeApprovalRequest"
	case *messages.ApprovalResponse:
		what = "~4CodeApprovalResponse"

	// generic entity exchange engines
	case *messages.EntityRequest:
		what = "~4CodeEntityRequest"
	case *messages.EntityResponse:
		what = "~4CodeEntityResponse"

	// testing
	case *message.TestMessage:
		what = "~4CodeEcho"

	default:
		return "", errors.Errorf("invalid encode type (%T)", v)
	}

	return what, nil
}

func encode(v interface{}) (*Envelope, error) {

	// determine the message type
	code, err1 := switchv2code(v)
	what, err2 := switchv2what(v)

	if nil != err1 {
		return nil, err1
	}

	if nil != err2 {
		return nil, err2
	}

	// encode the payload
	p := binstat.EnterTime(fmt.Sprintf("%s:%d", what, code), "")
	data, err := json.Marshal(v)
	binstat.LeaveVal(p, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("could not encode payload: %w", err)
	}

	env := Envelope{
		Code: code,
		Data: data,
	}

	return &env, nil
}
