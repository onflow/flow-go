package protocol_state

import (
	"github.com/onflow/flow-go/model/flow"
)

// StateMachineTelemetryConsumer consumes notifications produced by OrthogonalStoreStateMachine instances. Any state machine
// that performs processing of service events should notify the consumer about the events it received, successfully processed or
// detected as invalid.
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
//   - handle repetition of the same events (with some processing overhead).
type StateMachineTelemetryConsumer interface {
	// OnInvalidServiceEvent notifications are produced when a service event is detected as invalid by the state machine.
	OnInvalidServiceEvent(event flow.ServiceEvent, err error)
	// OnServiceEventReceived notifications are produced when a service event is received by the state machine.
	OnServiceEventReceived(event flow.ServiceEvent)
	// OnServiceEventProcessed notifications are produced when a service event is successfully processed by the state machine.
	OnServiceEventProcessed(event flow.ServiceEvent)
}

// StateMachineEventsTelemetryFactory is a factory method for creating StateMachineTelemetryConsumer instances.
// It is useful for creating consumers that provide extra information about the context in which they are operating.
// State machines evolve state based on inputs in the form of service events that are incorporated in blocks. Thus, the consumer
// can be created based on the block carrying the service events.
type StateMachineEventsTelemetryFactory func(candidateView uint64) StateMachineTelemetryConsumer
