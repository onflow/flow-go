package dkg

import (
	"fmt"
	"sync"
)

// State captures the state of a DKG engine
type State uint32

const (
	Init State = iota
	Phase1
	Phase2
	Phase3
	End
	Shutdown
)

// String returns the string representation of a State
func (s State) String() string {
	switch s {
	case Init:
		return "Init"
	case Phase1:
		return "Phase1"
	case Phase2:
		return "Phase2"
	case Phase3:
		return "Phase3"
	case End:
		return "End"
	case Shutdown:
		return "Shutdown"
	default:
		return fmt.Sprintf("Unknown %d", s)
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
