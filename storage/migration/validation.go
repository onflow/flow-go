package migration

import (
	"errors"
	"fmt"
	"os"
	"slices"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/storage"
)

// validateBadgerFolderExistPebbleFolderEmpty checks if the Badger directory exists and is non-empty,
// and if the Pebble directory does not exist or is empty.
func validateBadgerFolderExistPebbleFolderEmpty(badgerDir string, pebbleDir string) error {
	// Step 1.1: Ensure Badger directory exists and is non-empty
	badgerEntries, err := os.ReadDir(badgerDir)
	if err != nil || len(badgerEntries) == 0 {
		return fmt.Errorf("badger directory invalid or empty: %w", err)
	}

	// Step 1.2: Ensure Pebble directory does not exist or is empty
	if stat, err := os.Stat(pebbleDir); err == nil && stat.IsDir() {
		pebbleEntries, err := os.ReadDir(pebbleDir)
		if err != nil {
			return fmt.Errorf("failed to read pebble directory: %w", err)
		}
		if len(pebbleEntries) > 0 {
			return errors.New("pebble directory is not empty")
		}
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error checking pebble directory: %w", err)
	} else {
		// Create pebbleDir if it doesn't exist
		if err := os.MkdirAll(pebbleDir, 0755); err != nil {
			return fmt.Errorf("failed to create pebble directory: %w", err)
		}
	}

	return nil
}

func validateMinMaxKeyConsistency(badgerDB *badger.DB, pebbleDB *pebble.DB, prefixBytes int) error {
	keys, err := collectValidationKeysByPrefix(badgerDB, prefixBytes)
	if err != nil {
		return fmt.Errorf("failed to collect validation keys: %w", err)
	}
	if err := compareValuesBetweenDBs(keys, badgerDB, pebbleDB); err != nil {
		return fmt.Errorf("data mismatch found: %w", err)
	}
	return nil
}

func collectValidationKeysByPrefix(db *badger.DB, prefixBytes int) ([][]byte, error) {
	prefixes := GeneratePrefixes(prefixBytes)
	var allKeys [][]byte

	err := db.View(func(txn *badger.Txn) error {
		for _, prefix := range prefixes {
			// Find min key
			opts := badger.DefaultIteratorOptions
			it := txn.NewIterator(opts)
			it.Seek(prefix)
			if it.ValidForPrefix(prefix) {
				allKeys = append(allKeys, slices.Clone(it.Item().Key()))
			}
			it.Close()

			// Find max key with reverse iterator
			opts.Reverse = true
			it = txn.NewIterator(opts)

			// the upper bound is exclusive, so we need to seek to the upper bound
			// when the prefix is [0xff,0xff], the end is nil, and we will iterate
			// from the last key
			end := storage.PrefixUpperBound(prefix)
			it.Seek(end)
			if it.ValidForPrefix(prefix) {
				allKeys = append(allKeys, slices.Clone(it.Item().Key()))
			}
			it.Close()
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Deduplicate keys
	keyMap := make(map[string][]byte, len(allKeys))
	for _, k := range allKeys {
		keyMap[string(k)] = k
	}
	uniqueKeys := make([][]byte, 0, len(keyMap))
	for _, k := range keyMap {
		uniqueKeys = append(uniqueKeys, k)
	}

	return uniqueKeys, nil
}

func compareValuesBetweenDBs(keys [][]byte, badgerDB *badger.DB, pebbleDB *pebble.DB) error {
	for _, key := range keys {
		var badgerVal []byte
		err := badgerDB.View(func(txn *badger.Txn) error {
			item, err := txn.Get(key)
			if err != nil {
				return err
			}
			badgerVal, err = item.ValueCopy(nil)
			return err
		})
		if err != nil {
			return fmt.Errorf("badger get error for key %x: %w", key, err)
		}

		pebbleVal, closer, err := pebbleDB.Get(key)
		if err != nil {
			return fmt.Errorf("pebble get error for key %x: %w", key, err)
		}
		if string(pebbleVal) != string(badgerVal) {
			return fmt.Errorf("value mismatch for key %x: badger=%q pebble=%q", key, badgerVal, pebbleVal)
		}
		_ = closer.Close()
	}
	return nil
}
