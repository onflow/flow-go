package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/storage/badger/operation"
)

// InitPublic initializes a public database by checking and setting the database
// type marker. If an existing, inconsistent type marker is set, this method will
// return an error. Once a database type marker has been set using these methods,
// the type cannot be changed.
func InitPublic(opts badger.Options) (*badger.DB, error) {

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("could not open db: %w", err)
	}
	err = db.Update(operation.InsertPublicDBMarker)
	if err != nil {
		return nil, fmt.Errorf("could not assert db type: %w", err)
	}

	return db, nil
}

// InitSecret initializes a secrets database by checking and setting the database
// type marker. If an existing, inconsistent type marker is set, this method will
// return an error. Once a database type marker has been set using these methods,
// the type cannot be changed.
func InitSecret(opts badger.Options) (*badger.DB, error) {

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("could not open db: %w", err)
	}
	err = db.Update(operation.InsertSecretDBMarker)
	if err != nil {
		return nil, fmt.Errorf("could not assert db type: %w", err)
	}

	return db, nil
}
