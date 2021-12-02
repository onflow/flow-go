package hotstuff

import "github.com/onflow/flow-go/module"

// VoteCollectors is an interface which allows VoteAggregator to interact with collectors structured by
// view and blockID.
// Implementations of this interface are responsible for state transitions of `VoteCollector`s and pruning of
// stale and outdated collectors by view.
type VoteCollectors interface {
	module.ReadyDoneAware
	module.Startable

	// GetOrCreateCollector retrieves the hotstuff.VoteCollector for the specified
	// view or creates one if none exists.
	// When creating a vote collector, the view will be used to get epoch by view, then create the random beacon
	// signer object by epoch, because epoch determines DKG, which determines random beacon committee.
	// It returns:
	//  -  (collector, true, nil) if no collector can be found by the block ID, and a new collector was created.
	//  -  (collector, false, nil) if the collector can be found by the block ID
	//  -  (nil, false, error) if running into any exception creating the vote collector state machine
	// Expected error returns during normal operations:
	//  * mempool.DecreasingPruningHeightError
	GetOrCreateCollector(view uint64) (collector VoteCollector, created bool, err error)

	// PruneUpToView prunes the vote collectors with views _below_ the given view.
	// If `view` is smaller than the previous value, the previous value is kept
	// and the method call is a NoOp.
	PruneUpToView(view uint64)
}

// Workers queues and processes submitted tasks. We explicitly do not
// expose any functionality to terminate the worker pool.
type Workers interface {
	// Submit enqueues a function for a worker to execute. Submit will not block
	// regardless of the number of tasks submitted. Each task is immediately
	// given to an available worker or queued otherwise. Tasks are processed in
	// FiFO order.
	Submit(task func())
}

// Workerpool adds the functionality to terminate the workers to the
// Workers interface.
type Workerpool interface {
	Workers

	// StopWait stops the worker pool and waits for all queued tasks to
	// complete.  No additional tasks may be submitted, but all pending tasks are
	// executed by workers before this function returns.
	StopWait()
}
