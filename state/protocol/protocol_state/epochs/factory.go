package epochs

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/storage"
)

type EpochStateMachineFactory struct {
	params          protocol.GlobalParams
	setups          storage.EpochSetups
	commits         storage.EpochCommits
	protocolStateDB storage.ProtocolState
}

func NewEpochStateMachineFactory(
	params protocol.GlobalParams,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	protocolStateDB storage.ProtocolState) *EpochStateMachineFactory {
	return &EpochStateMachineFactory{
		params:          params,
		setups:          setups,
		commits:         commits,
		protocolStateDB: protocolStateDB,
	}
}

func (f *EpochStateMachineFactory) Create(
	candidate *flow.Header,
	parentState protocol_state.KVStoreReader,
	mutator protocol_state.KVStoreMutator,
) (protocol_state.KeyValueStoreStateMachine, error) {
	return NewEpochStateMachine(
		candidate,
		f.params,
		f.setups,
		f.commits,
		f.protocolStateDB,
		parentState,
		mutator,
	)
}
