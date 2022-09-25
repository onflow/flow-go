package ingestion

import (
	"fmt"
	"sync"
)

type StopAtHeight struct {
	sync.RWMutex
	height   uint64
	crash    bool
	stopping bool // if stopping process has started, we disallow changes
	set      bool // whether stop at height has been set at all, it its envisioned most of the time it won't
}

// NewStopAtHeight creates new empty StopAtHegiht
func NewStopAtHeight() *StopAtHeight {
	return &StopAtHeight{
		stopping: false,
	}
}

// Get returns
// boolean indicating if value is set - for easier comparisons, since its envisions most of the time this struct will be empty
// height and crash values
func (s *StopAtHeight) Get() (bool, uint64, bool) {
	s.RLock()
	defer s.RUnlock()
	return s.set, s.height, s.crash
}

// Try runs function f with current values of height and crash if the values are set
// f should return true if it started a process of stopping, so no further changes will
// be accepted.
// Try returns whatever f returned, or false if f has not been called
func (s *StopAtHeight) Try(f func(uint64, bool) bool) bool {
	s.Lock()
	defer s.Unlock()

	if !s.set {
		return false
	}

	started := f(s.height, s.crash)

	if started {
		s.stopping = true
	}

	return started
}

// Set sets new values and return old ones
func (s *StopAtHeight) Set(height uint64, crash bool) (bool, uint64, bool, error) {
	s.Lock()
	defer s.Unlock()

	oldSet := s.set
	oldHeight := s.height
	oldCrash := s.crash

	if s.stopping {
		return oldSet, oldHeight, oldCrash, fmt.Errorf("cannot update stop height, stopping already in progress for height %d with crash=%t", oldHeight, oldCrash)
	}

	s.set = true
	s.height = height
	s.crash = crash

	return oldSet, oldHeight, oldCrash, nil
}

func (s *StopAtHeight) Unset() {
	s.Lock()
	defer s.Unlock()
	s.set = false
	s.height = 0
	s.crash = false
}
