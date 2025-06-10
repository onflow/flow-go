package migration

import (
	"fmt"
	"os"
	"slices"

	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/storage"
)

// ValidationMode defines how thorough the validation should be
type ValidationMode string

const (
	// PartialValidation only checks min/max keys for each prefix
	PartialValidation ValidationMode = "partial"
	// FullValidation checks all keys in the database
	FullValidation ValidationMode = "full"
)

// isDirEmpty checks if a directory exists and is empty.
// Returns true if the directory is empty, false if it contains files,
// and an error if the directory doesn't exist or there's an error reading it.
func isDirEmpty(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

// createDirIfNotExists creates a directory if it doesn't exist.
// Returns an error if the directory already exists and is not empty,
// or if there's an error creating the directory.
func createDirIfNotExists(dir string) error {
	if stat, err := os.Stat(dir); err == nil {
		if !stat.IsDir() {
			return fmt.Errorf("path exists but is not a directory: %s", dir)
		}
		isEmpty, err := isDirEmpty(dir)
		if err != nil {
			return fmt.Errorf("failed to check if directory is empty: %w", err)
		}
		if !isEmpty {
			return fmt.Errorf("directory exists and is not empty: %s", dir)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("error checking directory: %w", err)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return nil
}

// validateBadgerFolderExistPebbleFolderEmpty checks if the Badger directory exists and is non-empty,
// and if the Pebble directory does not exist or is empty.
func validateBadgerFolderExistPebbleFolderEmpty(badgerDir string, pebbleDir string) error {
	// Step 1.1: Ensure Badger directory exists and is non-empty
	isEmpty, err := isDirEmpty(badgerDir)
	if err != nil {
		return fmt.Errorf("badger directory invalid: %w", err)
	}
	if isEmpty {
		return fmt.Errorf("badger directory is empty, %v", badgerDir)
	}

	// Step 1.2: Ensure Pebble directory does not exist or is empty
	if err := createDirIfNotExists(pebbleDir); err != nil {
		return fmt.Errorf("pebble directory validation failed %v: %w", pebbleDir, err)
	}

	return nil
}

func validateMinMaxKeyConsistency(badgerDB *badger.DB, pebbleDB *pebble.DB, prefixBytes int) error {
	keys, err := sampleValidationKeysByPrefix(badgerDB, prefixBytes)
	if err != nil {
		return fmt.Errorf("failed to collect validation keys: %w", err)
	}
	if err := compareValuesBetweenDBs(keys, badgerDB, pebbleDB); err != nil {
		return fmt.Errorf("data mismatch found: %w", err)
	}
	return nil
}

// sampleValidationKeysByPrefix takes a prefix bytes number (1 means 1 byte prefix, 2 means 2 bytes prefix, etc.),
// and returns a list of keys that are the min and max keys for each prefix.
// The output will be used to validate the consistency between Badger and Pebble databases.
// Why? Because we want to validate the consistency between Badger and Pebble databases by selecting
// some keys and compare their values between the two databases.
// An easy way to select keys is to go through each prefix, and find the min and max keys for each prefix using
// the database iterator.
func sampleValidationKeysByPrefix(db *badger.DB, prefixBytes int) ([][]byte, error) {
	// this includes all prefixes that is shorter than or equal to prefixBytes
	// for instance, if prefixBytes is 2, we will include all prefixes that is 1 byte or 2 bytes:
	// [
	//   [0x00], [0x01], [0x02], ..., [0xff], 												// 1 byte prefixes
	//   [0x00, 0x00], [0x00, 0x01], [0x00, 0x02], ..., [0xff, 0xff] 	// 2 byte prefixes
	// ]
	prefixes := GenerateKeysShorterThanPrefix(prefixBytes + 1)
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

// compareValuesBetweenDBs takes a list of keys and compares the values between Badger and Pebble databases,
// it returns error if any of the values are different.
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
			return fmt.Errorf("value mismatch for key %x: badger=%q pebble=%q: %w", key, badgerVal, pebbleVal,
				storage.ErrDataMismatch)
		}
		_ = closer.Close()
	}
	return nil
}

// validateData performs validation based on the configured validation mode
func validateData(badgerDB *badger.DB, pebbleDB *pebble.DB, cfg MigrationConfig) error {
	switch cfg.ValidationMode {
	case PartialValidation:
		return validateMinMaxKeyConsistency(badgerDB, pebbleDB, cfg.ReaderShardPrefixBytes)
	case FullValidation:
		return validateAllKeys(badgerDB, pebbleDB)
	default:
		return fmt.Errorf("unknown validation mode: %s", cfg.ValidationMode)
	}
}

// validateAllKeys performs a full validation by comparing all keys between Badger and Pebble
func validateAllKeys(badgerDB *badger.DB, pebbleDB *pebble.DB) error {
	var allKeys [][]byte
	err := badgerDB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			allKeys = append(allKeys, slices.Clone(it.Item().Key()))
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to collect all keys from Badger: %w", err)
	}

	return compareValuesBetweenDBs(allKeys, badgerDB, pebbleDB)
}
