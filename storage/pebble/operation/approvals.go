package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// InsertResultApproval inserts a ResultApproval by ID.
func InsertResultApproval(approval *flow.ResultApproval) func(*badger.Txn) error {
	return insert(makePrefix(codeResultApproval, approval.ID()), approval)
}

// RetrieveResultApproval retrieves an approval by ID.
func RetrieveResultApproval(approvalID flow.Identifier, approval *flow.ResultApproval) func(*badger.Txn) error {
	return retrieve(makePrefix(codeResultApproval, approvalID), approval)
}

// IndexResultApproval inserts a ResultApproval ID keyed by ExecutionResult ID
// and chunk index. If a value for this key exists, a storage.ErrAlreadyExists
// error is returned. This operation is only used by the ResultApprovals store,
// which is only used within a Verification node, where it is assumed that there
// is only one approval per chunk.
func IndexResultApproval(resultID flow.Identifier, chunkIndex uint64, approvalID flow.Identifier) func(*badger.Txn) error {
	return insert(makePrefix(codeIndexResultApprovalByChunk, resultID, chunkIndex), approvalID)
}

// LookupResultApproval finds a ResultApproval by result ID and chunk index.
func LookupResultApproval(resultID flow.Identifier, chunkIndex uint64, approvalID *flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(codeIndexResultApprovalByChunk, resultID, chunkIndex), approvalID)
}
