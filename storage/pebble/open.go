package pebble

import (
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/registers"
	"github.com/pkg/errors"
)

// NewBootstrappedRegistersWithPath initializes a new Registers instance with a pebble db
// if the database is not initialized, it close the database and return pebble.ErrNotBootstrapped
func NewBootstrappedRegistersWithPath(dir string) (*Registers, *pebble.DB, error) {
	db, err := OpenRegisterPebbleDB(dir)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize pebble db: %w", err)
	}
	registers, err := NewRegisters(db)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize registers: %w", err)
	}
	return registers, db, nil
}

// openRegisterPebbleDB opens the database
func OpenRegisterPebbleDB(dir string) (*pebble.DB, error) {
	cache := pebble.NewCache(1 << 20)
	defer cache.Unref()
	// currently pebble is only used for registers
	opts := DefaultPebbleOptions(cache, registers.NewMVCCComparer())
	db, err := pebble.Open(dir, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	return db, nil
}

// ReadHeightsFromBootstrappedDB reads the first and latest height from a bootstrapped register db
// If the register db is not bootstrapped, it returns ErrNotBootstrapped
// If the register db is corrupted, it returns an error
func ReadHeightsFromBootstrappedDB(db *pebble.DB) (firstHeight uint64, latestHeight uint64, err error) {
	// check height keys and populate cache. These two variables will have been set
	firstHeight, err = firstStoredHeight(db)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return 0, 0, fmt.Errorf("unable to initialize register storage, first height not found in db: %w", ErrNotBootstrapped)
		}
		// this means that the DB is either in a corrupted state or has not been initialized
		return 0, 0, fmt.Errorf("unable to initialize register storage, first height unavailable in db: %w", err)
	}
	latestHeight, err = latestStoredHeight(db)
	if err != nil {
		// first height is found, but latest height is not found, this means that the DB is in a corrupted state
		return 0, 0, fmt.Errorf("unable to initialize register storage, latest height unavailable in db: %w", err)
	}
	return firstHeight, latestHeight, nil
}

// IsDBNotBootstrapped returns true if the db is not bootstrapped
// otherwise return false
func IsDBNotBootstrapped(db *pebble.DB) (bool, error) {
	//
	_, err := firstStoredHeight(db)
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return false, fmt.Errorf("unable to check first stored height: %w", err)
	}

	_, err = latestStoredHeight(db)
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, storage.ErrNotFound) {
		return false, fmt.Errorf("unable to check latest stored height: %w", err)
	}
	return true, nil
}

func initHeights(db *pebble.DB, firstHeight uint64) error {
	batch := db.NewBatch()
	defer batch.Close()
	// update heights atomically to prevent one getting populated without the other
	err := batch.Set(firstHeightKey, encodedUint64(firstHeight), nil)
	if err != nil {
		return fmt.Errorf("unable to add first height to batch: %w", err)
	}
	err = batch.Set(latestHeightKey, encodedUint64(firstHeight), nil)
	if err != nil {
		return fmt.Errorf("unable to add latest height to batch: %w", err)
	}
	err = batch.Commit(pebble.Sync)
	if err != nil {
		return fmt.Errorf("unable to index first and latest heights: %w", err)
	}
	return nil
}
