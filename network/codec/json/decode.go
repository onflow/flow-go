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

// decode will decode the envelope into an entity.
func decode(env Envelope) (interface{}, error) {

	// create the desired message
	var v interface{}
	var what string
	switch env.Code {

	// consensus
	case CodeBlockProposal:
		what = "~4CodeBlockProposal"
		v = &messages.BlockProposal{}
	case CodeBlockVote:
		what = "~4CodeBlockVote"
		v = &messages.BlockVote{}

	// cluster consensus
	case CodeClusterBlockProposal:
		what = "~4CodeClusterBlockProposal"
		v = &messages.ClusterBlockProposal{}
	case CodeClusterBlockVote:
		what = "~4CodeClusterBlockVote"
		v = &messages.ClusterBlockVote{}
	case CodeClusterBlockResponse:
		what = "~4CodeClusterBlockResponse"
		v = &messages.ClusterBlockResponse{}

	// protocol state sync
	case CodeSyncRequest:
		what = "~4CodeSyncRequest"
		v = &messages.SyncRequest{}
	case CodeSyncResponse:
		what = "~4CodeSyncResponse"
		v = &messages.SyncResponse{}
	case CodeRangeRequest:
		what = "~4CodeRangeRequest"
		v = &messages.RangeRequest{}
	case CodeBatchRequest:
		what = "~4CodeBatchRequest"
		v = &messages.BatchRequest{}
	case CodeBlockResponse:
		what = "~4CodeBlockResponse"
		v = &messages.BlockResponse{}

	// collections, guarantees & transactions
	case CodeCollectionGuarantee:
		what = "~4CodeCollectionGuarantee"
		v = &flow.CollectionGuarantee{}
	case CodeTransactionBody:
		what = "~4CodeTransactionBody"
		v = &flow.TransactionBody{}
	case CodeTransaction:
		what = "~4CodeTransaction"
		v = &flow.Transaction{}

	// core messages for execution & verification
	case CodeExecutionReceipt:
		what = "~4CodeExecutionReceipt"
		v = &flow.ExecutionReceipt{}
	case CodeResultApproval:
		what = "~4CodeResultApproval"
		v = &flow.ResultApproval{}

	// execution state synchronization
	case CodeExecutionStateSyncRequest:
		what = "~4CodeExecutionStateSyncRequest"
		v = &messages.ExecutionStateSyncRequest{}
	case CodeExecutionStateDelta:
		what = "~4CodeExecutionStateDelta"
		v = &messages.ExecutionStateDelta{}

	// data exchange for execution of blocks
	case CodeChunkDataRequest:
		what = "~4CodeChunkDataRequest"
		v = &messages.ChunkDataRequest{}
	case CodeChunkDataResponse:
		what = "~4CodeChunkDataResponse"
		v = &messages.ChunkDataResponse{}

	case CodeApprovalRequest:
		what = "~4CodeApprovalRequest"
		v = &messages.ApprovalRequest{}
	case CodeApprovalResponse:
		what = "~4CodeApprovalResponse"
		v = &messages.ApprovalResponse{}

	// generic entity exchange engines
	case CodeEntityRequest:
		what = "~4CodeEntityRequest"
		v = &messages.EntityRequest{}
	case CodeEntityResponse:
		what = "~4CodeEntityResponse"
		v = &messages.EntityResponse{}

	// testing
	case CodeEcho:
		what = "~4CodeEcho"
		v = &message.TestMessage{}

	default:
		return nil, errors.Errorf("invalid message code (%d)", env.Code)
	}

	// unmarshal the payload
	p := binstat.NewTimeVal(fmt.Sprintf("%s:%d", what, env.Code), "", int64(len(env.Data)))
	err := json.Unmarshal(env.Data, v)
	binstat.End(p)
	if err != nil {
		return nil, fmt.Errorf("could not decode payload: %w", err)
	}

	return v, nil
}
