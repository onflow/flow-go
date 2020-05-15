// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package json

import (
	"encoding/json"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/model/coldstuff"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/libp2p/message"
	"github.com/dapperlabs/flow-go/model/messages"
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

	// coldstuff specific
	case CodeBlockCommit:
		v = &coldstuff.Commit{}

	// cluster consensus
	case CodeClusterBlockProposal:
		v = &messages.ClusterBlockProposal{}
	case CodeClusterBlockVote:
		v = &messages.ClusterBlockVote{}
	case CodeClusterBlockRequest:
		v = &messages.ClusterBlockRequest{}
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

	case CodeCollectionGuarantee:
		v = &flow.CollectionGuarantee{}
	case CodeTransactionBody:
		v = &flow.TransactionBody{}
	case CodeTransaction:
		v = &flow.Transaction{}

	case CodeCollectionRequest:
		v = &messages.CollectionRequest{}
	case CodeCollectionResponse:
		v = &messages.CollectionResponse{}

	case CodeEcho:
		v = &message.Echo{}

	case CodeExecutionReceipt:
		v = &flow.ExecutionReceipt{}
	//case CodeExecutionStateRequest:
	//	v = &messages.ExecutionStateRequest{}
	//case CodeExecutionStateResponse:
	//	v = &messages.ExecutionStateResponse{}
	case CodeExecutionStateSyncRequest:
		v = &messages.ExecutionStateSyncRequest{}
	case CodeExecutionStateDelta:
		v = &messages.ExecutionStateDelta{}

	case CodeChunkDataPackRequest:
		v = &messages.ChunkDataPackRequest{}
	case CodeChunkDataPackResponse:
		v = &messages.ChunkDataPackResponse{}

	case CodeResultApproval:
		v = &flow.ResultApproval{}

	default:
		return nil, errors.Errorf("invalid message code (%d)", env.Code)
	}

	// unmarshal the payload
	err := json.Unmarshal(env.Data, v)
	if err != nil {
		return nil, errors.Wrap(err, "could not decode payload")
	}

	return v, nil
}
