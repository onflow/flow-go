package mempool

import "github.com/dapperlabs/flow-go/model/flow"

type PendingResults interface {
	// Add adds the given pending result into the mempool
	Add(result *flow.PendingResult) bool

	// Has will check if the given pending result exists in the memory pool.
	Has(resultID flow.Identifier) bool

	// Rem will remove the pending result from the memory pool
	Rem(resultID flow.Identifier) bool

	// ByID finds the pending result from the memory pool
	ByID(resultID flow.Identifier) (*flow.PendingResult, bool)

	// Size return the number of pending results sotred in the memory pool
	Size() uint
}
