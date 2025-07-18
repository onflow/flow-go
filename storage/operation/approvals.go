package operation

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// InsertResultApproval inserts a ResultApproval by ID.
// The same key (`approval.ID()`) necessitates that the value (full `approval`) is
// also identical (otherwise, we would have a successful pre-image attack on our
// cryptographic hash function). Therefore, concurrent calls to this function are safe.
func InsertResultApproval(lctx lockctx.Proof, w storage.Writer, approval *flow.ResultApproval) error {
	if !lctx.HoldsLock(storage.LockIndexResultApproval) {
		return fmt.Errorf("missing lock for insert result approval for block %v result: %v",
			approval.Body.BlockID,
			approval.Body.ExecutionResultID)
	}
	return UpsertByKey(w, MakePrefix(codeResultApproval, approval.ID()), approval)
}

// RetrieveResultApproval retrieves an approval by ID.
// Returns `storage.ErrNotFound` if no Approval with the given ID has been stored.
func RetrieveResultApproval(r storage.Reader, approvalID flow.Identifier, approval *flow.ResultApproval) error {
	return RetrieveByKey(r, MakePrefix(codeResultApproval, approvalID), approval)
}

// IndexResultApproval inserts a ResultApproval ID keyed by ExecutionResult ID
// and chunk index.
// Note: Unsafe means it does not check if a different approval is indexed for the same
// chunk, and will overwrite the existing index.
// CAUTION:
//   - In general, the Flow protocol requires multiple approvals for the same chunk from different
//     verification nodes. In other words, there are multiple different approvals for the same chunk.
//     Therefore, this index Executed Chunk ➜ ResultApproval ID is *only safe* to be used by
//     Verification Nodes for tracking their own approvals (for the same ExecutionResult, a Verifier
//     will always produce the same approval)
//   - In order to make sure only one approval is indexed for the chunk, _all calls_ to
//     `UnsafeIndexResultApproval` must be synchronized by the higher-logic. Currently, we have the
//     lockctx.Proof to prove the higher logic is holding the lock inserting the approval after checking
//     that the approval is not already indexed.
func UnsafeIndexResultApproval(lctx lockctx.Proof, w storage.Writer, resultID flow.Identifier, chunkIndex uint64, approvalID flow.Identifier) error {
	if !lctx.HoldsLock(storage.LockIndexResultApproval) {
		return fmt.Errorf("missing lock for index result approval for result: %v", resultID)
	}
	return UpsertByKey(w, MakePrefix(codeIndexResultApprovalByChunk, resultID, chunkIndex), approvalID)
}

// LookupResultApproval finds a ResultApproval by result ID and chunk index.
// Returns `storage.ErrNotFound` if no Approval for  the given key (resultID, chunkIndex) has been stored.
//
// NOTE that the Flow protocol requires multiple approvals for the same chunk from different verification
// nodes. In other words, there are multiple different approvals for the same chunk. Therefore, the index
// Executed Chunk ➜ ResultApproval ID  (queried here) is *only safe* to be used by Verification Nodes
// for tracking their own approvals (for the same ExecutionResult, a Verifier will always produce the same approval)
func LookupResultApproval(r storage.Reader, resultID flow.Identifier, chunkIndex uint64, approvalID *flow.Identifier) error {
	return RetrieveByKey(r, MakePrefix(codeIndexResultApprovalByChunk, resultID, chunkIndex), approvalID)
}
