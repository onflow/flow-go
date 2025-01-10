package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertResultApproval inserts a ResultApproval by ID.
func InsertResultApproval(w storage.Writer, approval *flow.ResultApproval) error {
	return UpsertByKey(w, MakePrefix(codeResultApproval, approval.ID()), approval)
}

// RetrieveResultApproval retrieves an approval by ID.
func RetrieveResultApproval(r storage.Reader, approvalID flow.Identifier, approval *flow.ResultApproval) error {
	return RetrieveByKey(r, MakePrefix(codeResultApproval, approvalID), approval)
}

// UnsafeIndexResultApproval inserts a ResultApproval ID keyed by ExecutionResult ID
// and chunk index. If a value for this key exists, a storage.ErrAlreadyExists
// error is returned. This operation is only used by the ResultApprovals store,
// which is only used within a Verification node, where it is assumed that there
// is only one approval per chunk.
// CAUTION: Use of this function must be synchronized by storage.ResultApprovals.
func UnsafeIndexResultApproval(w storage.Writer, resultID flow.Identifier, chunkIndex uint64, approvalID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeIndexResultApprovalByChunk, resultID, chunkIndex), approvalID)
}

// LookupResultApproval finds a ResultApproval by result ID and chunk index.
func LookupResultApproval(r storage.Reader, resultID flow.Identifier, chunkIndex uint64, approvalID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeIndexResultApprovalByChunk, resultID, chunkIndex), approvalID)
}
