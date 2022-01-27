package mempool

import (
	"github.com/onflow/flow-go/model/flow"
)

// DeduplicationCache is a memory pool that tracks duplicate entities through their identifier.
type DeduplicationCache interface {
	// Add puts the identifier in the cache if it does not exist and returns true, otherwise, returns false.
	Add(flow.Identifier) bool
}
