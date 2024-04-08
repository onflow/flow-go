package stdmap

import (
	"fmt"
	"time"

	"github.com/onflow/flow-go/model/flow"
)

// Times implements the times memory pool used to store time.Times for an idetifier to track transaction metrics in
// access nodes
type Times struct {
	*Backend
}

// NewTimes creates a new memory pool for times
func NewTimes(limit uint) (*Times, error) {
	t := &Times{
		Backend: NewBackend(WithLimit(limit)),
	}

	return t, nil
}

type Time struct {
	id flow.Identifier
	ti time.Time
}

func (t *Time) ID() flow.Identifier {
	return t.id
}

func (t *Time) Checksum() flow.Identifier {
	return t.id
}

// Add adds a time to the mempool.
func (t *Times) Add(id flow.Identifier, ti time.Time) bool {
	return t.Backend.Add(&Time{id, ti})
}

// ByID returns the time with the given ID from the mempool.
func (t *Times) ByID(id flow.Identifier) (time.Time, bool) {
	entity, exists := t.Backend.ByID(id)
	if !exists {
		return time.Time{}, false
	}
	tt, ok := entity.(*Time)
	if !ok {
		panic(fmt.Sprintf("invalid entity in times pool (%T)", entity))
	}
	return tt.ti, true
}

// Remove removes the time with the given ID.
func (t *Times) Remove(id flow.Identifier) bool {
	return t.Backend.Remove(id)
}
