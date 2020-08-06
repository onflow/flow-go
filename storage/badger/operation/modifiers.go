package operation

import (
	"errors"
	"time"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/storage"
)

const initialRetryDelay = 10 * time.Millisecond
const maxRetryDelay = 250 * time.Millisecond

func SkipDuplicates(op func(*badger.Txn) error) func(tx *badger.Txn) error {
	return func(tx *badger.Txn) error {
		err := op(tx)
		if errors.Is(err, storage.ErrAlreadyExists) {
			return nil
		}
		return err
	}
}

func RetryOnConflict(action func(func(*badger.Txn) error) error, op func(tx *badger.Txn) error) error {
	delay := initialRetryDelay
	for {
		err := action(op)
		if errors.Is(err, badger.ErrConflict) {
			time.Sleep(delay)
			delay = delay * 2
			if delay > maxRetryDelay {
				delay = maxRetryDelay
			}
			continue
		}
		return err
	}
}
