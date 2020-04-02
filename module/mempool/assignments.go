// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED
package mempool

import (
	"github.com/dapperlabs/flow-go/model/chunkassignment"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Assignments represents a concurrency-safe memory pool for chunk assignments
type Assignments interface {

	// Has checks whether the Assignment with the given hash is currently in
	// the memory pool.
	Has(assignmentID flow.Identifier) bool

	// ByID retrieves the chunk assignment from mempool based on provided ID
	ByID(assignmentID flow.Identifier) (*chunkassignment.Assignment, error)

	// Add will add the given Assignment to the memory pool; it will error if
	// the Assignment is already in the memory pool.
	Add(assignmentFingerprint flow.Identifier, assignment *chunkassignment.Assignment) error

	// Rem will remove the given Assignment from the memory pool; it will
	// return true if the Assignment was known and removed.
	Rem(assignmentID flow.Identifier) bool

	// Size will return the current size of the memory pool.
	Size() uint

	// All will retrieve all Assignments that are currently in the memory pool
	// as a slice.
	All() []*chunkassignment.Assignment
}
