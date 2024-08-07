package pebble

import (
	"fmt"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/storage/pebble/operation"
)

// InitPublic initializes a public database by checking and setting the database
// type marker. If an existing, inconsistent type marker is set, this method will
// return an error. Once a database type marker has been set using these methods,
// the type cannot be changed.
func InitPublic(dir string, opts *pebble.Options) (*pebble.DB, error) {

	db, err := pebble.Open(dir, opts)
	if err != nil {
		return nil, fmt.Errorf("could not open db: %w", err)
	}
	err = operation.InsertPublicDBMarker(db)
	if err != nil {
		return nil, fmt.Errorf("could not assert db type: %w", err)
	}

	return db, nil
}

// InitSecret initializes a secrets database by checking and setting the database
// type marker. If an existing, inconsistent type marker is set, this method will
// return an error. Once a database type marker has been set using these methods,
// the type cannot be changed.
func InitSecret(dir string, opts *pebble.Options) (*pebble.DB, error) {

	db, err := pebble.Open(dir, opts)
	if err != nil {
		return nil, fmt.Errorf("could not open db: %w", err)
	}
	err = operation.InsertSecretDBMarker(db)
	if err != nil {
		return nil, fmt.Errorf("could not assert db type: %w", err)
	}

	return db, nil
}
