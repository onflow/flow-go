package operation

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertResultApproval inserts a ResultApproval by ID.
// The same key (`approval.ID()`) necessitates that the value (full `approval`) is
// also identical (otherwise, we would have a successful pre-image attack on our
// cryptographic hash function). Therefore, concurrent calls to this function are safe.
func InsertResultApproval(approval *flow.ResultApproval) func(storage.Writer) error {
	return insertW(makePrefix(codeResultApproval, approval.ID()), approval)
}

// RetrieveResultApproval retrieves an approval by ID.
func RetrieveResultApproval(approvalID flow.Identifier, approval *flow.ResultApproval) func(storage.Reader) error {
	return retrieveR(makePrefix(codeResultApproval, approvalID), approval)
}

// UnsafeIndexResultApproval inserts a ResultApproval ID keyed by ExecutionResult ID
// and chunk index. If a value for this key exists, a storage.ErrAlreadyExists
// error is returned. This operation is only used by the ResultApprovals store,
// which is only used within a Verification node, where it is assumed that there
// is only one approval per chunk.
// CAUTION: In order to prevent overwriting, use of this function must be
// synchronized with check (RetrieveResultApproval) for existance of the key.
func UnsafeIndexResultApproval(resultID flow.Identifier, chunkIndex uint64, approvalID flow.Identifier) func(storage.Writer) error {
	return insertW(makePrefix(codeIndexResultApprovalByChunk, resultID, chunkIndex), approvalID)
}

// LookupResultApproval finds a ResultApproval by result ID and chunk index.
func LookupResultApproval(resultID flow.Identifier, chunkIndex uint64, approvalID *flow.Identifier) func(storage.Reader) error {
	return retrieveR(makePrefix(codeIndexResultApprovalByChunk, resultID, chunkIndex), approvalID)
}
