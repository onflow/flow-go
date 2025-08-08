package database

import (
	"io"

	"github.com/cockroachdb/pebble/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/scaffold"
)

// InitPebbleDB is an alias for scaffold.InitPebbleDB.
func InitPebbleDB(logger zerolog.Logger, dir string) (*pebble.DB, io.Closer, error) {
	return scaffold.InitPebbleDB(logger, dir)
}
