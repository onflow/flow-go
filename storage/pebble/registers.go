package pebble

import (
	"encoding/binary"
	"fmt"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// Registers library that implements pebble storage for registers
// given a pebble instance with root block and root height populated
type Registers struct {
	db           *pebble.DB
	firstHeight  uint64
	latestHeight uint64
}

// NewRegisters takes a populated pebble instance with LatestHeight and FirstHeight set.
// Will fail if they those two keys are unavailable as it implies a corrupted or uninitialized state
func NewRegisters(db *pebble.DB) (*Registers, error) {
	registers := &Registers{
		db: db,
	}
	// check height keys and populate cache. These two variables will have been set
	firstHeight, err := registers.FirstHeight()
	if err != nil {
		// this means that the DB is either in a corrupted state or has not been initialized with a set of registers
		// and firstHeight
		return nil, fmt.Errorf("unable to initialize register storage, first height unavailable in db: %w", err)
	}
	latestHeight, err := registers.LatestHeight()
	if err != nil {
		// and firstHeight
		return nil, fmt.Errorf("unable to initialize register storage, latest height unavailable in db: %w", err)
	}
	// since we assume the state of the DB has complete registers as of FirstHeight indexed,
	// LatestHeight should also be the same
	registers.firstHeight = firstHeight
	registers.latestHeight = latestHeight
	return registers, nil
}

// Get returns the most recent updated payload for the given RegisterID.
// "most recent" means the updates happens most recent up the given height.
//
// For example, if there are 2 values stored for register A at height 6 and 11, then
// GetPayload(13, A) would return the value at height 11.
//
// If no payload is found, storage.ErrNotFound is returned
func (s *Registers) Get(
	height uint64,
	reg flow.RegisterID,
) ([]byte, error) {
	if height > s.latestHeight || height < s.firstHeight {
		return nil, storage.ErrHeightNotIndexed
	}
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
		return nil, storage.ErrNotFound
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

// Store sets the given entries in a batch.
func (s *Registers) Store(
	height uint64,
	entries flow.RegisterEntries,
) error {
	if height == s.latestHeight {
		// already updated
		return nil
	}

	nextHeight := s.latestHeight + 1
	if height != nextHeight {
		return fmt.Errorf("must store registers with the next height %v, but got %v", nextHeight, height)
	}
	batch := s.db.NewBatch()
	defer batch.Close()

	for _, entry := range entries {
		encoded := newLookupKey(height, entry.Key).Bytes()

		err := batch.Set(encoded, entry.Value, nil)
		if err != nil {
			return fmt.Errorf("failed to set key: %w", err)
		}
	}
	// increment height and commit
	err := batch.Set(LatestHeightKey(), EncodedUint64(height), nil)
	if err != nil {
		return fmt.Errorf("failed to update latest height %d", height)
	}
	err = batch.Commit(pebble.Sync)
	if err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}
	s.latestHeight = height

	return nil
}

// LatestHeight Gets the latest height of complete registers available
func (s *Registers) LatestHeight() (uint64, error) {
	return s.heightLookup(LatestHeightKey())
}

// FirstHeight first indexed height found in the store, typically root block for the spork
func (s *Registers) FirstHeight() (uint64, error) {
	return s.heightLookup(FirstHeightKey())
}

func (s *Registers) heightLookup(key []byte) (uint64, error) {
	res, closer, err := s.db.Get(key)
	if err != nil {
		return 0, err
	}
	defer closer.Close()
	return binary.BigEndian.Uint64(res), nil
}
