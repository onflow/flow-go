package kvstore

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
)

// PSVersionUpgradeStateMachineFactory is a factory for creating PSVersionUpgradeStateMachine instances.
type PSVersionUpgradeStateMachineFactory struct {
	params protocol.GlobalParams
}

var _ protocol_state.KeyValueStoreStateMachineFactory = (*PSVersionUpgradeStateMachineFactory)(nil)

func NewPSVersionUpgradeStateMachineFactory(params protocol.GlobalParams) *PSVersionUpgradeStateMachineFactory {
	return &PSVersionUpgradeStateMachineFactory{
		params: params,
	}
}

// Create instantiates a new PSVersionUpgradeStateMachine, which processes ProtocolStateVersionUpgrade ServiceEvent
// that are sealed by the candidate block (possibly still under construction) with the given view.
// No errors are expected during normal operations.
func (f *PSVersionUpgradeStateMachineFactory) Create(candidateView uint64, parentID flow.Identifier, parentState protocol.KVStoreReader, mutator protocol_state.KVStoreMutator) (protocol_state.KeyValueStoreStateMachine, error) {
	return NewPSVersionUpgradeStateMachine(candidateView, f.params, parentState, mutator), nil
}
