package kvstore

import (
	"errors"
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

type SetValueStateMachine struct {
	candidateView uint64
	parentState   protocol.KVStoreReader
	mutator       protocol_state.KVStoreMutator
	telemetry     protocol_state.StateMachineTelemetryConsumer
}

var _ protocol_state.KeyValueStoreStateMachine = (*SetValueStateMachine)(nil)

func (m *SetValueStateMachine) Build() (*transaction.DeferredBlockPersist, error) {
	return transaction.NewDeferredBlockPersist(), nil
}

func (m *SetValueStateMachine) EvolveState(orderedUpdates []flow.ServiceEvent) error {
	for _, update := range orderedUpdates {
		switch update.Type {
		case flow.ServiceEventSetEpochExtensionViewCount:
			setEpochExtensionViewCount, ok := update.Event.(*flow.SetEpochExtensionViewCount)
			if !ok {
				return fmt.Errorf("internal invalid type for SetEpochExtensionViewCount: %T", update.Event)
			}

			err := m.mutator.SetEpochExtensionViewCount(setEpochExtensionViewCount.Value)
			if err != nil {
				if errors.Is(err, ErrInvalidValue) {
					m.telemetry.OnInvalidServiceEvent(update,
						protocol.NewInvalidServiceEventErrorf("invalid value %v for SetEpochExtensionViewCount: %s",
							setEpochExtensionViewCount.Value, err.Error()))
					continue
				}
				return fmt.Errorf("unexpected error when processing SetEpochExtensionViewCount: %w", err)
			}

		// Service events not explicitly expected are ignored
		default:
			continue
		}
	}

	return nil
}

func (m *SetValueStateMachine) View() uint64 {
	return m.candidateView
}

func (m *SetValueStateMachine) ParentState() protocol.KVStoreReader {
	return m.parentState
}
