// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package json

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/model/messages"
)

// decode will decode the envelope into an entity.
func decode(env Envelope) (interface{}, error) {

	// create the desired message
	var v interface{}
	switch env.Code {

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

	case CodeDKGMessage:
		v = &messages.DKGMessage{}

	default:
		return nil, errors.Errorf("invalid message code (%d)", env.Code)
	}

	// unmarshal the payload
	err := json.Unmarshal(env.Data, v)
	if err != nil {
		return nil, fmt.Errorf("could not decode payload: %w", err)
	}

	return v, nil
}
