package operation

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// RetrieveResultApproval retrieves an approval by ID.
// Returns `storage.ErrNotFound` if no Approval with the given ID has been stored.
func RetrieveResultApproval(r storage.Reader, approvalID flow.Identifier, approval *flow.ResultApproval) error {
	return RetrieveByKey(r, MakePrefix(codeResultApproval, approvalID), approval)
}

// InsertAndIndexResultApproval inserts a ResultApproval ID keyed by ExecutionResult ID
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
func InsertAndIndexResultApproval(approval *flow.ResultApproval) func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
	approvalID := approval.ID()
	resultID := approval.Body.ExecutionResultID
	chunkIndex := approval.Body.ChunkIndex

	return func(lctx lockctx.Proof, rw storage.ReaderBatchWriter) error {
		if !lctx.HoldsLock(storage.LockIndexResultApproval) {
			return fmt.Errorf("missing lock for index result approval for result: %v", resultID)
		}

		var storedApprovalID flow.Identifier
		err := LookupResultApproval(rw.GlobalReader(), resultID, chunkIndex, &storedApprovalID)
		if err == nil {
			if storedApprovalID != approvalID {
				return fmt.Errorf("attempting to store conflicting approval (result: %v, chunk index: %d): storing: %v, stored: %v. %w",
					resultID, chunkIndex, approvalID, storedApprovalID, storage.ErrDataMismatch)
			}

			// already stored and indexed
			return nil
		}

		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("could not lookup result approval ID: %w", err)
		}

		err = UpsertByKey(rw.Writer(), MakePrefix(codeResultApproval, approvalID), approval)
		if err != nil {
			return fmt.Errorf("could not store result approval: %w", err)
		}

		return UpsertByKey(rw.Writer(), MakePrefix(codeIndexResultApprovalByChunk, resultID, chunkIndex), approvalID)
	}
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
