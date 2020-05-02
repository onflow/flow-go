package operation

import (
	"errors"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/storage"
)

func SkipDuplicates(op func(*badger.Txn) error) func(tx *badger.Txn) error {
	return func(tx *badger.Txn) error {
		err := op(tx)
		if errors.Is(err, storage.ErrAlreadyExists) {
			return nil
		}
		return err
	}
}

func RetryOnConflict(action func() error) error {
	for {
		err := action()
		if errors.Is(err, badger.ErrConflict) {
			continue
		}
		return err
	}
}
