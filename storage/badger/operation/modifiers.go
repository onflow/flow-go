package operation

import (
	"errors"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/storage"
)

func SkipDuplicates(op func(*badger.Txn) error) func(tx *badger.Txn) error {
	return func(tx *badger.Txn) error {
		if err := op(tx); err != nil && !errors.Is(err, storage.ErrAlreadyExists) {
			return err
		}
		return nil
	}
}

func RetryOnConflict(op func(*badger.Txn) error) func(tx *badger.Txn) error {
	return func(tx *badger.Txn) error {
		for {
			err := op(tx)
			if !errors.Is(err, badger.ErrConflict) {
				return err
			}
		}
	}
}
