package stdmap

import (
	chunkmodels "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

// Assignments implements the chunk assignment memory pool.
type Assignments struct {
	*Backend[flow.Identifier, *chunkmodels.Assignment]
}

// NewAssignments creates a new memory pool for Assignments.
func NewAssignments(limit uint) (*Assignments, error) {
	a := &Assignments{
		Backend: NewBackend[flow.Identifier, *chunkmodels.Assignment](WithLimit[flow.Identifier, *chunkmodels.Assignment](limit)),
	}
	return a, nil
}

// Has checks whether the Assignment with the given hash is currently in
// the memory pool.
func (a *Assignments) Has(assignmentID flow.Identifier) bool {
	return a.Backend.Has(assignmentID)

}

// ByID retrieves the chunk assignment from mempool based on provided ID
func (a *Assignments) ByID(assignmentID flow.Identifier) (*chunkmodels.Assignment, bool) {
	assignment, exists := a.Backend.ByID(assignmentID)
	if !exists {
		return nil, false
	}
	return assignment, true
}

// Add adds an Assignment to the mempool.
func (a *Assignments) Add(assignmentID flow.Identifier, assignment *chunkmodels.Assignment) bool {
	return a.Backend.Add(assignmentID, assignment)
}

// Remove will remove the given Assignment from the memory pool; it will
// return true if the Assignment was known and removed.
func (a *Assignments) Remove(assignmentID flow.Identifier) bool {
	return a.Backend.Remove(assignmentID)
}

// Size will return the current size of the memory pool.
func (a *Assignments) Size() uint {
	return a.Backend.Size()
}

// All returns all chunk data packs from the pool.
func (a *Assignments) All() []*chunkmodels.Assignment {
	entities := a.Backend.All()
	assignments := make([]*chunkmodels.Assignment, 0, len(entities))
	for _, assignment := range entities {
		assignments = append(assignments, assignment)
	}

	return assignments
}
