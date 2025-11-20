package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
)

// CreateDatastoreManager creates a new datastore manager of the specified type.
// It supports both Badger and Pebble datastores.
func CreateDatastoreManager(
	logger zerolog.Logger,
	executionDataDir string,
) (DatastoreManager, error) {

	// create the datastore directory if it does not exist
	datastoreDir := filepath.Join(executionDataDir, "blobstore")
	err := os.MkdirAll(datastoreDir, 0700)
	if err != nil {
		return nil, err
	}

	// create the appropriate datastore manager based on the DB mode
	var executionDatastoreManager DatastoreManager
	logger.Info().Msgf("Using Pebble datastore for execution data at %s", datastoreDir)
	executionDatastoreManager, err = NewPebbleDatastoreManager(
		logger.With().Str("pebbledb", "endata").Logger(),
		datastoreDir, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create PebbleDatastoreManager for execution data: %w", err)
	}

	return executionDatastoreManager, nil
}
