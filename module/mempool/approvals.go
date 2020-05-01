// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Approvals represents a concurrency-safe memory pool for result approvals.
type Approvals interface {

	// Add will add the given result approval to the memory pool.
	Add(approval *flow.ResultApproval) error

	// Has will check if the given approval is in the memory pool.
	Has(approvalID flow.Identifier) bool

	// Rem will attempt to remove the approval from the memory pool.
	Rem(approvalID flow.Identifier) bool

	// ByResultID will retrieve the approval by receipt ID.
	ByResultID(resultID flow.Identifier) []*flow.ResultApproval

	// Drop will drop the approvals for the given block.
	DropForBlock(blockID flow.Identifier)

	// Size will return the current size of the memory pool.
	Size() uint
}
