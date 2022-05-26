// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package json

import (
	"encoding/json"
	"fmt"

	"github.com/onflow/flow-go/network/codec"
	"github.com/pkg/errors"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/model/messages"
	_ "github.com/onflow/flow-go/utils/binstat"
)

func switchv2code(v interface{}) (uint8, error) {
	var code uint8

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

	// dkg
	case *messages.DKGMessage:
		what = "CodeDKGMessage"

	default:
		return "", errors.Errorf("invalid encode type (%T)", v)
	}

	return what, nil
}

func v2envEncode(v interface{}, via string) (*Envelope, error) {

	// determine the message type
	code, err := switchv2code(v)
	if nil != err {
		return nil, err
	}

	what, err := switchv2what(v)
	if nil != err {
		return nil, err
	}

	// encode the payload
	//bs := binstat.EnterTime(fmt.Sprintf("%s%s%s:%d", binstat.BinNet, via, what, code)) // e.g. ~3net::wire<1(json)CodeEntityRequest:23
	data, err := json.Marshal(v)
	//binstat.LeaveVal(bs, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("could not encode json payload of type %s: %w", what, err)
	}

	env := Envelope{
		Code: code,
		Data: data,
	}

	return &env, nil
}
