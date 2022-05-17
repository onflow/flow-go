package state

// StateHolder provides active states
// and facilitates common state management operations
// in order to make services such as accounts not worry about
// the state it is recommended that such services wraps
// a state manager instead of a state itself.
type StateHolder struct {
	enforceMemoryLimits      bool
	EnforceComputationLimits bool
	enforceInteractionLimits bool
	payerIsServiceAccount    bool
	startState               *State
	activeState              *State
}

// NewStateHolder constructs a new state manager
func NewStateHolder(startState *State) *StateHolder {
	return &StateHolder{
		enforceMemoryLimits:      true,
		EnforceComputationLimits: true,
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

// SetPayerIsServiceAccount sets if the payer is the service account
func (s *StateHolder) SetPayerIsServiceAccount() {
	s.payerIsServiceAccount = true
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
	s.enforceInteractionLimits = true
	s.EnforceComputationLimits = true
	s.enforceMemoryLimits = true
}

// DisableAllLimitEnforcements disables all the limits
func (s *StateHolder) DisableAllLimitEnforcements() {
	s.enforceInteractionLimits = false
	s.EnforceComputationLimits = false
	s.enforceMemoryLimits = false
}

// EnforceInteractionLimits returns if the interaction limits should be enforced or not
func (s *StateHolder) EnforceInteractionLimits() bool {
	if s.payerIsServiceAccount {
		return false
	}
	return s.enforceInteractionLimits
}

// EnforceMemoryLimits returns if the memory limits should be enforced or not
func (s *StateHolder) EnforceMemoryLimits() bool {
	if s.payerIsServiceAccount {
		return false
	}
	return s.enforceMemoryLimits
}
