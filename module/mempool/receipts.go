// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Receipts represents a concurrency-safe memory pool for execution receipts.
type Receipts interface {

	// Has checks whether the execution receipt with the given hash is currently in
	// the memory pool.
	Has(receiptID flow.Identifier) bool

	// Add will add the given execution receipt to the memory pool; it will error if
	// the execution receipt is already in the memory pool.
	Add(receipt *flow.ExecutionReceipt) error

	// Rem will remove the given execution receipt from the memory pool; it will
	// will return true if the execution receipt was known and removed.
	Rem(receiptID flow.Identifier) bool

	// ByID will retrieve the given execution receipt from the memory pool; it will
	// error if the execution receipt is not in the memory pool.
	ByID(receiptID flow.Identifier) (*flow.ExecutionReceipt, error)

	// ByResultID will retrieve a receipt by its result ID.
	ByResultID(resultID flow.Identifier) (*flow.ExecutionReceipt, error)

	// Size will return the current size of the memory pool.
	Size() uint

	// All will retrieve all execution receipts that are currently in the memory pool
	// as a slice.
	All() []*flow.ExecutionReceipt

	// Hash will return a fingerprint has representing the contents of the
	// entire memory pool.
	Hash() flow.Identifier
}
