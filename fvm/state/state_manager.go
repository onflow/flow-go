package state

import "fmt"

// StateManager provides active states
// and facilitates common state management operations
// in order to make services such as accounts not worry about
// the state it is recommended that such services wraps
// an state manager instead of an state itself.
type StateManager struct {
	startState  *State
	activeState *State
}

// NewStateManager constructs a new state manager
func NewStateManager(startState *State) *StateManager {
	return &StateManager{
		startState:  startState,
		activeState: startState,
	}
}

// State returns the active state
func (s *StateManager) State() *State {
	return s.activeState
}

// StartState returns the start state
func (s *StateManager) StartState() *State {
	return s.startState
}

// MergeStateIntoActiveState allows to merge any given state into the active state
func (s *StateManager) MergeStateIntoActiveState(other *State) error {
	return s.activeState.MergeAnyState(other)
}

// Nest creates a child state and set it as the active state
func (s *StateManager) Nest() {
	s.activeState = s.activeState.NewChild()
}

// RollUpWithMerge merges the active state into its parent and set the parent as the
// new active state.
func (s *StateManager) RollUpWithMerge() error {
	if s.activeState.parent == nil {
		return fmt.Errorf("parent not exist for this state")
	}

	err := s.activeState.parent.MergeState(s.activeState)
	if err != nil {
		return err
	}

	s.activeState = s.activeState.parent
	return nil
}

// RollUpWithTouchMergeOnly merges the active state's touches into its parent and set the parent as the
// new active state. useful for failed transactions
func (s *StateManager) RollUpWithTouchMergeOnly() error {
	if s.activeState.parent == nil {
		return fmt.Errorf("parent not exist for this state")
	}

	err := s.activeState.parent.MergeTouchLogs(s.activeState)
	if err != nil {
		return err
	}

	s.activeState = s.activeState.parent
	return nil
}

// RollUpNoMerge ignores the current active state
// and sets the parent as the active state
func (s *StateManager) RollUpNoMerge() error {
	if s.activeState.parent == nil {
		return fmt.Errorf("parent not exist for this state")
	}
	s.activeState = s.activeState.parent
	return nil
}

// ApplyStartStateToLedger applies start state deltas into the ledger
// note that you need to make sure all of the deltas are merged into
// the start state through roll ups before caling this
func (s *StateManager) ApplyStartStateToLedger() error {
	return s.startState.ApplyDeltaToLedger()
}
