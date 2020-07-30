// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// TransactionTimings represents a concurrency-safe memory pool for transaction timings.
type TransactionTimings interface {

	// Add adds a transaction timing to the mempool.
	Add(tx *flow.TransactionTiming) bool

	// ByID returns the transaction timing with the given ID from the mempool.
	ByID(txID flow.Identifier) (*flow.TransactionTiming, bool)
	// Adjust will adjust the transaction timing using the given function if the given key can be found.
	// Returns a bool which indicates whether the value was updated as well as the updated value.
	Adjust(txID flow.Identifier, f func(*flow.TransactionTiming) *flow.TransactionTiming) (*flow.TransactionTiming,
		bool)

	// All returns all transaction timings from the mempool.
	All() []*flow.TransactionTiming

	// Rem removes the transaction timing with the given ID.
	Rem(txID flow.Identifier) bool
}
