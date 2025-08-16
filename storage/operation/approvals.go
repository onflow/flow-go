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

// UnsafeIndexResultApproval inserts a ResultApproval ID keyed by ExecutionResult ID
// and chunk index. OVERWRITES the existing value for the key (it if exists).
// CAUTION:
//   - In general, the Flow protocol requires multiple approvals for the same chunk from different
//     verification nodes. In other words, there are multiple different approvals for the same chunk.
//     Therefore, this index Executed Chunk ➜ ResultApproval ID is *only safe* to be used by
//     Verification Nodes for tracking their own approvals (for the same ExecutionResult, a Verifier
//     must always produce the same approval).
//   - A verifier sending _different_ approvals for the _same chunk_ is a slashable protocol violation,
//     which we want to prevent in all cases via a sanity check. In order to ensure that at most a
//     single approval is indexed for the chunk, the CALLER must acquire [storage.LockIndexResultApproval]
//     and atomically check that no conflicting value was already indexed prior to writing.
//     [UnsafeIndexResultApproval] receives the `lockctx.Proof` to verify that the higher-level logic is
//     indeed holding the lock for inserting the approvals - and signaling to the caller that atomic checks
//     must be performed before calling this function.
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
