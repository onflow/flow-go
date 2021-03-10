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

// RollUp merges the active state into its parent and set the parent as the
// new active state (if any parents), if merge is set to true, it will merge
// the delta of the state and if false it ignores the child.
// if mergeTouches is set it merges the touches to the parent (useful for exec failure cases)
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

// RollUpAll calls the roll up until we reach to the start state
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

// ApplyStartStateToLedger applies start state deltas into the ledger
// note that you need to make sure all of the deltas are merged into
// the start state through roll ups before caling this
func (s *StateManager) ApplyStartStateToLedger() error {
	return s.startState.ApplyDeltaToLedger()
}
