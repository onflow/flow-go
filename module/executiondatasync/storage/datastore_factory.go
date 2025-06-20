package storage

import (
	"fmt"
	"os"
	"path/filepath"

	badgerds "github.com/ipfs/go-ds-badger2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

// CreateDatastoreManager creates a new datastore manager of the specified type.
// It supports both Badger and Pebble datastores.
func CreateDatastoreManager(
	logger zerolog.Logger,
	executionDataDir string,
	executionDataDBModeStr string,
) (DatastoreManager, error) {

	// create the datastore directory if it does not exist
	datastoreDir := filepath.Join(executionDataDir, "blobstore")
	err := os.MkdirAll(datastoreDir, 0700)
	if err != nil {
		return nil, err
	}

	// parse the execution data DB mode
	// TODO(leo): automatically parse the DB mode from the data in the database folder
	executionDataDBMode, err := execution_data.ParseExecutionDataDBMode(executionDataDBModeStr)
	if err != nil {
		return nil, fmt.Errorf("could not parse execution data DB mode: %w", err)
	}

	// create the appropriate datastore manager based on the DB mode
	var executionDatastoreManager DatastoreManager
	if executionDataDBMode == execution_data.ExecutionDataDBModePebble {
		executionDatastoreManager, err = NewPebbleDatastoreManager(
			logger.With().Str("pebbledb", "endata").Logger(),
			datastoreDir, nil)
		if err != nil {
			return nil, fmt.Errorf("could not create PebbleDatastoreManager for execution data: %w", err)
		}
	} else {
		executionDatastoreManager, err = NewBadgerDatastoreManager(datastoreDir, &badgerds.DefaultOptions)
		if err != nil {
			return nil, fmt.Errorf("could not create BadgerDatastoreManager for execution data: %w", err)
		}
	}

	return executionDatastoreManager, nil
}
