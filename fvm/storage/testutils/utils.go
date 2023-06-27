package testutils

import (
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/storage/state"
)

// NewSimpleTransaction returns a transaction which can be used to test
// fvm evaluation.  The returned transaction should not be committed.
func NewSimpleTransaction(
	snapshot snapshot.StorageSnapshot,
) storage.Transaction {
	blockDatabase := storage.NewBlockDatabase(snapshot, 0, nil)

	txn, err := blockDatabase.NewTransaction(0, state.DefaultParameters())
	if err != nil {
		panic(err)
	}

	return txn
}
