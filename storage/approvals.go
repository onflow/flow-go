package storage

import (
	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/model/flow"
)

// ResultApprovals implements persistent storage for result approvals.
// Implementations of this interface must be concurrency safe.
//
// CAUTION suitable only for _Verification Nodes_ for persisting their _own_ approvals!
//   - In general, the Flow protocol requires multiple approvals for the same chunk from different
//     verification nodes. In other words, there are multiple different approvals for the same chunk.
//   - Internally, ResultApprovals populates an index from Executed Chunk ➜ ResultApproval. This is
//     *only safe* for Verification Nodes when tracking their own approvals (for the same ExecutionResult,
//     a Verifier will always produce the same approval)
type ResultApprovals interface {

	// StoreMyApproval returns a functor, whose execution
	//  - will store the given ResultApproval
	//  - and index it by result ID and chunk index.
	//  - requires storage.LockIndexResultApproval lock to be held by the caller
	// The functor's expected error returns during normal operation are:
	//  - `storage.ErrDataMismatch` if a *different* approval for the same key pair (ExecutionResultID, chunk index) is already indexed
	//
	// CAUTION: the Flow protocol requires multiple approvals for the same chunk from different verification
	// nodes. In other words, there are multiple different approvals for the same chunk. Therefore, the index
	// Executed Chunk ➜ ResultApproval ID (populated here) is *only safe* to be used by Verification Nodes
	// for tracking their own approvals.
	//
	// For the same ExecutionResult, a Verifier will always produce the same approval. Therefore, this operation
	// is idempotent, i.e. repeated calls with the *same inputs* are equivalent to just calling the method once;
	// still the method succeeds on each call. However, when attempting to index *different* ResultApproval IDs
	// for the same key (resultID, chunkIndex) this method returns an exception, as this should never happen for
	// a correct Verification Node indexing its own approvals.
	// It returns a functor so that some computation (such as computing approval ID) can be done
	// before acquiring the lock.
	StoreMyApproval(approval *flow.ResultApproval) func(lctx lockctx.Proof) error

	// ByID retrieves a ResultApproval by its ID.
	// Returns [storage.ErrNotFound] if no Approval with the given ID has been stored.
	ByID(approvalID flow.Identifier) (*flow.ResultApproval, error)

	// ByChunk retrieves a ResultApproval by result ID and chunk index.
	// Returns [storage.ErrNotFound] if no Approval for  the given key (resultID, chunkIndex) has been stored.
	ByChunk(resultID flow.Identifier, chunkIndex uint64) (*flow.ResultApproval, error)
}
