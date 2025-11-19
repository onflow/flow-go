package transactions

import (
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

// TxResultCache is a cache for transaction results used by the API to avoid unnecessary lookups on
// historical access nodes.
type TxResultCache interface {
	// Get retrieves a cached transaction result by transaction ID,	returns true if it exists in the
	// cache, otherwise false.
	Get(flow.Identifier) (*accessmodel.TransactionResult, bool)

	// Add adds a transaction result to the cache keyed by its transaction ID, and returns true if
	// an entry was evicted in the process.
	Add(flow.Identifier, *accessmodel.TransactionResult) bool
}

// NoopTxResultCache is a no-op implementation of the TxResultCache interface.
type NoopTxResultCache struct{}

var _ TxResultCache = (*NoopTxResultCache)(nil)

// NewNoopTxResultCache creates a new no-op implementation of the TxResultCache interface.
func NewNoopTxResultCache() *NoopTxResultCache {
	return &NoopTxResultCache{}
}

// Get retrieves a cached transaction result by transaction ID,	returns true if it exists in the
// cache, otherwise false.
//
// This is a no-op implementation and always returns nil and false
func (n *NoopTxResultCache) Get(flow.Identifier) (*accessmodel.TransactionResult, bool) {
	return nil, false
}

// Add adds a transaction result to the cache keyed by its transaction ID, and returns true if
// an entry was evicted in the process.
//
// This is a no-op implementation which simply ignores the inputs and returns false.
func (n *NoopTxResultCache) Add(flow.Identifier, *accessmodel.TransactionResult) bool {
	return false
}
