package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/storage/badger/operation"
)

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

func InitSnowflake(opts badger.Options) (*badger.DB, error) {

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("could not open db: %w", err)
	}
	err = db.Update(operation.InsertSnowflakeDBMarker)
	if err != nil {
		return nil, fmt.Errorf("could not assert db type: %w", err)
	}

	return db, nil
}

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
