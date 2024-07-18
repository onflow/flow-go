package pebble

import (
	"errors"
	"fmt"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/storage"
)

func handleError(err error, t interface{}) error {
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return storage.ErrNotFound
		}

		return fmt.Errorf("could not retrieve %T: %w", t, err)
	}
	return nil
}
