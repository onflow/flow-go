package operation

import (
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/storage"
)

type dbTypeMarker int

func (marker dbTypeMarker) String() string {
	return [...]string{
		"dbMarkerPublic",
		"dbMarkerSnowflake",
		"dbMarkerSecret",
	}[marker]
}

const (
	dbMarkerPublic dbTypeMarker = iota
	dbMarkerSnowflake
	dbMarkerSecret
)

func InsertPublicDBMarker(txn *badger.Txn) error {
	return insertDBTypeMarker(dbMarkerPublic)(txn)
}

func InsertSnowflakeDBMarker(txn *badger.Txn) error {
	return insertDBTypeMarker(dbMarkerSnowflake)(txn)
}

func InsertSecretDBMarker(txn *badger.Txn) error {
	return insertDBTypeMarker(dbMarkerSecret)(txn)
}

func EnsurePublicDB(db *badger.DB) error {
	return ensureDBWithType(db, dbMarkerPublic)
}

func EnsureSnowflakeDB(db *badger.DB) error {
	return ensureDBWithType(db, dbMarkerSnowflake)
}

func EnsureSecretDB(db *badger.DB) error {
	return ensureDBWithType(db, dbMarkerSecret)
}

func insertDBTypeMarker(marker dbTypeMarker) func(*badger.Txn) error {
	return func(txn *badger.Txn) error {
		var storedMarker dbTypeMarker
		err := retrieveDBType(&storedMarker)(txn)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("could not check db type marker: %w", err)
		}

		// we retrieved a marker from storage
		if err == nil {
			// the marker in storage does not match - error
			if storedMarker != marker {
				return fmt.Errorf("could not store db type marker - inconsistent marker already stored (expected: %s, actual: %s)", marker, storedMarker)
			}
			// the marker is already in storage - we're done
			return nil
		}

		// no marker in storage, insert it
		return insert(makePrefix(codeDBType), marker)(txn)
	}
}

func ensureDBWithType(db *badger.DB, expectedMarker dbTypeMarker) error {
	var actualMarker dbTypeMarker
	err := db.View(retrieveDBType(&actualMarker))
	if err != nil {
		return fmt.Errorf("could not get db type: %w", err)
	}
	if actualMarker != expectedMarker {
		return fmt.Errorf("wrong db type (expected: %s, actual: %s)", expectedMarker, actualMarker)
	}
	return nil
}

func retrieveDBType(marker *dbTypeMarker) func(*badger.Txn) error {
	return retrieve(makePrefix(codeDBType), marker)
}
