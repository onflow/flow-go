package state

type StateManager struct {
	startState  *State
	activeState *State
}

func NewStateManager(startState *State) *StateManager {
	return &StateManager{
		startState:  startState,
		activeState: startState,
	}
}

func (s *StateManager) Nest() {
	s.activeState = s.activeState.NewChild()
}

func (s *StateManager) RollUp(merge bool) {
	if merge {
		s.activeState.parent.MergeState(s.activeState)
	}
	// otherwise ignore for now
	if s.activeState.parent != nil {
		s.activeState = s.activeState.parent
	}
}

func (s *StateManager) ApplyStartStateToLedger() error {
	return s.startState.ApplyDeltaToLedger()
}

func (s *StateManager) State() *State {
	return s.activeState
}
