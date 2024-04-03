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

// Create creates a new instance of an underlying type that operates on KV Store and is created for a specific candidate block.
// No errors are expected during normal operations.
func (f *PSVersionUpgradeStateMachineFactory) Create(view uint64, parentID flow.Identifier, parentState protocol_state.KVStoreReader, mutator protocol_state.KVStoreMutator) (protocol_state.KeyValueStoreStateMachine, error) {
	return NewPSVersionUpgradeStateMachine(view, f.params, parentState, mutator), nil
}
