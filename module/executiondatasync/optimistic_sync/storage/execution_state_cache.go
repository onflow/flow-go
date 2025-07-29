package storage

import "github.com/onflow/flow-go/model/flow"

// ExecutionStateCache provides access to execution state snapshots for querying data at specific ExecutionResults.
type ExecutionStateCache interface {
	// Snapshot returns a view of the execution state as of the requested ExecutionResult.
	//
	// The method validates that:
	//   - The result is available
	//   - The result is ready for querying
	//   - The result descends from the latest sealed result
	//
	// The returned Snapshot provides access to execution data, including both sealed and unsealed data.
	//
	// Expected errors during normal operation:
	//   - storage.ErrNotFound - result is not available, not ready for querying, or does not descend from the latest sealed result.
	//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
	Snapshot(executionResultID flow.Identifier) (Snapshot, error)
}
