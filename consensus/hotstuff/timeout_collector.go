package hotstuff

import (
	"errors"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

var (
	TimeoutForIncompatibleViewError = errors.New("timeout for incompatible view")
)

// OnTCCreated is a callback which will be used by TimeoutCollector to submit a TC when it's able to create it
type OnTCCreated func(tc *flow.TimeoutCertificate)

// OnPartialTCCreated is a callback which will be used by TimeoutCollector to notify about collecting f+1 timeouts for
// some view
type OnPartialTCCreated func(view uint64)

// OnNewQCDiscovered is a callback which will be called to notify about new validated QC
type OnNewQCDiscovered func(*flow.QuorumCertificate)

// OnNewTCDiscovered is a callback which will be called to notify about new validated TC
type OnNewTCDiscovered func(*flow.TimeoutCertificate)

// TimeoutCollector collects all timeout objects for a specified view. On the happy path, it
// generates a TimeoutCertificate when enough timeouts have been collected.
type TimeoutCollector interface {
	// AddTimeout adds a timeout object to the collector
	// When f+1 TOs will be collected then callback for partial TC will be triggered,
	// after collecting 2f+1 TOs a TC will be created and passed to the EventLoop
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
	// * TimeoutForIncompatibleViewError - submitted timeout for incompatible view
	// * model.InvalidTimeoutError - submitted invalid timeout(invalid structure or invalid signature)
	// All other errors should be treated as exceptions.
	Process(timeout *model.TimeoutObject) error
}
