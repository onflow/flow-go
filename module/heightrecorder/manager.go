package heightrecorder

import (
	"math"
	"sync"
)

type ProcessedHeightManager interface {
	RegisterHeightRecorder(ProcessedHeightRecorder)
}

type Manager struct {
	recorders []ProcessedHeightRecorder
	mu        sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) RegisterHeightRecorder(recorder ProcessedHeightRecorder) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.recorders = append(m.recorders, recorder)
}

// HighestCompleteHeight returns the highest height that all registered recorders have completed processing.
// i.e. it returns the lowest complete height from the set of recorders.
// If no recorders are registered or return a valid result, false is returned indicating no results are available.
func (m *Manager) HighestCompleteHeight() (uint64, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lowestHeight := uint64(math.MaxUint64)
	for _, recorder := range m.recorders {
		if height, ok := recorder.HighestCompleteHeight(); ok {
			if height < lowestHeight {
				lowestHeight = height
			}
		}
	}
	if lowestHeight == math.MaxUint64 {
		return 0, false
	}
	return lowestHeight, true
}
