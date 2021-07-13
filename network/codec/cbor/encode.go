// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package cbor

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"
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
		what = "CodeBlockProposal"
	case *messages.BlockVote:
		what = "CodeBlockVote"

	// protocol state sync
	case *messages.SyncRequest:
		what = "CodeSyncRequest"
	case *messages.SyncResponse:
		what = "CodeSyncResponse"
	case *messages.RangeRequest:
		what = "CodeRangeRequest"
	case *messages.BatchRequest:
		what = "CodeBatchRequest"
	case *messages.BlockResponse:
		what = "CodeBatchRequest"

	// cluster consensus
	case *messages.ClusterBlockProposal:
		what = "CodeClusterBlockProposal"
	case *messages.ClusterBlockVote:
		what = "CodeClusterBlockVote"
	case *messages.ClusterBlockResponse:
		what = "CodeClusterBlockResponse"

	// collections, guarantees & transactions
	case *flow.CollectionGuarantee:
		what = "CodeCollectionGuarantee"
	case *flow.TransactionBody:
		what = "CodeTransactionBody"
	case *flow.Transaction:
		what = "CodeTransaction"

	// core messages for execution & verification
	case *flow.ExecutionReceipt:
		what = "CodeExecutionReceipt"
	case *flow.ResultApproval:
		what = "CodeResultApproval"

	// execution state synchronization
	case *messages.ExecutionStateSyncRequest:
		what = "CodeExecutionStateSyncRequest"
	case *messages.ExecutionStateDelta:
		what = "CodeExecutionStateDelta"

	// data exchange for execution of blocks
	case *messages.ChunkDataRequest:
		what = "CodeChunkDataRequest"
	case *messages.ChunkDataResponse:
		what = "CodeChunkDataResponse"

	// result approvals
	case *messages.ApprovalRequest:
		what = "CodeApprovalRequest"
	case *messages.ApprovalResponse:
		what = "CodeApprovalResponse"

	// generic entity exchange engines
	case *messages.EntityRequest:
		what = "CodeEntityRequest"
	case *messages.EntityResponse:
		what = "CodeEntityResponse"

	// testing
	case *message.TestMessage:
		what = "CodeEcho"

	default:
		return "", errors.Errorf("invalid encode type (%T)", v)
	}

	return what, nil
}

func v2envEncode(v interface{}, via string) ([]byte, uint8, error) {

	// determine the message type
	code, err1 := switchv2code(v)
	what, err2 := switchv2what(v)

	if nil != err1 {
		return nil, 0, err1
	}

	if nil != err2 {
		return nil, 0, err2
	}

	// encode the payload
	p := binstat.EnterTime(fmt.Sprintf("%s%s:%d", via, what, code), "")
	opts := cbor.CoreDetEncOptions() // CBOR deterministic options
	em, err3 := opts.EncMode()
	if err3 != nil {
		return nil, 0, err3
	}
	data, err := em.Marshal(v)
	binstat.LeaveVal(p, int64(len(data)))
	if err != nil {
		return nil, 0, fmt.Errorf("could not encode payload: %w", err)
	}

	return data, code, nil
}
