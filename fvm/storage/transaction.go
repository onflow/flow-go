package storage

import (
	"github.com/onflow/flow-go/fvm/derived"
	"github.com/onflow/flow-go/fvm/state"
)

type Transaction interface {
	state.NestedTransaction
	derived.DerivedTransaction
}

type TransactionComitter interface {
	Transaction

	// Validate returns nil if the transaction does not conflict with
	// previously committed transactions.  It returns an error otherwise.
	Validate() error

	// Commit commits the transaction.  If the transaction conflict with
	// previously committed transactions, an error is returned and the
	// transaction is not committed.
	Commit() error
}

// TODO(patrick): implement proper transaction.
type SerialTransaction struct {
	state.NestedTransaction
	derived.DerivedTransactionCommitter
}
