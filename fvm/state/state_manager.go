package state

import "fmt"

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

func (s *StateManager) State() *State {
	return s.activeState
}

func (s *StateManager) MergeStateIntoActiveState(other *State) error {
	return s.activeState.MergeAnyState(other)
}

func (s *StateManager) Nest() {
	s.activeState = s.activeState.NewChild()
}

func (s *StateManager) RollUp(merge bool, mergeTouches bool) error {
	var err error

	if s.activeState.parent == nil {
		return fmt.Errorf("parent not exist for this state")
	}

	// TODO merge the register touches
	if merge {
		err = s.activeState.parent.MergeState(s.activeState)
	} else {
		if mergeTouches {
			err = s.activeState.parent.MergeTouchLogs(s.activeState)
		}
	}
	if err != nil {
		return err
	}
	// otherwise ignore for now
	if s.activeState.parent != nil {
		s.activeState = s.activeState.parent
	}
	return nil
}

func (s *StateManager) RollUpAll(merge bool, mergeTouches bool) error {
	for {
		if s.activeState == s.startState || s.activeState.parent == nil {
			break
		}
		err := s.RollUp(merge, mergeTouches)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *StateManager) ApplyStartStateToLedger() error {
	return s.startState.ApplyDeltaToLedger()
}
