package hotstuff

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
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
