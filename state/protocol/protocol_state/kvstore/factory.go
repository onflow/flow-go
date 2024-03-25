package kvstore

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
)

type PSVersionUpgradeStateMachineFactory struct {
	params protocol.GlobalParams
}

func NewPSVersionUpgradeStateMachineFactory(params protocol.GlobalParams) *PSVersionUpgradeStateMachineFactory {
	return &PSVersionUpgradeStateMachineFactory{
		params: params,
	}
}

func (f *PSVersionUpgradeStateMachineFactory) Create(
	candidate *flow.Header,
	parentState protocol_state.KVStoreReader,
	mutator protocol_state.KVStoreMutator,
) (protocol_state.KeyValueStoreStateMachine, error) {
	return NewPSVersionUpgradeStateMachine(candidate.View, f.params, parentState, mutator), nil
}
