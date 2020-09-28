package operation

import (
	"errors"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
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
