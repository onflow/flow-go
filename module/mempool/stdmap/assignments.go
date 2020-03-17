package stdmap

import (
	"github.com/dapperlabs/flow-go/model/chunkassignment"
)

// Assignments implements the ChunkDataPack memory pool.
type Assignments struct {
	*Backend
}

// NewAssignments creates a new memory pool for Assignments.
func NewAssignments(limit uint) (*Assignments, error) {
	a := &Assignments{
		Backend: NewBackend(WithLimit(limit)),
	}
	return a, nil
}

// Add adds an Assignment to the mempool.
func (a *Assignments) Add(assignment *chunkassignment.Assignment) error {
	return a.Backend.Add(assignment)
}

// All returns all chunk data packs from the pool.
func (a *Assignments) All() []*chunkassignment.Assignment {
	entities := a.Backend.All()
	assignments := make([]*chunkassignment.Assignment, 0, len(entities))
	for _, entity := range entities {
		assignments = append(assignments, entity.(*chunkassignment.Assignment))
	}
	return assignments
}
