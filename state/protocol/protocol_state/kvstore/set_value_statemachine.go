package kvstore

import (
	"errors"
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/state/protocol/protocol_state/helper"
)

// SetValueStateMachine encapsulates the logic for evolving sub-state of KV store by setting particular values.
// Specifically, it consumes ProtocolStateVersionUpgrade ServiceEvent that are sealed by the candidate block
// (possibly still under construction) with the given view.
// Each relevant event is validated before it is applied to the KV store.
// All updates are applied to a copy of parent KV store, so parent KV store is not modified.
// A separate instance should be created for each block to process the updates therein.
type SetValueStateMachine struct {
	helper.BaseKeyValueStoreStateMachine
	telemetry protocol_state.StateMachineTelemetryConsumer
}

var _ protocol_state.KeyValueStoreStateMachine = (*SetValueStateMachine)(nil)

// NewSetValueStateMachine creates a new state machine to update a specific sub-state of the KV Store.
func NewSetValueStateMachine(
	telemetry protocol_state.StateMachineTelemetryConsumer,
	candidateView uint64,
	parentState protocol.KVStoreReader,
	mutator protocol_state.KVStoreMutator,
) *SetValueStateMachine {
	return &SetValueStateMachine{
		BaseKeyValueStoreStateMachine: helper.NewBaseKeyValueStoreStateMachine(candidateView, parentState, mutator),
		telemetry:                     telemetry,
	}
}

// EvolveState applies the state change(s) on sub-state P for the candidate block (under construction).
// Implementation processes only relevant service events and ignores all other events.
// No errors are expected during normal operations.
func (m *SetValueStateMachine) EvolveState(orderedUpdates []flow.ServiceEvent) error {
	for _, update := range orderedUpdates {
		switch update.Type {
		case flow.ServiceEventSetEpochExtensionViewCount:
			setEpochExtensionViewCount, ok := update.Event.(*flow.SetEpochExtensionViewCount)
			if !ok {
				return fmt.Errorf("internal invalid type for SetEpochExtensionViewCount: %T", update.Event)
			}

			m.telemetry.OnServiceEventReceived(update)
			err := m.Mutator.SetEpochExtensionViewCount(setEpochExtensionViewCount.Value)
			if err != nil {
				if errors.Is(err, ErrInvalidValue) {
					m.telemetry.OnInvalidServiceEvent(update,
						protocol.NewInvalidServiceEventErrorf("invalid value %v for SetEpochExtensionViewCount: %s",
							setEpochExtensionViewCount.Value, err.Error()))
					continue
				}
				return fmt.Errorf("unexpected error when processing SetEpochExtensionViewCount: %w", err)
			}
			m.telemetry.OnServiceEventProcessed(update)

		// Service events not explicitly expected are ignored
		default:
			continue
		}
	}

	return nil
}
