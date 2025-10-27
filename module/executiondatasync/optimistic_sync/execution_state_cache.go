package optimistic_sync

import (
	"github.com/onflow/flow-go/model/flow"
)

// ExecutionStateCache provides access to execution state snapshots for querying data at specific ExecutionResults.
type ExecutionStateCache interface {
	// Snapshot returns a view of the execution state as of the provided ExecutionResult.
	// The returned Snapshot provides access to execution state data for the fork ending
	// on the provided ExecutionResult which extends from the latest sealed result.
	// The result may be sealed or unsealed. Only data for finalized blocks is available.
	//
	// Expected errors during normal operation:
	//   - storage.ErrNotFound - result is not available, not ready for querying, or does not descend from the latest sealed result.
	//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
	Snapshot(executionResultID flow.Identifier) (Snapshot, error)
}
