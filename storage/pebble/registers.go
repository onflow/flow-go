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
	latestHeight *atomic.Uint64
}

var _ storage.RegisterIndex = (*Registers)(nil)

// NewRegisters takes a populated pebble instance with LatestHeight and FirstHeight set.
// return storage.ErrNotBootstrapped if they those two keys are unavailable as it implies a uninitialized state
// return other error if database is in a corrupted state
func NewRegisters(db *pebble.DB) (*Registers, error) {
	// check height keys and populate cache. These two variables will have been set
	firstHeight, latestHeight, err := ReadHeightsFromBootstrappedDB(db)
	if err != nil {
		// first height is found, but latest height is not found, this means that the DB is in a corrupted state
		return nil, fmt.Errorf("unable to initialize register storage, latest height unavailable in db: %w", err)
	}
	/// All registers between firstHeight and lastHeight have been indexed
	return &Registers{
		db:           db,
		firstHeight:  firstHeight,
		latestHeight: atomic.NewUint64(latestHeight),
	}, nil
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
	iter := s.db.NewIter(&pebble.IterOptions{
		UseL6Filters: true,
	})

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
	err := batch.Set(latestHeightKey, encodedUint64(height), nil)
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

func firstStoredHeight(db *pebble.DB) (uint64, error) {
	return heightLookup(db, firstHeightKey)
}

func latestStoredHeight(db *pebble.DB) (uint64, error) {
	return heightLookup(db, latestHeightKey)
}

func heightLookup(db *pebble.DB, key []byte) (uint64, error) {
	res, closer, err := db.Get(key)
	if err != nil {
		return 0, convertNotFoundError(err)
	}
	defer closer.Close()
	return binary.BigEndian.Uint64(res), nil
}

// convert pebble NotFound error to storage NotFound error
func convertNotFoundError(err error) error {
	if errors.Is(err, pebble.ErrNotFound) {
		return storage.ErrNotFound
	}
	return err
}
