package optimistic_sync

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// CoreFactory is a factory object for creating new Core instances.
type CoreFactory interface {
	NewCore(result *flow.ExecutionResult) Core
}

// ErrResultAbandoned is returned when calling one of the methods after the result has been abandoned.
// Not exported because this is not an expected error condition.
var ErrResultAbandoned = fmt.Errorf("result abandoned")

// <component_spec>
// Core defines the interface for pipelined execution result processing. There are 3 main steps which
// must be completed sequentially and exactly once.
// 1. Download the BlockExecutionData and TransactionResultErrorMessages for the execution result.
// 2. Index the downloaded data into mempools.
// 3. Persist the indexed data to into persisted storage.
//
// If the protocol abandons the execution result, Abandon() is called to signal to the Core instance
// that processing will stop and any data accumulated may be discarded. Abandon() may be called at
// any time, but may block until in-progress operations are complete.
// </component_spec>
//
// All exported methods are safe for concurrent use.
type Core interface {
	// Download retrieves all necessary data for processing from the network.
	// Download will block until the data is successfully downloaded, and has not internal timeout.
	// When Aboandon is called, the caller must cancel the context passed in to shutdown the operation
	// otherwise it may block indefinitely.
	//
	// Expected error returns during normal operation:
	// - [context.Canceled]: if the provided context was canceled before completion
	Download(ctx context.Context) error

	// Index processes the downloaded data and stores it into in-memory indexes.
	// Must be called after Download.
	//
	// No error returns are expected during normal operations
	Index() error

	// Persist stores the indexed data in permanent storage.
	// Must be called after Index.
	//
	// No error returns are expected during normal operations
	Persist() error

	// Abandon indicates that the protocol has abandoned this state. Hence processing will be aborted
	// and any data dropped.
	// This method will block until other in-progress operations are complete. If Download is in progress,
	// the caller should cancel its context to ensure the operation completes in a timely manner.
	Abandon()
}
