package protocol_state

import (
	"github.com/onflow/flow-go/model/flow"
)

// StateMachineEventsConsumer consumers notifications produced by OrthogonalStoreStateMachine instances. Any state machine
// that performs processing of service events should notify the consumer about the events it received, successfully processed or
// detected as invalid.
type StateMachineEventsConsumer interface {
	// OnInvalidServiceEvent notifications are produced when a service event is detected as invalid by the state machine.
	OnInvalidServiceEvent(event flow.ServiceEvent, err error)
	// OnServiceEventReceived notifications are produced when a service event is received by the state machine.
	OnServiceEventReceived(event flow.ServiceEvent)
	// OnServiceEventProcessed notifications are produced when a service event is successfully processed by the state machine.
	OnServiceEventProcessed(event flow.ServiceEvent)
}

type StateMachineEventsConsumerFactoryMethod func(candidateView uint64) StateMachineEventsConsumer
