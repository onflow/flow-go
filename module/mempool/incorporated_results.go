package mempool

import "github.com/onflow/flow-go/model/flow"

// IncorporatedResults represents a concurrency safe memory pool for
// incorporated results
type IncorporatedResults interface {
	// Add adds an IncorporatedResult to the mempool
	Add(result *flow.IncorporatedResult) (bool, error)

	// All returns all the IncorporatedResults in the mempool
	All() flow.IncorporatedResultList

	// ByResultID returns all the IncorporatedResults that contain a specific
	// ExecutionResult, indexed by IncorporatedBlockID, along with the
	// ExecutionResult.
	ByResultID(resultID flow.Identifier) (*flow.ExecutionResult, map[flow.Identifier]*flow.IncorporatedResult, bool)

	// Rem removes an IncorporatedResult from the mempool
	Rem(incorporatedResult *flow.IncorporatedResult) bool

	// Size returns the number of items in the mempool
	Size() uint
}
