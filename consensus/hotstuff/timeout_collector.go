package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// TimeoutCollector collects all timeout objects for a specified view. On the happy path, it
// generates a TimeoutCertificate when enough timeouts have been collected.
// Implementations of TimeoutCollector must be concurrency safe.
type TimeoutCollector interface {
	// AddTimeout adds a Timeout Object [TO] to the collector.
	// When TOs from strictly more than 1/3 of consensus participants (measured by weight)
	// were collected, the callback for partial TC will be triggered.
	// After collecting TOs from a supermajority, a TC will be created and passed to the EventLoop.
	// All errors propagated to caller are exceptions.
	AddTimeout(timeoutObject *model.TimeoutObject) error

	// View returns the view that this instance is collecting timeouts for.
	// This method is useful when adding the newly created timeout collector to timeout collectors map.
	View() uint64
}

// TimeoutProcessor performs processing of single timeout object.
// It implements the timeout object specific processing logic.
// Depending on their implementation, a TimeoutProcessor might drop timeouts or attempt to construct a TC.
type TimeoutProcessor interface {
	// Process performs processing of single timeout object. This function is safe to call from multiple goroutines.
	// Expected error returns during normal operations:
	// * timeoutcollector.ErrTimeoutForIncompatibleView - submitted timeout for incompatible view
	// * model.InvalidTimeoutError - submitted invalid timeout(invalid structure or invalid signature)
	// All other errors should be treated as exceptions.
	Process(timeout *model.TimeoutObject) error
}
