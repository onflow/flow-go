package stdmap

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
)

// Times implements the times memory pool used to store time.Times for an idetifier to track transaction metrics in
// access nodes
type Times struct {
	*Backend[flow.Identifier, time.Time]
}

// NewTimes creates a new memory pool for times
func NewTimes(limit uint) (*Times, error) {
	t := &Times{
		Backend: NewBackend[flow.Identifier, time.Time](WithLimit[flow.Identifier, time.Time](limit)),
	}

	return t, nil
}

// Add adds a time to the mempool.
func (t *Times) Add(id flow.Identifier, ti time.Time) bool {
	return t.Backend.Add(id, ti)
}

// ByID returns the time with the given ID from the mempool.
func (t *Times) ByID(id flow.Identifier) (time.Time, bool) {
	ti, exists := t.Backend.ByID(id)
	if !exists {
		return time.Time{}, false
	}

	return ti, true
}

// Remove removes the time with the given ID.
func (t *Times) Remove(id flow.Identifier) bool {
	return t.Backend.Remove(id)
}
