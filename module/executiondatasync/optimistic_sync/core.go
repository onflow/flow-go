package optimistic_sync

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
)

// CoreFactory is a factory object for creating new Core instances.
type CoreFactory interface {
	NewCore(result *flow.ExecutionResult) Core
}

// Core defines the interface for pipeline processing steps.
// Each implementation should handle an execution data and implement the three-phase processing:
// download, index, and persist.
// CAUTION: The Core instance should not be used after Abandon is called as it could cause panic due to cleared data.
// Core implementations must be
// - CONCURRENCY SAFE
type Core interface {
	// Download retrieves all necessary data for processing.
	// Concurrency safe - all operations will be executed sequentially.
	//
	// Expected errors:
	// - context.Canceled: if the provided context was canceled before completion
	// - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
	Download(ctx context.Context) error

	// Index processes the downloaded data and creates in-memory indexes.
	// Concurrency safe - all operations will be executed sequentially.
	//
	// No errors are expected during normal operations
	Index() error

	// Persist stores the indexed data in permanent storage.
	// Concurrency safe - all operations will be executed sequentially.
	//
	// No errors are expected during normal operations
	Persist() error

	// Abandon indicates that the protocol has abandoned this state. Hence processing will be aborted
	// and any data dropped.
	// Concurrency safe - all operations will be executed sequentially.
	// CAUTION: The Core instance should not be used after Abandon is called as it could cause panic due to cleared data.
	//
	// No errors are expected during normal operations
	Abandon() error
}
