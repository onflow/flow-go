package optimistic_sync

import (
	"context"
)

// Core defines the interface for pipeline processing steps.
// Each implementation should handle an execution data and implement the three-phase processing:
// download, index, and persist.
//
// CAUTION: not concurrency safe!
type Core interface {
	// Download retrieves all necessary data for processing.
	// CAUTION: not concurrency safe!
	// Expected errors:
	// - context.Canceled: if the provided context was canceled before completion
	// - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
	Download(ctx context.Context) error

	// Index processes the downloaded data and creates in-memory indexes.
	// CAUTION: not concurrency safe!
	//
	// No errors are expected during normal operations
	Index() error

	// Persist stores the indexed data in permanent storage.
	// CAUTION: not concurrency safe!
	//
	// No errors are expected during normal operations
	Persist() error

	// Abandon indicates that the protocol has abandoned this state. Hence processing will be aborted
	// and any data dropped.
	// CAUTION: not concurrency safe!
	//
	// No errors are expected during normal operations
	Abandon() error
}
