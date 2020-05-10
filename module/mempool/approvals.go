// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Approvals represents a concurrency-safe memory pool for result approvals.
type Approvals interface {

	// Has will check if the given approval is in the memory pool.
	Has(approvalID flow.Identifier) bool

	// Add will add the given result approval to the memory pool. It will return
	// false if it was already in the mempool.
	Add(approval *flow.ResultApproval) bool

	// Rem will attempt to remove the approval from the memory pool.
	Rem(approvalID flow.Identifier) bool

	// ByID retrieve the result approval with the given ID from the memory pool.
	// It will return false if it was not found in the mempool.
	ByID(approvalID flow.Identifier) (*flow.ResultApproval, bool)

	// Size will return the current size of the memory pool.
	Size() uint

	// All will return a list of all approvals in the memory pool.
	All() []*flow.ResultApproval

	// DropForResult will drop the approvals for the given block.
	DropForResult(resultID flow.Identifier) []flow.Identifier
}
