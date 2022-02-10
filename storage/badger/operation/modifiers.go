package operation

import (
	"errors"
	"syscall"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/transaction"
)

func SkipDuplicates(op func(*badger.Txn) error) func(tx *badger.Txn) error {
	return func(tx *badger.Txn) error {
		err := op(tx)
		if errors.Is(err, storage.ErrAlreadyExists) {
			metrics.GetStorageCollector().SkipDuplicate()
			return nil
		}
		return err
	}
}

func RetryOnConflict(action func(func(*badger.Txn) error) error, op func(tx *badger.Txn) error) error {
	for {
		err := action(op)
		if errors.Is(err, badger.ErrConflict) {
			metrics.GetStorageCollector().RetryOnConflict()
			continue
		}
		return err
	}
}

func RetryOnConflictTx(db *badger.DB, action func(*badger.DB, func(*transaction.Tx) error) error, op func(*transaction.Tx) error) error {
	for {
		err := action(db, op)
		if errors.Is(err, badger.ErrConflict) {
			metrics.GetStorageCollector().RetryOnConflict()
			continue
		}
		return err
	}
}

// TerminateOnFullDisk helper function to crash node if write failed because disk is full
func TerminateOnFullDisk(err error) error {
	// using panic so any deferred functions can still execute
	// relevant badgerDB code: https://github.com/dgraph-io/badger/blob/156819ccb106bbeb207e985f561780e2929344bc/value.go#L1454-L1463
	if err != nil && errors.Is(err, syscall.ENOSPC) {
		panic("disk full, terminating node...")
	}
	return err
}
