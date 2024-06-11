package protocol_state

import (
	"github.com/onflow/flow-go/model/flow"
)

type StateMachineEventsConsumer interface {
	OnInvalidServiceEvent(event flow.ServiceEvent, err error)

	OnServiceEventReceived(event flow.ServiceEvent)

	OnServiceEventProcessed(event flow.ServiceEvent)
}
