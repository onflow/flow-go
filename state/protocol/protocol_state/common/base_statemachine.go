package common

import (
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

// BaseKeyValueStoreStateMachine implements a subset of the KeyValueStoreStateMachine interface which is usually common
// to all state machines that operate on the KV store.
// All implementors can override the methods as needed.
type BaseKeyValueStoreStateMachine struct {
	candidateView uint64
	parentState   protocol.KVStoreReader
	EvolvingState protocol_state.KVStoreMutator
}

// NewBaseKeyValueStoreStateMachine creates a new instance of BaseKeyValueStoreStateMachine.
func NewBaseKeyValueStoreStateMachine(
	candidateView uint64,
	parentState protocol.KVStoreReader,
	evolvingState protocol_state.KVStoreMutator,
) BaseKeyValueStoreStateMachine {
	return BaseKeyValueStoreStateMachine{
		candidateView: candidateView,
		parentState:   parentState,
		EvolvingState: evolvingState,
	}
}

// Build is a no-op by default. If a state machine needs to persist data, it should override this method.
func (m *BaseKeyValueStoreStateMachine) Build() (*transaction.DeferredBlockPersist, error) {
	return transaction.NewDeferredBlockPersist(), nil
}

// View returns the view associated with this state machine.
// The view of the state machine equals the view of the block carrying the respective updates.
func (m *BaseKeyValueStoreStateMachine) View() uint64 {
	return m.candidateView
}

// ParentState returns parent state associated with this state machine.
func (m *BaseKeyValueStoreStateMachine) ParentState() protocol.KVStoreReader {
	return m.parentState
}
