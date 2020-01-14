// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Approvals represents a concurrency-safe memory pool for result approvals.
type Approvals interface {

	// Has checks whether the result approval with the given hash is currently in
	// the memory pool.
	Has(approvalID flow.Identifier) bool

	// Add will add the given result approval to the memory pool; it will error if
	// the result approval is already in the memory pool.
	Add(approval *flow.ResultApproval) error

	// Rem will remove the given result approval from the memory pool; it will
	// will return true if the result approval was known and removed.
	Rem(approvalID flow.Identifier) bool

	// Get will retrieve the given result approval from the memory pool; it will
	// error if the result approval is not in the memory pool.
	Get(approvalID flow.Identifier) (*flow.ResultApproval, error)

	// Size will return the current size of the memory pool.
	Size() uint

	// All will retrieve all result approvals that are currently in the memory pool
	// as a slice.
	All() []*flow.ResultApproval

	// Hash will return a fingerprint has representing the contents of the
	// entire memory pool.
	Hash() flow.Identifier
}
