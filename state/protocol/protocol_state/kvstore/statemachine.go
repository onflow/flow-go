package kvstore

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/protocol_state"
)

type ProcessingStateMachine struct {
	view        uint64
	parentState protocol_state.Reader
	state       protocol_state.API
}

var _ protocol_state.KeyValueStoreStateMachine = (*ProcessingStateMachine)(nil)

func NewProcessingStateMachine(view uint64) *ProcessingStateMachine {
	return &ProcessingStateMachine{
		view: view,
	}
}

func (m *ProcessingStateMachine) Build() (updatedState protocol_state.Reader, stateID flow.Identifier, hasChanges bool) {
	//TODO implement me
	panic("implement me")
}

func (m *ProcessingStateMachine) ProcessUpdate(update *flow.ServiceEvent) error {
	switch update.Type {
	case flow.ServiceEventProtocolStateVersionUpgrade:
		versionUpgrade, ok := update.Event.(*flow.ProtocolStateVersionUpgrade)
		if !ok {
			return fmt.Errorf("internal invalid type for ProtocolStateVersionUpgrade: %T", update.Event)
		}

		if m.parentState.GetProtocolStateVersion() >= versionUpgrade.NewProtocolStateVersion {
			return fmt.Errorf("invalid protocol state version upgrade: %d -> %d",
				m.parentState.GetVersionUpgrade(), versionUpgrade.NewProtocolStateVersion)
		}

		if m.view >= versionUpgrade.ActiveView {
			return fmt.Errorf("invalid protocol state version upgrade: view %d -> %d",
				m.view, versionUpgrade.ActiveView)
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
