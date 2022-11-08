package hotstuff

import (
	"context"
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// PartialTcCreated represents a notification emitted by the TimeoutProcessor component,
// whenever it has collected TimeoutObjects from a superminority
// of consensus participants for a specific view. Along with the view, it
// reports the newest QC and TC (for previous view) discovered during
// timeout collection. Per convention, the newest QC is never nil, while
// the TC for the previous view might be nil.
type PartialTcCreated struct {
	View       uint64
	NewestQC   *flow.QuorumCertificate
	LastViewTC *flow.TimeoutCertificate
}

// EventHandler runs a state machine to process proposals, QC and local timeouts.
// Not concurrency safe.
type EventHandler interface {

	// OnReceiveQc processes a valid qc constructed by internal vote aggregator or discovered in TimeoutObject.
	// All inputs should be validated before feeding into this function. Assuming trusted data.
	// No errors are expected during normal operation.
	OnReceiveQc(qc *flow.QuorumCertificate) error

	// OnReceiveTc processes a valid tc constructed by internal timeout aggregator, discovered in TimeoutObject or
	// broadcast over the network.
	// All inputs should be validated before feeding into this function. Assuming trusted data.
	// No errors are expected during normal operation.
	OnReceiveTc(tc *flow.TimeoutCertificate) error

	// OnReceiveProposal processes a block proposal received from another HotStuff
	// consensus participant.
	// All inputs should be validated before feeding into this function. Assuming trusted data.
	// No errors are expected during normal operation.
	OnReceiveProposal(proposal *model.Proposal) error

	// OnLocalTimeout handles a local timeout event by creating a model.TimeoutObject and broadcasting it.
	// No errors are expected during normal operation.
	OnLocalTimeout() error

	// OnPartialTcCreated handles notification produces by the internal timeout aggregator. If the notification is for the current view,
	// a corresponding model.TimeoutObject is broadcast to the consensus committee.
	// No errors are expected during normal operation.
	OnPartialTcCreated(partialTC *PartialTcCreated) error

	// TimeoutChannel returns a channel that sends a signal on timeout.
	TimeoutChannel() <-chan time.Time

	// Start starts the event handler.
	// No errors are expected during normal operation.
	// CAUTION: EventHandler is not concurrency safe. The Start method must
	// be executed by the same goroutine that also calls the other business logic
	// methods, or concurrency safety has to be implemented externally.
	Start(ctx context.Context) error
}
