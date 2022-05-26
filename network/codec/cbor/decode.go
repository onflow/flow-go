// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package cbor

import (
	"github.com/onflow/flow-go/network/codec"
	"github.com/pkg/errors"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/model/messages"
)

func switchenv2v(code uint8) (interface{}, error) {
	var v interface{}

	switch code {

	// consensus
	case codec.CodeBlockProposal:
		v = &messages.BlockProposal{}
	case codec.CodeBlockVote:
		v = &messages.BlockVote{}

	// cluster consensus
	case codec.CodeClusterBlockProposal:
		v = &messages.ClusterBlockProposal{}
	case codec.CodeClusterBlockVote:
		v = &messages.ClusterBlockVote{}
	case codec.CodeClusterBlockResponse:
		v = &messages.ClusterBlockResponse{}

	// protocol state sync
	case codec.CodeSyncRequest:
		v = &messages.SyncRequest{}
	case codec.CodeSyncResponse:
		v = &messages.SyncResponse{}
	case codec.CodeRangeRequest:
		v = &messages.RangeRequest{}
	case codec.CodeBatchRequest:
		v = &messages.BatchRequest{}
	case codec.CodeBlockResponse:
		v = &messages.BlockResponse{}

	// collections, guarantees & transactions
	case codec.CodeCollectionGuarantee:
		v = &flow.CollectionGuarantee{}
	case codec.CodeTransactionBody:
		v = &flow.TransactionBody{}
	case codec.CodeTransaction:
		v = &flow.Transaction{}

	// core messages for execution & verification
	case codec.CodeExecutionReceipt:
		v = &flow.ExecutionReceipt{}
	case codec.CodeResultApproval:
		v = &flow.ResultApproval{}

	// execution state synchronization
	case codec.CodeExecutionStateSyncRequest:
		v = &messages.ExecutionStateSyncRequest{}
	case codec.CodeExecutionStateDelta:
		v = &messages.ExecutionStateDelta{}

	// data exchange for execution of blocks
	case codec.CodeChunkDataRequest:
		v = &messages.ChunkDataRequest{}
	case codec.CodeChunkDataResponse:
		v = &messages.ChunkDataResponse{}

	case codec.CodeApprovalRequest:
		v = &messages.ApprovalRequest{}
	case codec.CodeApprovalResponse:
		v = &messages.ApprovalResponse{}

	// generic entity exchange engines
	case codec.CodeEntityRequest:
		v = &messages.EntityRequest{}
	case codec.CodeEntityResponse:
		v = &messages.EntityResponse{}

	// testing
	case codec.CodeEcho:
		v = &message.TestMessage{}

	// dkg
	case codec.CodeDKGMessage:
		v = &messages.DKGMessage{}

	default:
		return nil, errors.Errorf("invalid message code (%d)", code)
	}

	return v, nil
}

func switchenv2what(code uint8) (string, error) {
	var what string

	switch code {

	// consensus
	case codec.CodeBlockProposal:
		what = "CodeBlockProposal"
	case codec.CodeBlockVote:
		what = "CodeBlockVote"

	// cluster consensus
	case codec.CodeClusterBlockProposal:
		what = "CodeClusterBlockProposal"
	case codec.CodeClusterBlockVote:
		what = "CodeClusterBlockVote"
	case codec.CodeClusterBlockResponse:
		what = "CodeClusterBlockResponse"

	// protocol state sync
	case codec.CodeSyncRequest:
		what = "CodeSyncRequest"
	case codec.CodeSyncResponse:
		what = "CodeSyncResponse"
	case codec.CodeRangeRequest:
		what = "CodeRangeRequest"
	case codec.CodeBatchRequest:
		what = "CodeBatchRequest"
	case codec.CodeBlockResponse:
		what = "CodeBlockResponse"

	// collections, guarantees & transactions
	case codec.CodeCollectionGuarantee:
		what = "CodeCollectionGuarantee"
	case codec.CodeTransactionBody:
		what = "CodeTransactionBody"
	case codec.CodeTransaction:
		what = "CodeTransaction"

	// core messages for execution & verification
	case codec.CodeExecutionReceipt:
		what = "CodeExecutionReceipt"
	case codec.CodeResultApproval:
		what = "CodeResultApproval"

	// execution state synchronization
	case codec.CodeExecutionStateSyncRequest:
		what = "CodeExecutionStateSyncRequest"
	case codec.CodeExecutionStateDelta:
		what = "CodeExecutionStateDelta"

	// data exchange for execution of blocks
	case codec.CodeChunkDataRequest:
		what = "CodeChunkDataRequest"
	case codec.CodeChunkDataResponse:
		what = "CodeChunkDataResponse"

	case codec.CodeApprovalRequest:
		what = "CodeApprovalRequest"
	case codec.CodeApprovalResponse:
		what = "CodeApprovalResponse"

	// generic entity exchange engines
	case codec.CodeEntityRequest:
		what = "CodeEntityRequest"
	case codec.CodeEntityResponse:
		what = "CodeEntityResponse"

	// testing
	case codec.CodeEcho:
		what = "CodeEcho"

	// dkg
	case codec.CodeDKGMessage:
		what = "CodeDKGMessage"

	default:
		return "", errors.Errorf("invalid message code (%d)", code)
	}

	return what, nil
}

// Given a uint8 'code', return a Golang interface 'v', and a string
// 'what' naming the interface for debugging purposes.
// Return an error if the 'code' is unknown.
// NOTE: 'v' is the network message payload in unserialized form.
// NOTE: 'code' is the message type.
// NOTE: 'what' is the 'code' name for debugging / instrumentation.
// NOTE: 'envelope' contains 'code' & serialized / encoded 'v'.
func envelopeCode2v(code uint8) (string, interface{}, error) {

	// create the desired message
	v, err := switchenv2v(code)
	if nil != err {
		return "", nil, err
	}

	what, err := switchenv2what(code)
	if nil != err {
		return "", nil, err
	}

	return what, v, nil
}
