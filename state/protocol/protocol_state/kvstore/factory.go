package kvstore

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
)

// PSVersionUpgradeStateMachineFactory is a factory for creating PSVersionUpgradeStateMachine instances.
type PSVersionUpgradeStateMachineFactory struct {
	telemetry protocol_state.StateMachineTelemetryConsumer
}

var _ protocol_state.KeyValueStoreStateMachineFactory = (*PSVersionUpgradeStateMachineFactory)(nil)

// NewPSVersionUpgradeStateMachineFactory returns a factory for instantiating PSVersionUpgradeStateMachines.
// The created state machines report their operations to the provided telemetry consumer.
func NewPSVersionUpgradeStateMachineFactory(telemetry protocol_state.StateMachineTelemetryConsumer) *PSVersionUpgradeStateMachineFactory {
	return &PSVersionUpgradeStateMachineFactory{
		telemetry: telemetry,
	}
}

// Create instantiates a new PSVersionUpgradeStateMachine, which processes ProtocolStateVersionUpgrade ServiceEvents
// that are sealed by the candidate block (possibly still under construction) with the given view.
// No errors are expected during normal operations.
func (f *PSVersionUpgradeStateMachineFactory) Create(candidateView uint64, _ flow.Identifier, parentState protocol.KVStoreReader, mutator protocol_state.KVStoreMutator) (protocol_state.KeyValueStoreStateMachine, error) {
	return NewPSVersionUpgradeStateMachine(f.telemetry, candidateView, parentState, mutator), nil
}

// SetValueStateMachineFactory is a factory for creating SetValueStateMachine instances.
type SetValueStateMachineFactory struct {
	telemetry protocol_state.StateMachineTelemetryConsumer
}

var _ protocol_state.KeyValueStoreStateMachineFactory = (*SetValueStateMachineFactory)(nil)

// NewSetValueStateMachineFactory returns a factory for instantiating SetValueStateMachines.
// The created state machines report their operations to the provided telemetry consumer.
func NewSetValueStateMachineFactory(telemetry protocol_state.StateMachineTelemetryConsumer) *SetValueStateMachineFactory {
	return &SetValueStateMachineFactory{telemetry: telemetry}
}

// Create creates a new instance of SetValueStateMachine.
// No errors are expected during normal operations.
func (f *SetValueStateMachineFactory) Create(candidateView uint64, _ flow.Identifier, parentState protocol.KVStoreReader, mutator protocol_state.KVStoreMutator) (protocol_state.KeyValueStoreStateMachine, error) {
	return NewSetValueStateMachine(f.telemetry, candidateView, parentState, mutator), nil
}
