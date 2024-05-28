package epochs

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/storage"
)

// EpochStateMachineFactory is a factory for creating EpochStateMachine instances.
// It holds all the necessary data to create a new instance of EpochStateMachine.
type EpochStateMachineFactory struct {
	params               protocol.GlobalParams
	setups               storage.EpochSetups
	commits              storage.EpochCommits
	epochProtocolStateDB storage.ProtocolState
}

var _ protocol_state.KeyValueStoreStateMachineFactory = (*EpochStateMachineFactory)(nil)

func NewEpochStateMachineFactory(
	params protocol.GlobalParams,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	epochProtocolStateDB storage.ProtocolState,
) *EpochStateMachineFactory {
	return &EpochStateMachineFactory{
		params:               params,
		setups:               setups,
		commits:              commits,
		epochProtocolStateDB: epochProtocolStateDB,
	}
}

// Create creates a new instance of an underlying type that operates on KV Store and is created for a specific candidate block.
// No errors are expected during normal operations.
func (f *EpochStateMachineFactory) Create(candidateView uint64, parentBlockID flow.Identifier, parentState protocol.KVStoreReader, mutator protocol_state.KVStoreMutator) (protocol_state.KeyValueStoreStateMachine, error) {
	return NewEpochStateMachine(
		candidateView,
		parentBlockID,
		f.params,
		f.setups,
		f.commits,
		f.epochProtocolStateDB,
		parentState,
		mutator,
		func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (StateMachine, error) {
			return NewHappyPathStateMachine(candidateView, parentState)
		},
		func(candidateView uint64, parentState *flow.RichProtocolStateEntry) (StateMachine, error) {
			return NewFallbackStateMachine(candidateView, parentState), nil
		},
	)
}
