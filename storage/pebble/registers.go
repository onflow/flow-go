package pebble

import (
	"encoding/binary"
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/onflow/flow-go/storage"

	"github.com/onflow/flow-go/model/flow"
)

// library that implements pebble storage for registers
type Registers struct {
	db           *pebble.DB
	firstHeight  uint64
	latestHeight uint64
}

func NewRegisters(db *pebble.DB) (*Registers, error) {
	return &Registers{
		db: db,
	}, nil
}

// Get returns the most recent updated payload for the given RegisterID.
// "most recent" means the updates happens most recent up the given height.
//
// For example, if there are 2 values stored for register A at height 6 and 11, then
// GetPayload(13, A) would return the value at height 11.
//
// If no payload is found, an empty byte slice is returned.
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
		return []byte{}, nil
	}

	binaryValue, err := iter.ValueAndErr()
	if err != nil {
		return nil, storage.ErrNotFound
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
	// value check
	upperBound := s.latestHeight + 1
	if height > upperBound || height < s.latestHeight {
		return fmt.Errorf("requested height out of bounds [%d, %d]", s.latestHeight, upperBound)
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

	err := batch.Commit(pebble.Sync)
	if err != nil {
		return fmt.Errorf("failed to commit batch: %w", err)
	}

	// increment height if necessary
	if height > s.latestHeight {
		err = s.updateLatestHeight(height)
		if err != nil {
			return err
		}
	}
	return nil
}

// LatestHeight Gets the latest height of complete registers available
func (s *Registers) LatestHeight() (uint64, error) {
	return s.heightLookup(latestHeightKey())
}

// FirstHeight first indexed height found in the store, typically root block for the spork
func (s *Registers) FirstHeight() (uint64, error) {
	return s.heightLookup(firstHeightKey())
}

func (s *Registers) heightLookup(key []byte) (uint64, error) {
	res, closer, err := s.db.Get(key)
	defer closer.Close()
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(res), nil
}

func (s *Registers) indexHeightWithKey(key []byte, height uint64) error {
	payload := make([]byte, 0)
	encoded := binary.BigEndian.AppendUint64(payload, height)
	return s.db.Set(key, encoded, nil)
}

func (s *Registers) updateFirstHeight(height uint64) error {
	err := s.indexHeightWithKey(firstHeightKey(), height)
	if err != nil {
		return fmt.Errorf("failed to update first height: %w", err)
	}
	s.firstHeight = height
	return nil
}

func (s *Registers) updateLatestHeight(height uint64) error {
	err := s.indexHeightWithKey(latestHeightKey(), height)
	if err != nil {
		return fmt.Errorf("failed to update latest height: %w", err)
	}
	s.latestHeight = height
	return nil
}
