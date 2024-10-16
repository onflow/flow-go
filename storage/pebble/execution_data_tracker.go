package pebble

import (
	"bytes"
	"errors"
	"fmt"
	"sync"

	"github.com/cockroachdb/pebble"
	"github.com/ipfs/go-cid"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/pebble/operation"
)

// The `ffBytes` constant is used to ensure that all keys with a specific prefix are included during forward iteration.
// By appending a suffix of 0xff bytes to the prefix, all keys that start with the given prefix are captured before
// completing the iteration.
var ffBytes = bytes.Repeat([]byte{0xFF}, storage.BlobRecordKeyLength)

type StorageOption func(*ExecutionDataTracker)

var _ storage.ExecutionDataTracker = (*ExecutionDataTracker)(nil)

// The ExecutionDataTracker component manages the following information:
//   - The latest pruned height
//   - The latest fulfilled height
//   - The set of CIDs of execution data blobs known at each height, allowing removal of blob data from local storage
//     once a fulfilled height is pruned
//   - For each CID, the most recent height at which it was observed, ensuring that blob data needed at higher heights
//     is not removed during pruning
//
// The component invokes the provided prune callback for a CID when the last height at which that CID appears is pruned.
// This callback can be used to delete the corresponding blob data from the blob store.
type ExecutionDataTracker struct {
	// Ensures that pruning operations are not run concurrently with other database writes.
	// Acquires the read lock for non-prune WRITE operations.
	// Acquires the write lock for prune WRITE operations.
	mu sync.RWMutex

	db            *pebble.DB
	pruneCallback storage.PruneCallback
	logger        zerolog.Logger
}

// WithPruneCallback is used to configure the ExecutionDataTracker with a custom prune callback.
func WithPruneCallback(callback storage.PruneCallback) StorageOption {
	return func(s *ExecutionDataTracker) {
		s.pruneCallback = callback
	}
}

// NewExecutionDataTracker initializes a new ExecutionDataTracker.
//
// Parameters:
// - logger: The logger for logging tracker operations.
// - path: The file path for the underlying Pebble database.
// - startHeight: The initial fulfilled height to be set if no previous fulfilled height is found.
// - opts: Additional configuration options such as custom prune callbacks.
//
// No errors are expected during normal operation.
func NewExecutionDataTracker(
	logger zerolog.Logger,
	path string,
	startHeight uint64,
	opts ...StorageOption,
) (*ExecutionDataTracker, error) {
	lg := logger.With().Str("module", "pebble_storage_tracker").Logger()
	db, err := pebble.Open(path, nil)
	if err != nil {
		return nil, fmt.Errorf("could not open db: %w", err)
	}

	tracker := &ExecutionDataTracker{
		db:            db,
		pruneCallback: func(c cid.Cid) error { return nil },
		logger:        lg,
	}

	for _, opt := range opts {
		opt(tracker)
	}

	lg.Info().Msgf("initialize tracker with start height: %d", startHeight)

	err = storage.InitTracker(
		tracker,
		lg,
		startHeight,
		func(startHeight uint64) error {
			return operation.WithReaderBatchWriter(db, operation.InitTrackerHeights(startHeight))
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize tracker: %w", err)
	}

	lg.Info().Msgf("tracker initialized")

	return tracker, nil
}

// Update is used to track new blob CIDs.
// It can be used to track blobs for both sealed and unsealed
// heights, and the same blob may be added multiple times for
// different heights.
// The same blob may also be added multiple times for the same
// height, but it will only be tracked once per height.
//
// No errors are expected during normal operation.
func (s *ExecutionDataTracker) Update(f storage.UpdateFn) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return f(s.trackBlobs)
}

// SetFulfilledHeight updates the fulfilled height value,
// which is the lowest from the highest block heights `h` such that all
// heights <= `h` are sealed and the sealed execution data
// has been downloaded or indexed.
// It is up to the caller to ensure that this is never
// called with a value lower than the pruned height.
//
// No errors are expected during normal operation
func (s *ExecutionDataTracker) SetFulfilledHeight(height uint64) error {
	err := operation.UpdateTrackerFulfilledHeight(height)(s.db)
	if err != nil {
		return fmt.Errorf("failed to set fulfilled height value: %w", err)
	}

	return nil
}

// GetFulfilledHeight returns the current fulfilled height.
//
// No errors are expected during normal operation.
func (s *ExecutionDataTracker) GetFulfilledHeight() (uint64, error) {
	var fulfilledHeight uint64

	err := operation.RetrieveTrackerFulfilledHeight(&fulfilledHeight)(s.db)
	if err != nil {
		return 0, err
	}

	return fulfilledHeight, nil
}

// setPrunedHeight updates the current pruned height.
//
// No errors are expected during normal operation.
func (s *ExecutionDataTracker) setPrunedHeight(height uint64) error {
	err := operation.UpdateTrackerPrunedHeight(height)(s.db)
	if err != nil {
		return fmt.Errorf("failed to set pruned height value: %w", err)
	}

	return nil
}

// GetPrunedHeight returns the current pruned height.
//
// No errors are expected during normal operation.
func (s *ExecutionDataTracker) GetPrunedHeight() (uint64, error) {
	var prunedHeight uint64

	err := operation.RetrieveTrackerPrunedHeight(&prunedHeight)(s.db)
	if err != nil {
		return 0, err
	}

	return prunedHeight, nil
}

// trackBlob tracks a single blob CID at the specified block height.
// This method first inserts a record of the blob at the specified block height.
// It then checks if the current block height is greater than the previously recorded
// latest height for this CID. If so, it updates the latest height to the current block height.
//
// Parameters:
// - blockHeight: The height at which the blob was observed.
// - c: The CID of the blob to be tracked.
//
// No errors are expected during normal operation.
func (s *ExecutionDataTracker) trackBlob(blockHeight uint64, c cid.Cid) func(tx storage.PebbleReaderBatchWriter) error {
	return func(tx storage.PebbleReaderBatchWriter) error {
		_, w := tx.ReaderWriter()
		err := operation.InsertBlob(blockHeight, c)(w)
		if err != nil {
			return fmt.Errorf("failed to add blob record: %w", err)
		}

		var latestHeight uint64
		err = operation.RetrieveTrackerLatestHeight(c, &latestHeight)(s.db)
		if err != nil {
			if !errors.Is(err, storage.ErrNotFound) {
				return fmt.Errorf("failed to get latest height: %w", err)
			}
		} else if latestHeight >= blockHeight {
			// don't update the latest height if there is already a higher block height containing this blob
			return nil
		}

		err = operation.UpsertTrackerLatestHeight(c, blockHeight)(w)
		if err != nil {
			return fmt.Errorf("failed to set latest height value: %w", err)
		}

		return nil
	}
}

// trackBlobs tracks multiple blob CIDs at the specified block height.
//
// Parameters:
// - blockHeight: The height at which the blobs were observed.
// - cids: The CIDs of the blobs to be tracked.
//
// No errors are expected during normal operation.
func (s *ExecutionDataTracker) trackBlobs(blockHeight uint64, cids ...cid.Cid) error {
	if len(cids) > 0 {
		return operation.WithReaderBatchWriter(s.db, func(tx storage.PebbleReaderBatchWriter) error {
			for _, c := range cids {
				if err := s.trackBlob(blockHeight, c)(tx); err != nil {
					return fmt.Errorf("failed to track blob %s: %w", c.String(), err)
				}
			}

			return nil
		})
	}

	return nil
}

// batchDelete deletes multiple blobs from the storage in a batch operation.
//
// Parameters:
// - deleteInfos: Information about the blobs to be deleted, including their heights and
// whether to delete the latest height record.
//
// No errors are expected during normal operation.
func (s *ExecutionDataTracker) batchDelete(deleteInfos []*storage.DeleteInfo) func(storage.PebbleReaderBatchWriter) error {
	return func(tx storage.PebbleReaderBatchWriter) error {
		_, w := tx.ReaderWriter()

		for _, dInfo := range deleteInfos {
			err := operation.RemoveBlob(dInfo.Height, dInfo.Cid)(w)
			if err != nil {
				return fmt.Errorf("failed to delete blob record for Cid %s: %w", dInfo.Cid.String(), err)
			}

			if dInfo.DeleteLatestHeightRecord {
				err = operation.RemoveTrackerLatestHeight(dInfo.Cid)(w)
				if err != nil {
					return fmt.Errorf("failed to delete latest height record for Cid %s: %w", dInfo.Cid.String(), err)
				}
			}
		}

		return nil
	}
}

// PruneUpToHeight removes all data from storage corresponding
// to block heights up to and including the given height,
// and updates the latest pruned height value.
// It locks the ExecutionDataTracker and ensures that no other writes
// can occur during the pruning.
// It is up to the caller to ensure that this is never
// called with a value higher than the fulfilled height.
//
// No errors are expected during normal operation.
func (s *ExecutionDataTracker) PruneUpToHeight(height uint64) error {
	blobRecordPrefix := []byte{storage.PrefixBlobRecord}
	itemsPerBatch := storage.DeleteItemsPerBatch
	var batch []*storage.DeleteInfo

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.setPrunedHeight(height); err != nil {
		return err
	}

	err := func(r pebble.Reader) error {
		options := pebble.IterOptions{
			LowerBound: blobRecordPrefix,
			UpperBound: append(blobRecordPrefix, ffBytes...),
		}

		it, err := r.NewIter(&options)
		if err != nil {
			return fmt.Errorf("can not create iterator: %w", err)
		}
		defer it.Close()

		// iterate over blob records, calling pruneCallback for any CIDs that should be pruned
		// and cleaning up the corresponding tracker records
		for it.SeekGE(blobRecordPrefix); it.Valid(); it.Next() {
			blobRecordKey := it.Key()

			blockHeight, blobCid, err := storage.ParseBlobRecordKey(blobRecordKey)
			if err != nil {
				return fmt.Errorf("malformed blob record key %v: %w", blobRecordKey, err)
			}

			// iteration occurs in key order, so block heights are guaranteed to be ascending
			if blockHeight > height {
				break
			}

			dInfo := &storage.DeleteInfo{
				Cid:    blobCid,
				Height: blockHeight,
			}

			var latestHeight uint64
			err = operation.RetrieveTrackerLatestHeight(blobCid, &latestHeight)(r)
			if err != nil {
				return fmt.Errorf("failed to get latest height entry for Cid %s: %w", blobCid.String(), err)
			}

			// a blob is only removable if it is not referenced by any blob tree at a higher height
			if latestHeight < blockHeight {
				// this should never happen
				return fmt.Errorf(
					"inconsistency detected: latest height recorded for Cid %s is %d, but blob record exists at height %d",
					blobCid.String(), latestHeight, blockHeight,
				)
			}

			// the current block height is the last to reference this CID, prune the CID and remove
			// all tracker records
			if latestHeight == blockHeight {
				if err := s.pruneCallback(blobCid); err != nil {
					return err
				}
				dInfo.DeleteLatestHeightRecord = true
			}

			// remove tracker records for pruned heights
			batch = append(batch, dInfo)
			if len(batch) == itemsPerBatch {
				if err := operation.WithReaderBatchWriter(s.db, s.batchDelete(batch)); err != nil {
					return err
				}
				batch = nil
			}
		}

		if len(batch) > 0 {
			if err := operation.WithReaderBatchWriter(s.db, s.batchDelete(batch)); err != nil {
				return err
			}
		}

		return nil
	}(s.db)
	if err != nil {
		return err
	}
	return nil
}
