package optimistic_sync

import (
	"context"
)

// Core defines the interface for pipeline processing steps.
// Each implementation should handle an execution data and implement the three-phase processing:
// download, index, and persist.
type Core interface {
	// Download retrieves all necessary data for processing.
	// Expected errors:
	// - context.Canceled: if the provided context was canceled before completion
	//
	// All other errors are unexpected and may indicate a bug or inconsistent state
	Download(ctx context.Context) error

	// Index processes the downloaded data and creates in-memory indexes.
	//
	// No errors are expected during normal operations
	Index() error

	// Persist stores the indexed data in permanent storage.
	//
	// No errors are expected during normal operations
	Persist() error

	// Abandon indicates that the protocol has abandoned this state. Hence processing will be aborted
	// and any data dropped.
	//
	// No errors are expected during normal operations
	Abandon() error
}
