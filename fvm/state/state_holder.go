package state

import (
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/meter/noop"
)

// StateHolder provides active states
// and facilitates common state management operations
// in order to make services such as accounts not worry about
// the state it is recommended that such services wraps
// a state manager instead of a state itself.
type StateHolder struct {
	mh meter.MeteringHandler

	startState  *State
	activeState *State
}

type StateHolderOption func(*StateHolder)

// NewStateHolder constructs a new state manager
func NewStateHolder(startState *State, options ...StateHolderOption) *StateHolder {
	sh := &StateHolder{
		mh: noop.MeteringHandler{},

		startState:  startState,
		activeState: startState,
	}
	for _, option := range options {
		option(sh)
	}
	return sh
}

func WithMeteringHandler(mh meter.MeteringHandler) StateHolderOption {
	return func(holder *StateHolder) {
		holder.mh = mh
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

// NewChild constructs a new child of active state
// and set it as active state and return it
// this is basically a utility function for common
// operations
func (s *StateHolder) NewChild() *State {
	child := s.activeState.NewChild()
	s.activeState = child
	return s.activeState
}

func (s *StateHolder) MeteringHandler() meter.MeteringHandler {
	return s.mh
}
