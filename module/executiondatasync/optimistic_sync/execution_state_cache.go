package optimistic_sync

import (
	"errors"

	"github.com/onflow/flow-go/model/flow"
)

// ExecutionStateCache provides access to execution state snapshots for querying data at specific ExecutionResults.
type ExecutionStateCache interface {
	// Snapshot returns a view of the execution state as of the provided ExecutionResult.
	// The returned Snapshot provides access to execution state data for the fork ending
	// on the provided ExecutionResult which extends from the latest sealed result.
	// The result may be sealed or unsealed. Only data for finalized blocks is available.
	//
	// Expected error returns during normal operation:
	//   - [SnapshotNotFoundError]: Result is not available, not ready for querying, or does not descend from the latest sealed result.
	Snapshot(executionResultID flow.Identifier) (Snapshot, error)
}

var SnapshotNotFoundError = errors.New("execution state snapshot for the given execution result ID not found")
