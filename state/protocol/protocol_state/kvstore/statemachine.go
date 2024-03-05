package kvstore

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
)

type ProcessingStateMachine struct {
	view        uint64
	parentState protocol_state.Reader
	state       protocol_state.API
	params      protocol.GlobalParams
}

var _ protocol_state.KeyValueStoreStateMachine = (*ProcessingStateMachine)(nil)

func NewProcessingStateMachine(
	view uint64,
	params protocol.GlobalParams,
	parentState protocol_state.Reader,
	mutator protocol_state.API,
) *ProcessingStateMachine {
	return &ProcessingStateMachine{
		view:        view,
		parentState: parentState,
		state:       mutator.Clone(),
		params:      params,
	}
}

func (m *ProcessingStateMachine) Build() (updatedState protocol_state.Reader, stateID flow.Identifier, hasChanges bool) {
	updatedState = m.state.Clone()
	stateID = updatedState.ID()
	hasChanges = stateID != m.parentState.ID()
	return
}

func (m *ProcessingStateMachine) ProcessUpdate(update *flow.ServiceEvent) error {
	switch update.Type {
	case flow.ServiceEventProtocolStateVersionUpgrade:
		versionUpgrade, ok := update.Event.(*flow.ProtocolStateVersionUpgrade)
		if !ok {
			return fmt.Errorf("internal invalid type for ProtocolStateVersionUpgrade: %T", update.Event)
		}

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

func (m *ProcessingStateMachine) View() uint64 {
	return m.view
}

func (m *ProcessingStateMachine) ParentState() protocol_state.Reader {
	return m.parentState
}
