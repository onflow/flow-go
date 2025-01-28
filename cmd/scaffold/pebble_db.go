package scaffold

import (
	"fmt"
	"io"
	"os"

	"github.com/cockroachdb/pebble"

	pebblestorage "github.com/onflow/flow-go/storage/pebble"
)

func InitPebbleDB(dir string) (*pebble.DB, io.Closer, error) {
	// Pre-create DB path
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create pebble db (path: %s): %w", dir, err)
	}

	db, err := pebblestorage.OpenDefaultPebbleDB(dir)
	if err != nil {
		return nil, nil, err
	}

	return db, db, nil
}
