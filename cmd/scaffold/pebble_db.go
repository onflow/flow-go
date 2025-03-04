package scaffold

import (
	"fmt"
	"io"
	"os"

	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog"

	pebblestorage "github.com/onflow/flow-go/storage/pebble"
)

func InitPebbleDB(logger zerolog.Logger, dir string) (*pebble.DB, io.Closer, error) {
	// if the pebble DB is not set, we skip initialization
	// the pebble DB must be provided to initialize
	// since we've set an default directory for the pebble DB, this check
	// is not necessary, but rather a sanity check
	if dir == "not set" {
		return nil, nil, fmt.Errorf("missing required flag '--pebble-dir'")
	}

	// Pre-create DB path
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create pebble db (path: %s): %w", dir, err)
	}

	db, err := pebblestorage.OpenDefaultPebbleDB(logger, dir)
	if err != nil {
		return nil, nil, fmt.Errorf("could not open newly created pebble db (path: %s): %w", dir, err)
	}

	return db, db, nil
}
