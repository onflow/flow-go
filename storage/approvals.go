package storage

import (
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

	// Store stores a ResultApproval by its ID.
	// No errors are expected during normal operations.
	Store(result *flow.ResultApproval) error

	// Index indexes a ResultApproval by result ID and chunk index.
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
	// No errors are expected during normal operations.
	Index(resultID flow.Identifier, chunkIndex uint64, approvalID flow.Identifier) error

	// ByID retrieves a ResultApproval by its ID.
	// Returns [storage.ErrNotFound] if no Approval with the given ID has been stored.
	ByID(approvalID flow.Identifier) (*flow.ResultApproval, error)

	// ByChunk retrieves a ResultApproval by result ID and chunk index.
	// Returns [storage.ErrNotFound] if no Approval for  the given key (resultID, chunkIndex) has been stored.
	ByChunk(resultID flow.Identifier, chunkIndex uint64) (*flow.ResultApproval, error)
}
