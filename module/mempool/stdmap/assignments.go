package stdmap

import (
	chunkmodels "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
)

// Assignments implements the chunk assignment memory pool.
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

// Has checks whether the Assignment with the given hash is currently in
// the memory pool.
func (a *Assignments) Has(assignmentID flow.Identifier) bool {
	return a.Backend.Has(assignmentID)

}

// ByID retrieves the chunk assignment from mempool based on provided ID
func (a *Assignments) ByID(assignmentID flow.Identifier) (*chunkmodels.Assignment, bool) {
	entity, exists := a.Backend.ByID(assignmentID)
	if !exists {
		return nil, false
	}
	adp := entity.(*chunkmodels.AssignmentDataPack)
	return adp.Assignment(), true
}

// Add adds an Assignment to the mempool.
func (a *Assignments) Add(fingerprint flow.Identifier, assignment *chunkmodels.Assignment) bool {
	return a.Backend.Add(chunkmodels.NewAssignmentDataPack(fingerprint, assignment))
}

// Rem will remove the given Assignment from the memory pool; it will
// return true if the Assignment was known and removed.
func (a *Assignments) Rem(assignmentID flow.Identifier) bool {
	return a.Backend.Rem(assignmentID)
}

// Size will return the current size of the memory pool.
func (a *Assignments) Size() uint {
	return a.Backend.Size()
}

// All returns all chunk data packs from the pool.
func (a *Assignments) All() []*chunkmodels.Assignment {
	entities := a.Backend.All()
	assignments := make([]*chunkmodels.Assignment, 0, len(entities))
	for _, entity := range entities {
		assignments = append(assignments, entity.(*chunkmodels.AssignmentDataPack).Assignment())
	}
	return assignments
}
