package controller

import (
	"sync"
)

// State captures the state of a DKG engine
type State uint32

const (
	Init State = iota
	Phase0
	Phase1
	Phase2
	End
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
	case End:
		return "End"
	case Shutdown:
		return "Shutdown"
	default:
		return "Unknown"
	}
}

// Manager wraps a State with get and set methods
type Manager struct {
	sync.Mutex
	state State
}

// GetState returns the current state.
func (m *Manager) GetState() State {
	m.Lock()
	defer m.Unlock()
	return m.state
}

// SetState sets the state.
func (m *Manager) SetState(s State) {
	m.Lock()
	defer m.Unlock()
	m.state = s
}
