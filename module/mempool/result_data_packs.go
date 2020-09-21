package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification"
)

// ResultDataPacks represents a concurrency-safe memory pool for ResultDataPack data structure.
type ResultDataPacks interface {
	// Add will add the given ResultDataPack to the mempool. It will return
	// false if it was already in the mempool.
	Add(result *verification.ResultDataPack) bool

	// Has returns true if a ResultDataPack with the specified identifier exists.
	Has(resultID flow.Identifier) bool

	// Rem will remove the pending result from the memory pool
	Rem(resultID flow.Identifier) bool

	// Get returns the ResultDataPack and true, if the ResultDataPack is in the
	// mempool. Otherwise, it returns nil and false.
	Get(resultID flow.Identifier) (*verification.ResultDataPack, bool)

	// Size returns total number ResultDataPacks in mempool
	Size() uint
}
