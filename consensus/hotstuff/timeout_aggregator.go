package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module"
)

// TimeoutAggregator verifies and aggregates timeout objects to build timeout certificates [TCs].
// When enough timeout objects are collected, it builds a TC and sends it to the EventLoop
// TimeoutAggregator also detects protocol violation, including invalid timeouts, double timeout, etc and
// notifies a HotStuff consumer for slashing.
type TimeoutAggregator interface {
	module.ReadyDoneAware
	module.Startable

	// AddTimeout verifies and aggregates a timeout object.
	// This method can be called concurrently, timeouts will be queued and processed asynchronously.
	AddTimeout(timeoutObject *model.TimeoutObject)

	// PruneUpToView deletes all timeouts _below_ to the given view, as well as
	// related indices. We only retain and process timeouts, whose view is equal or larger
	// than `lowestRetainedView`. If `lowestRetainedView` is smaller than the
	// previous value, the previous value is kept and the method call is a NoOp.
	// This value should be set the latest active view maintained by `Pacemaker`.
	PruneUpToView(lowestRetainedView uint64)
}
