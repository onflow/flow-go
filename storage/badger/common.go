package badger

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"
)

func handleError(err error, t interface{}) error {
	if err != nil {
		if errors.Is(err, badger.ErrKeyNotFound) {
			return err
		}

		return fmt.Errorf("could not retrieve %T: %w", t, err)
	}
	return nil
}
