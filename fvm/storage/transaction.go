package storage

import (
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/logical"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/storage/state"
)

type TransactionPreparer interface {
	state.NestedTransactionPreparer
	derived.DerivedTransactionPreparer
}

type Transaction interface {
	TransactionPreparer

	// SnapshotTime returns the transaction's current snapshot time.
	SnapshotTime() logical.Time

	// Finalize convert transaction preparer's intermediate state into
	// committable state.
	Finalize() error

	// Validate returns nil if the transaction does not conflict with
	// previously committed transactions.  It returns an error otherwise.
	Validate() error

	// Commit commits the transaction.  If the transaction conflict with
	// previously committed transactions, an error is returned and the
	// transaction is not committed.
	Commit() (*snapshot.ExecutionSnapshot, error)
}
