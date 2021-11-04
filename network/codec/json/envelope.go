// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package json

import (
	"encoding/json"
)

const (

	// consensus
	CodeBlockProposal = iota + 1
	CodeBlockVote

	// protocol state sync
	CodeSyncRequest
	CodeSyncResponse
	CodeRangeRequest
	CodeRangeResponse
	CodeBatchRequest
	CodeBatchResponse

	// cluster consensus
	CodeClusterBlockProposal
	CodeClusterBlockVote
	CodeClusterBatchResponse
	CodeClusterRangeResponse

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
)

// Envelope is a wrapper to convey type information with JSON encoding without
// writing custom bytes to the wire.
type Envelope struct {
	Code uint8
	Data json.RawMessage
}
