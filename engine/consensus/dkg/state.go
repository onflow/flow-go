package dkg

import (
	"sync/atomic"
)

// State captures the state of a DKG engine
type State uint32

const (
	Init State = iota
	Phase0
	Phase1
	Phase2
	Shutdown
)

// String returns the string representation of a State
func (s State) String() string {
	switch s {
	case Phase0:
		return "Phase0"
	case Phase1:
		return "Phase1"
	case Phase2:
		return "Phase2"
	case Shutdown:
		return "Shutdown"
	default:
		return "Unknown"
	}
}

// Manager wraps a State with get and set methods
type Manager struct {
	state State
}

// GetState returns the current state.
func (m *Manager) GetState() State {
	stateAddr := (*uint32)(&m.state)
	return State(atomic.LoadUint32(stateAddr))
}

// SetState sets the state.
func (m *Manager) SetState(s State) {
	stateAddr := (*uint32)(&m.state)
	atomic.StoreUint32(stateAddr, uint32(s))
}
