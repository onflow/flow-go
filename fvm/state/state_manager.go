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
	parents     map[*State]*State
}

// NewStateManager constructs a new state manager
func NewStateManager(startState *State) *StateManager {
	return &StateManager{
		startState:  startState,
		activeState: startState,
		parents:     make(map[*State]*State),
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
	return s.activeState.MergeState(other)
}

// Nest creates a child state and set it as the active state
func (s *StateManager) Nest() {
	new := s.activeState.NewChild()
	s.parents[new] = s.activeState
	s.activeState = new
}

// RollUpWithMerge merges the active state into its parent and set the parent as the
// new active state.
func (s *StateManager) RollUpWithMerge() error {
	if s.parents[s.activeState] == nil {
		return fmt.Errorf("parent not exist for this state")
	}

	err := s.parents[s.activeState].MergeState(s.activeState)
	if err != nil {
		return err
	}

	s.activeState = s.parents[s.activeState]
	return nil
}

// RollUpNoMerge ignores the current active state
// and sets the parent as the active state
func (s *StateManager) RollUpNoMerge() error {
	if s.parents[s.activeState] == nil {
		return fmt.Errorf("parent not exist for this state")
	}
	s.activeState = s.parents[s.activeState]
	return nil
}
