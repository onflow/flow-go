package state

// StateHolder provides active states
// and facilitates common state management operations
// in order to make services such as accounts not worry about
// the state it is recommended that such services wraps
// a state manager instead of a state itself.
type StateHolder struct {
	enforceInteractionLimits bool
	startState               *State
	activeState              *State
}

// NewStateHolder constructs a new state manager
func NewStateHolder(startState *State) *StateHolder {
	return &StateHolder{
		enforceInteractionLimits: true,
		startState:               startState,
		activeState:              startState,
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

// EnableLimitEnforcement sets that the interaction limit should be enforced
func (s *StateHolder) EnableLimitEnforcement() {
	s.enforceInteractionLimits = true
}

// DisableLimitEnforcement sets that the interaction limit should not be enforced
func (s *StateHolder) DisableLimitEnforcement() {
	s.enforceInteractionLimits = false
}

// NewChild constructs a new child of active state
// and set it as active state and return it
// this is basically a utility function for common
// operations
func (s *StateHolder) NewChild() *State {
	new := s.activeState.NewChild()
	s.activeState = new
	return s.activeState
}

// EnforceInteractionLimits returns if the interaction limits should be enforced or not
func (s *StateHolder) EnforceInteractionLimits() bool {
	return s.enforceInteractionLimits
}
