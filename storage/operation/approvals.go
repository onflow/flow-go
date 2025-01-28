package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertResultApproval inserts a ResultApproval by ID.
// The same key (`approval.ID()`) necessitates that the value (full `approval`) is
// also identical (otherwise, we would have a successful pre-image attack on our
// cryptographic hash function). Therefore, concurrent calls to this function are safe.
func InsertResultApproval(w storage.Writer, approval *flow.ResultApproval) error {
	return UpsertByKey(w, MakePrefix(codeResultApproval, approval.ID()), approval)
}

// RetrieveResultApproval retrieves an approval by ID.
func RetrieveResultApproval(r storage.Reader, approvalID flow.Identifier, approval *flow.ResultApproval) error {
	return RetrieveByKey(r, MakePrefix(codeResultApproval, approvalID), approval)
}

// UnsafeIndexResultApproval inserts a ResultApproval ID keyed by ExecutionResult ID
// and chunk index.
// Unsafe means that it does not check if a different approval is indexed for the same
// chunk, and will overwrite the existing index.
// CAUTION: in order to make sure only one approval is indexed for the chunk, the Caller
// must synchronized by storage.ResultApprovals and check no approval is already indexed.
func UnsafeIndexResultApproval(w storage.Writer, resultID flow.Identifier, chunkIndex uint64, approvalID flow.Identifier) error {
	return UpsertByKey(w, MakePrefix(codeIndexResultApprovalByChunk, resultID, chunkIndex), approvalID)
}

// LookupResultApproval finds a ResultApproval by result ID and chunk index.
func LookupResultApproval(r storage.Reader, resultID flow.Identifier, chunkIndex uint64, approvalID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeIndexResultApprovalByChunk, resultID, chunkIndex), approvalID)
}
