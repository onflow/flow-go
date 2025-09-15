package optimistic_sync

import (
	"errors"

	"github.com/onflow/flow-go/model/flow"
)

// ErrSnapshotNotFound indicates that the execution result is not found or not ready for querying.
var ErrSnapshotNotFound = errors.New("execution result not found for block")

// ExecutionStateCache provides access to execution state snapshots for querying data at specific ExecutionResults.
type ExecutionStateCache interface {
	// Snapshot returns a view of the execution state as of the provided ExecutionResult.
	// The returned Snapshot provides access to execution state data for the fork ending
	// on the provided ExecutionResult which extends from the latest sealed result.
	// The result may be sealed or unsealed. Only data for finalized blocks is available.
	//
	// Expected error returns during normal operation:
	//   - ErrSnapshotNotFound - result is not found or not ready for querying.
	Snapshot(executionResultID flow.Identifier) (Snapshot, error)
}
