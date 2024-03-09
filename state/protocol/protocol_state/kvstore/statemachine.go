package kvstore

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
)

// ProcessingStateMachine is a dedicated structure that encapsulates all logic for evolving KV store, based on the content
// of a new block.
// ProcessingStateMachine processes a subset of service events that are relevant for the KV store, and ignores all other events.
// Each relevant event is validated before it is applied to the KV store.
// All updates are applied to a copy of parent KV store, so parent KV store is not modified.
// A separate instance should be created for each block to process the updates therein.
type ProcessingStateMachine struct {
	view        uint64
	parentState protocol_state.KVStoreReader
	state       protocol_state.KVStoreAPI
	params      protocol.GlobalParams
}

var _ protocol_state.KeyValueStoreStateMachine = (*ProcessingStateMachine)(nil)

// NewProcessingStateMachine creates a new key-value store state machine.
// The underlying state is a clone of the parent state to ensure that the parent state is not modified.
func NewProcessingStateMachine(
	view uint64,
	params protocol.GlobalParams,
	parentState protocol_state.KVStoreReader,
	mutator protocol_state.KVStoreAPI,
) *ProcessingStateMachine {
	return &ProcessingStateMachine{
		view:        view,
		parentState: parentState,
		state:       mutator.Clone(),
		params:      params,
	}
}

// Build returns updated key-value store model, state ID and a flag indicating if there were any changes.
func (m *ProcessingStateMachine) Build() (updatedState protocol_state.KVStoreReader, stateID flow.Identifier, hasChanges bool) {
	updatedState = m.state.Clone()
	stateID = updatedState.ID()
	hasChanges = stateID != m.parentState.ID()
	return
}

// ProcessUpdate updates the current state of key-value store.
// KeyValueStoreStateMachine captures only a subset of all service events, those that are relevant for the KV store. All other events are ignored.
// Implementors MUST ensure KeyValueStoreStateMachine is left in functional state if an invalid service event has been supplied.
// Expected errors indicating that we have observed and invalid service event from protocol's point of view.
//   - `protocol.InvalidServiceEventError` - if the service event is invalid for the current protocol state.
func (m *ProcessingStateMachine) ProcessUpdate(update *flow.ServiceEvent) error {
	switch update.Type {
	case flow.ServiceEventProtocolStateVersionUpgrade:
		versionUpgrade, ok := update.Event.(*flow.ProtocolStateVersionUpgrade)
		if !ok {
			return fmt.Errorf("internal invalid type for ProtocolStateVersionUpgrade: %T", update.Event)
		}

		// To switch the protocol version, replica needs to process a block with a view >= activation view.
		// But we cannot activate a new version till the block containing the seal is finalized because we cannot switch between chain forks.
		// The problem is that finality is local to each node due to the nature of the consensus algorithm itself.
		// We would like to guarantee that all nodes switch the protocol version at exactly the same block.
		// To guarantee that all nodes switch the protocol version at exactly the same block, we require that the
		// activation view is higher than the sealing view + Δ when accepting the event. Δ represents the finalization lag
		// to give time for replicas to finalize the block containing the seal for the version upgrade event.
		// When replica reaches activation view and the latest finalized protocol state knows about the version upgrade,
		// then it's safe to switch the protocol version.
		if m.view+m.params.EpochCommitSafetyThreshold() >= versionUpgrade.ActiveView {
			return protocol.NewInvalidServiceEventErrorf("invalid protocol state version upgrade view %d -> %d: %w",
				m.view+m.params.EpochCommitSafetyThreshold(), versionUpgrade.ActiveView, ErrInvalidActivationView)
		}

		if m.parentState.GetProtocolStateVersion() >= versionUpgrade.NewProtocolStateVersion {
			return protocol.NewInvalidServiceEventErrorf("invalid protocol state version upgrade %d -> %d: %w",
				m.parentState.GetProtocolStateVersion(), versionUpgrade.NewProtocolStateVersion, ErrInvalidUpgradeVersion)
		}

		activator := &protocol_state.ViewBasedActivator[uint64]{
			Data:           versionUpgrade.NewProtocolStateVersion,
			ActivationView: versionUpgrade.ActiveView,
		}
		m.state.SetVersionUpgrade(activator)

	default:
		return nil
	}

	return nil
}

// View returns the view that is associated with this KeyValueStoreStateMachine.
// The view of the KeyValueStoreStateMachine equals the view of the block carrying the respective updates.
func (m *ProcessingStateMachine) View() uint64 {
	return m.view
}

// ParentState returns parent state that is associated with this state machine.
func (m *ProcessingStateMachine) ParentState() protocol_state.KVStoreReader {
	return m.parentState
}
