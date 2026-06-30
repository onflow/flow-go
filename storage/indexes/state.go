package indexes

import (
	"errors"
	"fmt"

	"github.com/jordanschalm/lockctx"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

// IndexState manages height tracking for a block-height-indexed store.
// Concrete index types embed *IndexState to inherit height management.
//
// All read methods are safe for concurrent access. Write methods (PrepareStore) must be called
// sequentially, and the caller must hold the required lock until the batch is committed.
type IndexState struct {
	db            storage.DB
	firstHeight   uint64
	latestHeight  *atomic.Uint64
	requiredLock  string
	lowerBoundKey []byte
	upperBoundKey []byte
}

// New reads the first and latest heights from the DB and returns a new IndexState.
//
// Expected error returns during normal operation:
//   - [storage.ErrNotBootstrapped]: if the index has not been initialized
func NewIndexState(db storage.DB, requiredLock string, lowerBoundKey, upperBoundKey []byte) (*IndexState, error) {
	firstHeight, err := readHeight(db.Reader(), lowerBoundKey)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, storage.ErrNotBootstrapped
		}
		return nil, fmt.Errorf("could not get first height: %w", err)
	}

	latestHeight, err := readHeight(db.Reader(), upperBoundKey)
	if err != nil {
		return nil, fmt.Errorf("could not get latest height: %w", err)
	}

	return &IndexState{
		db:            db,
		firstHeight:   firstHeight,
		latestHeight:  atomic.NewUint64(latestHeight),
		requiredLock:  requiredLock,
		lowerBoundKey: lowerBoundKey,
		upperBoundKey: upperBoundKey,
	}, nil
}

// Bootstrap writes the bounds keys at initialStartHeight to the batch and returns a new IndexState.
// The caller is responsible for writing initial items to the same batch before committing.
//
// Expected error returns during normal operation:
//   - [storage.ErrAlreadyExists]: if either bounds key already exists
func BootstrapIndexState(
	lctx lockctx.Proof,
	rw storage.ReaderBatchWriter,
	db storage.DB,
	requiredLock string,
	lowerBoundKey, upperBoundKey []byte,
	initialStartHeight uint64,
) (*IndexState, error) {
	if !lctx.HoldsLock(requiredLock) {
		return nil, fmt.Errorf("missing required lock: %s", requiredLock)
	}

	for _, key := range [][]byte{lowerBoundKey, upperBoundKey} {
		exists, err := operation.KeyExists(rw.GlobalReader(), key)
		if err != nil {
			return nil, fmt.Errorf("could not check bounds key: %w", err)
		}
		if exists {
			return nil, fmt.Errorf("bounds key already exists: %w", storage.ErrAlreadyExists)
		}
	}

	writer := rw.Writer()
	if err := operation.UpsertByKey(writer, lowerBoundKey, initialStartHeight); err != nil {
		return nil, fmt.Errorf("could not set first height: %w", err)
	}
	if err := operation.UpsertByKey(writer, upperBoundKey, initialStartHeight); err != nil {
		return nil, fmt.Errorf("could not set latest height: %w", err)
	}

	// the caller is responsible for writing initial data to the batch before committing

	return &IndexState{
		db:            db,
		firstHeight:   initialStartHeight,
		latestHeight:  atomic.NewUint64(initialStartHeight),
		requiredLock:  requiredLock,
		lowerBoundKey: lowerBoundKey,
		upperBoundKey: upperBoundKey,
	}, nil
}

// FirstIndexedHeight returns the first (oldest) indexed height.
func (s *IndexState) FirstIndexedHeight() uint64 {
	return s.firstHeight
}

// LatestIndexedHeight returns the latest indexed height.
func (s *IndexState) LatestIndexedHeight() uint64 {
	return s.latestHeight.Load()
}

// PrepareStore validates that blockHeight is the next consecutive height, verifies the required
// lock is held, writes the upper bound key update to the batch, and registers an OnCommitSucceed
// callback to advance the in-memory latestHeight.
//
// Expected error returns during normal operation:
//   - [storage.ErrAlreadyExists]: if blockHeight is already indexed
func (s *IndexState) PrepareStore(lctx lockctx.Proof, rw storage.ReaderBatchWriter, blockHeight uint64) error {
	if !lctx.HoldsLock(s.requiredLock) {
		return fmt.Errorf("missing required lock: %s", s.requiredLock)
	}

	// make sure the block height is the next consecutive height
	expected := s.latestHeight.Load() + 1
	if blockHeight < expected {
		return storage.ErrAlreadyExists
	}
	if blockHeight > expected {
		return fmt.Errorf("must index consecutive heights: expected %d, got %d", expected, blockHeight)
	}

	// sanity check that the stored height is in sync with the in-memory height
	storedLatestHeight, err := readHeight(rw.GlobalReader(), s.upperBoundKey)
	if err != nil {
		return fmt.Errorf("could not get latest indexed height: %w", err)
	}
	if blockHeight != storedLatestHeight+1 {
		return fmt.Errorf("must index consecutive heights: expected %d, got %d", storedLatestHeight+1, blockHeight)
	}

	if err := operation.UpsertByKey(rw.Writer(), s.upperBoundKey, blockHeight); err != nil {
		return fmt.Errorf("could not update latest height: %w", err)
	}

	storage.OnCommitSucceed(rw, func() {
		s.latestHeight.Store(blockHeight)
	})

	return nil
}
