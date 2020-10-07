package mempool

import "github.com/onflow/flow-go/model/flow"

// IncorporatedResults represents a concurrency safe memory pool for
// incorporated results
type IncorporatedResults interface {
	// Add adds an IncorporatedResult to the mempool
	Add(result *flow.IncorporatedResult) bool

	// All returns all the IncorporatedResults in the mempool
	All() []*flow.IncorporatedResult

	// ByResultID returns all the IncorporatedResults that contain a specific
	// ExecutionResult.
	ByResultID(resultID flow.Identifier) []*flow.IncorporatedResult

	// Rem removes an IncorporatedResult from the mempool
	Rem(incorporatedResultID flow.Identifier) bool

	// Size returns the number of items in the mempool
	Size() uint
}
