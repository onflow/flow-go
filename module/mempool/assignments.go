// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED
package mempool

import (
	chunkmodels "github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Assignments represents a concurrency-safe memory pool for chunk assignments
type Assignments interface {

	// Has checks whether the Assignment with the given hash is currently in
	// the memory pool.
	Has(assignmentID flow.Identifier) bool

	// Add will add the given assignment to the memory pool. It will return
	// false if it was already in the mempool.
	Add(assignmentFingerprint flow.Identifier, assignment *chunkmodels.Assignment) bool

	// Rem will remove the given Assignment from the memory pool; it will
	// return true if the Assignment was known and removed.
	Rem(assignmentID flow.Identifier) bool

	// ByID retrieve the chunk assigment with the given ID from the memory pool.
	// It will return false if it was not found in the mempool.
	ByID(assignmentID flow.Identifier) (*chunkmodels.Assignment, bool)

	// Size will return the current size of the memory pool.
	Size() uint

	// All will retrieve all Assignments that are currently in the memory pool
	// as a slice.
	All() []*chunkmodels.Assignment
}
