package database

import (
	"io"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/cmd/scaffold"
)

// InitPebbleDB is an alias for scaffold.InitPebbleDB.
func InitPebbleDB(dir string) (*pebble.DB, io.Closer, error) {
	return scaffold.InitPebbleDB(dir)
}
