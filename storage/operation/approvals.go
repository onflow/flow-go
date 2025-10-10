package operation

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// RetrieveResultApproval retrieves an approval by ID.
// Returns `storage.ErrNotFound` if no Approval with the given ID has been stored.
func RetrieveResultApproval(r storage.Reader, approvalID flow.Identifier, approval *flow.ResultApproval) error {
	return RetrieveByKey(r, MakePrefix(codeResultApproval, approvalID), approval)
}

// InsertAndIndexResultApproval atomically performs the following storage operations:
//  1. Store ResultApproval by its ID (in this step, accidental overwrites with inconsistent values
//     are prevented by using a collision-resistant hash to derive the key from the value)
//  2. Index approval by the executed chunk, specifically the key pair (ExecutionResultID, chunk index).
//     - first, we ensure that no _different_ approval has already been indexed for the same key pair
//     - only if the prior check succeeds, we write the index to the database
//
// CAUTION:
//   - In general, the Flow protocol requires multiple approvals for the same chunk from different
//     verification nodes. In other words, there are multiple different approvals for the same chunk.
//     Therefore, this index Executed Chunk ➜ ResultApproval ID is *only safe* to be used by
//     Verification Nodes for tracking their own approvals (for the same ExecutionResult, a Verifier
//     will always produce the same approval)
//   - In order to make sure only one approval is indexed for the chunk, _all calls_ to
//     `InsertAndIndexResultApproval` must be synchronized by the higher-logic. Currently, we have the
//     lockctx.Proof to prove the higher logic is holding the lock inserting the approval after checking
//     that the approval is not already indexed.
//
// Expected error returns:
//   - `storage.ErrDataMismatch` if a *different* approval for the same key pair (ExecutionResultID, chunk index) is already indexed
func InsertAndIndexResultApproval(approval *flow.ResultApproval) Functor {
	approvalID := approval.ID()
	resultID := approval.Body.ExecutionResultID
	chunkIndex := approval.Body.ChunkIndex

	errmsg := fmt.Sprintf("InsertAndIndexResultApproval failed with approvalID %v, chunkIndex %v, resultID %v",
		approvalID, chunkIndex, resultID)
	return WrapError(errmsg, BindFunctors(
		HoldingLock(storage.LockIndexResultApproval),
		Overwriting(MakePrefix(codeResultApproval, approvalID), approval),
		InsertingWithMismatchCheck(MakePrefix(codeIndexResultApprovalByChunk, resultID, chunkIndex), approvalID),
	))
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
