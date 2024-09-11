package common

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage"
	storagebadger "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
)

func InitStorage(datadir string) *badger.DB {
	return InitStorageWithTruncate(datadir, false)
}

func InitStorageWithTruncate(datadir string, truncate bool) *badger.DB {
	if err := ensureFolderExistAndNotEmpty(datadir); err != nil {
		log.Fatal().Err(err).Msgf("could not open badger database store at %v", datadir)
	}

	opts := badger.
		DefaultOptions(datadir).
		WithKeepL0InMemory(true).
		WithLogger(nil).
		WithTruncate(truncate)

	db, err := badger.Open(opts)
	if err != nil {
		log.Fatal().Err(err).Msg("could not open key-value store")
	}

	// in order to void long iterations with big keys when initializing with an
	// already populated database, we bootstrap the initial maximum key size
	// upon starting
	err = operation.RetryOnConflict(db.Update, func(tx *badger.Txn) error {
		return operation.InitMax(tx)
	})
	if err != nil {
		log.Fatal().Err(err).Msg("could not initialize max tracker")
	}

	return db
}

func InitStorages(db *badger.DB) *storage.All {
	metrics := &metrics.NoopCollector{}

	return storagebadger.InitAll(metrics, db)
}

// ensureFolderExistAndNotEmpty checks if the folder exists, is a directory, and is not empty
func ensureFolderExistAndNotEmpty(dir string) error {
	// Get the absolute path
	absPath, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("error getting absolute path: %w", err)
	}

	// Check if the folder exists
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("folder does not exist: %s", absPath)
	} else if err != nil {
		return fmt.Errorf("error checking folder: %w", err)
	}

	// Check if it's a directory
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", absPath)
	}

	// Check if the directory is empty
	f, err := os.Open(absPath)
	if err != nil {
		return fmt.Errorf("error opening folder: %w", err)
	}
	defer f.Close()

	// Read the directory's contents
	_, err = f.Readdirnames(1) // Attempt to read at least one file
	if err == io.EOF {
		return fmt.Errorf("folder is empty: %s", absPath)
	} else if err != nil {
		return fmt.Errorf("error reading folder contents: %w", err)
	}

	return nil
}
