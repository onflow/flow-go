package badger

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/storage"
)

func handleError(err error, t interface{}) error {
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return storage.ErrNotFound
		}

		return fmt.Errorf("could not retrieve %T: %w", t, err)
	}
	return nil
}
