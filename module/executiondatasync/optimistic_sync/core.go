package optimistic_sync

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
)

// CoreFactory is a factory for creating new Core instances.
type CoreFactory interface {
	NewCore(result *flow.ExecutionResult) Core
}

// Core implements the core logic for execution data processing. It exposes methods for each of the
// processing steps, which must be called sequentially in the order: Download, Index, Persist.
// Abandon may be called at any time to abort processing and cleanup working data.
// The Core instance cannot be used after Abandon is called, and will return [pipeline.ErrAbandoned].
type Core interface {
	// Download retrieves execution data and transaction results error for the block.
	//
	// Expected error returns during normal operations:
	// - context.Canceled: if the provided context was canceled before completion
	// - [ErrAbandoned]: if the core is already abandoned
	Download(ctx context.Context) error

	// Index processes the downloaded execution data and transaction results error messages and
	// indexes them into in-memory storage.
	//
	// Expected error returns during normal operations:
	// - ErrAbandoned: if the core is already abandoned
	Index() error

	// Persist stores the indexed data into permanent storage.
	//
	// Expected error returns during normal operations:
	// - ErrAbandoned: if the core is already abandoned
	Persist() error

	// Abandon indicates that the protocol has abandoned this state. Hence processing will be aborted
	// and any data dropped.
	//
	// Expected error returns during normal operations:
	// - ErrAbandoned: if the core is already abandoned
	Abandon() error
}
