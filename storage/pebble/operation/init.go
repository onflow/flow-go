package operation

import (
	"errors"
	"fmt"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/storage"
)

// marker used to denote a database type
type dbTypeMarker int

func (marker dbTypeMarker) String() string {
	return [...]string{
		"dbMarkerPublic",
		"dbMarkerSecret",
	}[marker]
}

const (
	// dbMarkerPublic denotes the public database
	dbMarkerPublic dbTypeMarker = iota
	// dbMarkerSecret denotes the secrets database
	dbMarkerSecret
)

func InsertPublicDBMarker(db *pebble.DB) error {
	return WithReaderBatchWriter(db, insertDBTypeMarker(dbMarkerPublic))
}

func InsertSecretDBMarker(db *pebble.DB) error {
	return WithReaderBatchWriter(db, insertDBTypeMarker(dbMarkerSecret))
}

func EnsurePublicDB(db *pebble.DB) error {
	return ensureDBWithType(db, dbMarkerPublic)
}

func EnsureSecretDB(db *pebble.DB) error {
	return ensureDBWithType(db, dbMarkerSecret)
}

// insertDBTypeMarker inserts a database type marker if none exists. If a marker
// already exists in the database, this function will return an error if the
// marker does not match the argument, or return nil if it matches.
func insertDBTypeMarker(marker dbTypeMarker) func(storage.PebbleReaderBatchWriter) error {
	return func(rw storage.PebbleReaderBatchWriter) error {
		r, txn := rw.ReaderWriter()
		var storedMarker dbTypeMarker
		err := retrieveDBType(&storedMarker)(r)
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

// ensureDBWithType ensures the given database has been initialized with the
// given database type marker. If the given database has not been initialized
// with any marker, or with a different marker than expected, returns an error.
func ensureDBWithType(db *pebble.DB, expectedMarker dbTypeMarker) error {
	var actualMarker dbTypeMarker
	err := retrieveDBType(&actualMarker)(db)
	if err != nil {
		return fmt.Errorf("could not get db type: %w", err)
	}
	if actualMarker != expectedMarker {
		return fmt.Errorf("wrong db type (expected: %s, actual: %s)", expectedMarker, actualMarker)
	}
	return nil
}

func retrieveDBType(marker *dbTypeMarker) func(pebble.Reader) error {
	return retrieve(makePrefix(codeDBType), marker)
}
