package ingestion

import "sync"

type StopAtHeight struct {
	sync.RWMutex
	height uint64
	crash  bool
	set    bool
}

// NewStopAtHeight creates new empty StopAtHegiht
func NewStopAtHeight() *StopAtHeight {
	return &StopAtHeight{
		set: false,
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

// Set sets new values and return old ones
func (s *StopAtHeight) Set(height uint64, crash bool) (bool, uint64, bool) {
	s.Lock()
	defer s.Unlock()

	oldSet := s.set
	oldHeight := s.height
	oldCrash := s.crash

	s.set = true
	s.height = height
	s.crash = crash

	return oldSet, oldHeight, oldCrash
}

func (s *StopAtHeight) Unset() {
	s.Lock()
	defer s.Unlock()
	s.set = false
	s.height = 0
	s.crash = false
}
