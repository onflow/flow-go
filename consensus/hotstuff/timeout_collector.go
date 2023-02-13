package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// TimeoutCollector collects all timeout objects for a specified view. On the happy path, it
// generates a TimeoutCertificate when enough timeouts have been collected. The TimeoutCollector
// is a higher-level structure that orchestrates deduplication, caching and processing of
// timeouts, delegating those tasks to underlying modules (such as TimeoutProcessor).
// Implementations of TimeoutCollector must be concurrency safe.
type TimeoutCollector interface {
	// AddTimeout adds a Timeout Object [TO] to the collector.
	// When TOs from strictly more than 1/3 of consensus participants (measured by weight)
	// were collected, the callback for partial TC will be triggered.
	// After collecting TOs from a supermajority, a TC will be created and passed to the EventLoop.
	// Expected error returns during normal operations:
	// * timeoutcollector.ErrTimeoutForIncompatibleView - submitted timeout for incompatible view
	// All other exceptions are symptoms of potential state corruption.
	AddTimeout(timeoutObject *model.TimeoutObject) error

	// View returns the view that this instance is collecting timeouts for.
	// This method is useful when adding the newly created timeout collector to timeout collectors map.
	View() uint64
}

// TimeoutProcessor ingests Timeout Objects [TO] for a particular view. It implements the algorithms
// for validating TOs, orchestrates their low-level aggregation and emits `OnPartialTcCreated` and
// `OnTcConstructedFromTimeouts` notifications. TimeoutProcessor cannot deduplicate TOs (this should
// be handled by the higher-level TimeoutCollector) and errors instead.
// Depending on their implementation, a TimeoutProcessor might drop timeouts or attempt to construct a TC.
type TimeoutProcessor interface {
	// Process performs processing of single timeout object. This function is safe to call from multiple goroutines.
	// Expected error returns during normal operations:
	// * timeoutcollector.ErrTimeoutForIncompatibleView - submitted timeout for incompatible view
	// * model.InvalidTimeoutError - submitted invalid timeout(invalid structure or invalid signature)
	// * model.DuplicatedSignerError if a timeout from the same signer was previously already added
	//   It does _not necessarily_ imply that the timeout is invalid or the sender is equivocating.
	// All other errors should be treated as exceptions.
	Process(timeout *model.TimeoutObject) error
}

// TimeoutCollectorFactory performs creation of TimeoutCollector for a given view
type TimeoutCollectorFactory interface {
	// Create is a factory method to generate a TimeoutCollector for a given view
	// Expected error returns during normal operations:
	//  * model.ErrViewForUnknownEpoch no epoch containing the given view is known
	// All other errors should be treated as exceptions.
	Create(view uint64) (TimeoutCollector, error)
}

// TimeoutProcessorFactory performs creation of TimeoutProcessor for a given view
type TimeoutProcessorFactory interface {
	// Create is a factory method to generate a TimeoutProcessor for a given view
	// Expected error returns during normal operations:
	//  * model.ErrViewForUnknownEpoch no epoch containing the given view is known
	// All other errors should be treated as exceptions.
	Create(view uint64) (TimeoutProcessor, error)
}
