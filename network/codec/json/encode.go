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

func encode(v interface{}) (*Envelope, error) {

	// determine the message type
	var code uint8
	switch v.(type) {

	// consensus
	case *messages.BlockProposal:
		code = CodeBlockProposal
	case *messages.BlockVote:
		code = CodeBlockVote

	// coldstuff-specific
	case *coldstuff.Commit:
		code = CodeBlockCommit

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
	case *messages.ClusterBlockRequest:
		code = CodeClusterBlockRequest
	case *messages.ClusterBlockResponse:
		code = CodeClusterBlockResponse

	case *flow.CollectionGuarantee:
		code = CodeCollectionGuarantee
	case *flow.TransactionBody:
		code = CodeTransactionBody
	case *flow.Transaction:
		code = CodeTransaction

	case *messages.CollectionRequest:
		code = CodeCollectionRequest
	case *messages.CollectionResponse:
		code = CodeCollectionResponse

	case *message.Echo:
		code = CodeEcho

	case *flow.ExecutionReceipt:
		code = CodeExecutionReceipt
	case *messages.ChunkDataPackRequest:
		code = CodeChunkDataPackRequest
	case *messages.ChunkDataPackResponse:
		code = CodeChunkDataPackResponse
	case *messages.ExecutionStateSyncRequest:
		code = CodeExecutionStateSyncRequest
	case *messages.ExecutionStateDelta:
		code = CodeExecutionStateDelta
	case *flow.ResultApproval:
		code = CodeResultApproval

	default:
		return nil, errors.Errorf("invalid encode type (%T)", v)
	}

	// encode the payload
	data, err := json.Marshal(v)
	if err != nil {
		return nil, errors.Wrap(err, "could not encode payload")
	}

	env := Envelope{
		Code: code,
		Data: data,
	}

	return &env, nil
}
