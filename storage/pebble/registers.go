package pebble

import (
	"encoding/binary"
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/pkg/errors"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// Registers library that implements pebble storage for registers
// given a pebble instance with root block and root height populated
type Registers struct {
	db           *pebble.DB
	firstHeight  uint64
	latestHeight atomic.Uint64
}

var _ storage.RegisterIndex = (*Registers)(nil)

// NewRegisters takes a populated pebble instance with LatestHeight and FirstHeight set.
// Will fail if they those two keys are unavailable as it implies a corrupted or uninitialized state
// expected errors:
// - storage.ErrNotBootstrapped error is returned if the database wasn't bootstrapped which involves populating
// the database with first and last height as well as registers
func NewRegisters(db *pebble.DB) (*Registers, error) {
	registers := &Registers{
		db: db,
	}

	// load first and last heights and cache the values, if the first or last height is not successfully loaded
	// we return bootstrapped error indicating the bootstrap did not run successfully
	firstHeight, err := registers.firstStoredHeight()
	if err != nil {
		return nil, errors.Wrap(storage.ErrNotBootstrapped, "first height not loaded")
	}
	latestHeight, err := registers.latestStoredHeight()
	if err != nil {
		return nil, errors.Wrap(storage.ErrNotBootstrapped, "latest height not loaded")
	}

	// all registers between firstHeight and lastHeight have been indexed
	registers.firstHeight = firstHeight
	registers.latestHeight.Store(latestHeight)

	return registers, nil
}

// Get returns the most recent updated payload for the given RegisterID.
// "most recent" means the updates happens most recent up the given height.
//
// For example, if there are 2 values stored for register A at height 6 and 11, then
// GetPayload(13, A) would return the value at height 11.
//
// - storage.ErrNotFound if no register values are found
// - storage.ErrHeightNotIndexed if the requested height is out of the range of stored heights
func (s *Registers) Get(
	reg flow.RegisterID,
	height uint64,
) (flow.RegisterValue, error) {
	latestHeight := s.latestHeight.Load()
	if height > latestHeight || height < s.firstHeight {
		return nil, errors.Wrap(
			storage.ErrHeightNotIndexed,
			fmt.Sprintf("height %d not indexed, indexed range is [%d-%d]", height, s.firstHeight, latestHeight),
		)
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
		// no such register found
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
// This function is expected to be called at one batch per height, sequentially. Under normal conditions,
// it should be called wth the value of height set to LatestHeight + 1
// CAUTION: This function is not safe for concurrent use.
func (s *Registers) Store(
	entries flow.RegisterEntries,
	height uint64,
) error {
	latestHeight := s.latestHeight.Load()
	// This check is for a special case for the execution node.
	// Upon restart, it may be in a state where registers are indexed in pebble for the latest height
	// but the remaining execution data in badger is not, so we skip the indexing step without throwing an error
	if height == latestHeight {
		// already updated
		return nil
	}

	nextHeight := latestHeight + 1
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
	err := batch.Set(latestHeightKey(), encodedUint64(height), nil)
	if err != nil {
		return fmt.Errorf("failed to update latest height %d", height)
	}
	err = batch.Commit(pebble.Sync)
	if err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}
	s.latestHeight.Store(height)

	return nil
}

// LatestHeight Gets the latest height of complete registers available
func (s *Registers) LatestHeight() uint64 {
	return s.latestHeight.Load()
}

// FirstHeight first indexed height found in the store, typically root block for the spork
func (s *Registers) FirstHeight() uint64 {
	return s.firstHeight
}

func (s *Registers) firstStoredHeight() (uint64, error) {
	return s.heightLookup(firstHeightKey())
}

func (s *Registers) latestStoredHeight() (uint64, error) {
	return s.heightLookup(latestHeightKey())
}

func (s *Registers) heightLookup(key []byte) (uint64, error) {
	res, closer, err := s.db.Get(key)
	if err != nil {
		return 0, err
	}
	defer closer.Close()
	return binary.BigEndian.Uint64(res), nil
}
