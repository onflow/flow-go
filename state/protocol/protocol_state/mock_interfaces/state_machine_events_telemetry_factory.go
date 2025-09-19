package mockinterfaces

import "github.com/onflow/flow-go/state/protocol/protocol_state"

// ExecForkActor allows to create a mock for the ExecForkActor callback
type StateMachineEventsTelemetryFactory interface {
	Execute(candidateView uint64) protocol_state.StateMachineTelemetryConsumer
}
