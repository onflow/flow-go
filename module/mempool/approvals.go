// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"github.com/onflow/flow-go/model/flow"
)

// Approvals represents a concurrency-safe memory pool for result approvals.
type Approvals interface {

	// Add will add the given result approval to the memory pool. It will return
	// false if it was already in the mempool.
	Add(approval *flow.ResultApproval) (bool, error)

	// RemApproval removes a specific approval.
	RemApproval(approval *flow.ResultApproval) (bool, error)

	// Rem will attempt to remove all the approvals associated with a chunk.
	RemChunk(resultID flow.Identifier, chunkIndex uint64) bool

	// ByChunk returns all the approvals associated with a chunk. It returns an
	// empty map if there is nothing.
	ByChunk(resultID flow.Identifier, chunkIndex uint64) map[flow.Identifier]*flow.ResultApproval

	// All will return a list of all approvals in the memory pool.
	All() []*flow.ResultApproval

	// Size will return the current size of the memory pool.
	Size() uint
}
