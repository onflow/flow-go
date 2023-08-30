package payload

import (
	"fmt"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/pebble/registers/config"
)

type Storage struct {
	db *pebble.DB
}

// NewStorage creates a pebble-backed payload storage.
// The reason we use a separate storage for payloads we need a Comparer with a custom Split function.
//
// It needs to access the last available payload with height less or equal to the requested height.
// This means all point-lookups are range scans.
func NewStorage(dbPath string, cache *pebble.Cache) (*Storage, error) {
	opts := config.DefaultPebbleOptions(cache, config.NewMVCCComparer())
	db, err := pebble.Open(dbPath, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	return &Storage{
		db: db,
	}, nil
}

// GetPayload returns the most recent updated payload for the given RegisterID.
// "most recent" means the updates happens most recent up the given height.
//
// For example, if there are 2 values stored for register A at height 6 and 11, then
// GetPayload(13, A) would return the value at height 11.
//
// If no payload is found, an empty byte slice is returned.
func (s *Storage) GetPayload(
	height uint64,
	reg flow.RegisterID,
) ([]byte, error) {
	iter, err := s.db.NewIter(&pebble.IterOptions{
		UseL6Filters: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create pebble iterator: %w", err)
	}
	defer iter.Close()

	encoded := newLookupKey(height, reg).Bytes()
	ok := iter.SeekPrefixGE(encoded)
	if !ok {
		return []byte{}, nil
	}

	binaryValue, err := iter.ValueAndErr()
	if err != nil {
		return nil, fmt.Errorf("failed to get value: %w", err)
	}
	// preventing caller from modifying the iterator's value slices
	valueCopy := make([]byte, len(binaryValue))
	copy(valueCopy, binaryValue)

	return valueCopy, nil
}

// BatchSetPayload sets the given entries in a batch.
func (s *Storage) BatchSetPayload(
	height uint64,
	entries flow.RegisterEntries,
) error {
	batch := s.db.NewBatch()
	defer batch.Close()

	for _, entry := range entries {
		encoded := newLookupKey(height, entry.Key).Bytes()

		err := batch.Set(encoded, entry.Value, nil)
		if err != nil {
			return fmt.Errorf("failed to set key: %w", err)
		}
	}

	err := batch.Commit(pebble.Sync)
	if err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}

	return nil
}

func (s *Storage) Checkpoint(dir string) error {
	return s.db.Checkpoint(dir)
}

// Close closes the storage.
func (s *Storage) Close() error {
	return s.db.Close()
}
