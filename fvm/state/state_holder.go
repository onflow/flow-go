package state

import (
	"math"
)

// StateHolder provides active states
// and facilitates common state management operations
// in order to make services such as accounts not worry about
// the state it is recommended that such services wraps
// a state manager instead of a state itself.
type StateHolder struct {
	enforceLimits bool
	startState    *State
	activeState   *State
}

// NewStateHolder constructs a new state manager
func NewStateHolder(startState *State) *StateHolder {
	return &StateHolder{
		enforceLimits: true,
		startState:    startState,
		activeState:   startState,
	}
}

// State returns the active state
func (s *StateHolder) State() *State {
	return s.activeState
}

// SetActiveState sets active state
func (s *StateHolder) SetActiveState(st *State) {
	s.activeState = st
}

// DisableMemoryAndInteractionLimits sets the memory and interaction limits to
// MaxUint64, effectively disabling these limits
func (s *StateHolder) DisableMemoryAndInteractionLimits() {
	s.activeState.SetTotalMemoryLimit(math.MaxUint64)
	s.activeState.SetTotalInteractionLimit(math.MaxUint64)
}

// NewChild constructs a new child of active state
// and set it as active state and return it
// this is basically a utility function for common
// operations
func (s *StateHolder) NewChild() *State {
	child := s.activeState.NewChild()
	s.activeState = child
	return s.activeState
}

// EnableAllLimitEnforcements enables all the limits
func (s *StateHolder) EnableAllLimitEnforcements() {
	s.enforceLimits = true
}

// DisableAllLimitEnforcements disables all the limits
func (s *StateHolder) DisableAllLimitEnforcements() {
	s.enforceLimits = false
}

// EnforceLimits returns if limits should be enforced or not
func (s *StateHolder) EnforceLimits() bool {
	return s.enforceLimits
}
