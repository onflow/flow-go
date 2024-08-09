package badger

import (
	"errors"
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/ipfs/go-cid"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

const defaultDiscardRatio = 0.25

// getBatchItemCountLimit returns the maximum number of items that can be included in a single batch
// transaction based on the number / total size of updates per item.
func getBatchItemCountLimit(db *badger.DB, writeCountPerItem int64, writeSizePerItem int64) int {
	totalSizePerItem := 2*writeCountPerItem + writeSizePerItem // 2 bytes per entry for user and internal meta
	maxItemCountByWriteCount := db.MaxBatchCount() / writeCountPerItem
	maxItemCountByWriteSize := db.MaxBatchSize() / totalSizePerItem

	if maxItemCountByWriteCount < maxItemCountByWriteSize {
		return int(maxItemCountByWriteCount)
	} else {
		return int(maxItemCountByWriteSize)
	}
}

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

	db            *badger.DB
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
	lg := logger.With().Str("module", "tracker_storage").Logger()
	db, err := badger.Open(badger.LSMOnlyOptions(path))
	if err != nil {
		return nil, fmt.Errorf("could not open tracker db: %w", err)
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

	if err := tracker.init(startHeight); err != nil {
		return nil, fmt.Errorf("failed to initialize tracker: %w", err)
	}

	lg.Info().Msgf("tracker initialized")

	return tracker, nil
}

// init initializes the ExecutionDataTracker by setting the fulfilled and pruned heights.
//
// Parameters:
// - startHeight: The initial fulfilled height to be set if no previous fulfilled height is found.
//
// No errors are expected during normal operation.
func (s *ExecutionDataTracker) init(startHeight uint64) error {
	fulfilledHeight, fulfilledHeightErr := s.GetFulfilledHeight()
	prunedHeight, prunedHeightErr := s.GetPrunedHeight()

	if fulfilledHeightErr == nil && prunedHeightErr == nil {
		if prunedHeight > fulfilledHeight {
			return fmt.Errorf(
				"inconsistency detected: pruned height (%d) is greater than fulfilled height (%d)",
				prunedHeight,
				fulfilledHeight,
			)
		}

		s.logger.Info().Msgf("prune from height %v up to height %d", fulfilledHeight, prunedHeight)
		// replay pruning in case it was interrupted during previous shutdown
		if err := s.PruneUpToHeight(prunedHeight); err != nil {
			return fmt.Errorf("failed to replay pruning: %w", err)
		}
		s.logger.Info().Msgf("finished pruning")
	} else if errors.Is(fulfilledHeightErr, storage.ErrNotFound) && errors.Is(prunedHeightErr, storage.ErrNotFound) {
		// db is empty, we need to bootstrap it
		if err := s.db.Update(operation.InitTrackerHeights(startHeight)); err != nil {
			return fmt.Errorf("failed to bootstrap storage: %w", err)
		}
	} else {
		return multierror.Append(fulfilledHeightErr, prunedHeightErr).ErrorOrNil()
	}

	return nil
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
	err := s.db.Update(operation.UpdateTrackerFulfilledHeight(height))
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

	err := s.db.View(operation.RetrieveTrackerFulfilledHeight(&fulfilledHeight))
	if err != nil {
		return 0, err
	}

	return fulfilledHeight, nil
}

// setPrunedHeight updates the current pruned height.
//
// No errors are expected during normal operation.
func (s *ExecutionDataTracker) setPrunedHeight(height uint64) error {
	err := s.db.Update(operation.UpdateTrackerPrunedHeight(height))
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

	err := s.db.View(operation.RetrieveTrackerPrunedHeight(&prunedHeight))
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
// - tx: The BadgerDB transaction in which the operation is performed.
// - blockHeight: The height at which the blob was observed.
// - c: The CID of the blob to be tracked.
//
// No errors are expected during normal operation.
func (s *ExecutionDataTracker) trackBlob(tx *badger.Txn, blockHeight uint64, c cid.Cid) error {
	err := operation.InsertBlob(blockHeight, c)(tx)
	if err != nil {
		return fmt.Errorf("failed to add blob record: %w", err)
	}

	var latestHeight uint64
	err = operation.RetrieveTrackerLatestHeight(c, &latestHeight)(tx)
	if err != nil {
		if !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("failed to get latest height: %w", err)
		}
	} else {
		// don't update the latest height if there is already a higher block height containing this blob
		if latestHeight >= blockHeight {
			return nil
		}
	}

	err = operation.UpsertTrackerLatestHeight(c, blockHeight)(tx)
	if err != nil {
		return fmt.Errorf("failed to set latest height value: %w", err)
	}

	return nil
}

// trackBlobs tracks multiple blob CIDs at the specified block height.
//
// Parameters:
// - blockHeight: The height at which the blobs were observed.
// - cids: The CIDs of the blobs to be tracked.
//
// No errors are expected during normal operation.
func (s *ExecutionDataTracker) trackBlobs(blockHeight uint64, cids ...cid.Cid) error {
	cidsPerBatch := storage.CidsPerBatch
	maxCidsPerBatch := getBatchItemCountLimit(s.db, 2, storage.BlobRecordKeyLength+storage.LatestHeightKeyLength+8)
	if maxCidsPerBatch < cidsPerBatch {
		cidsPerBatch = maxCidsPerBatch
	}

	for len(cids) > 0 {
		batchSize := cidsPerBatch
		if len(cids) < batchSize {
			batchSize = len(cids)
		}
		batch := cids[:batchSize]

		if err := operation.RetryOnConflict(s.db.Update, func(txn *badger.Txn) error {
			for _, c := range batch {
				if err := s.trackBlob(txn, blockHeight, c); err != nil {
					return fmt.Errorf("failed to track blob %s: %w", c.String(), err)
				}
			}

			return nil
		}); err != nil {
			return err
		}

		cids = cids[batchSize:]
	}

	return nil
}

// batchDelete deletes multiple blobs from the storage in a batch operation.
//
// Parameters:
// - deleteInfos: Information about the blobs to be deleted, including their heights and whether to delete the latest height record.
//
// Ensures that all specified blobs are deleted from the storage.
//
// No errors are expected during normal operation.
func (s *ExecutionDataTracker) batchDelete(deleteInfos []*storage.DeleteInfo) error {
	batch := NewBatch(s.db)

	for _, dInfo := range deleteInfos {
		writeBatch := batch.GetWriter()
		err := operation.BatchRemoveBlob(dInfo.Height, dInfo.Cid)(writeBatch)
		if err != nil {
			return fmt.Errorf("cannot batch remove blob: %w", err)
		}

		if dInfo.DeleteLatestHeightRecord {
			err = operation.BatchRemoveTrackerLatestHeight(dInfo.Cid)(writeBatch)
			if err != nil {
				return fmt.Errorf("cannot batch remove latest height record: %w", err)
			}
		}
	}

	err := batch.Flush()
	if err != nil {
		return fmt.Errorf("cannot flush batch to remove execution data: %w", err)
	}

	return nil
}

// batchDeleteItemLimit determines the maximum number of items that can be deleted in a batch operation.
func (s *ExecutionDataTracker) batchDeleteItemLimit() int {
	itemsPerBatch := storage.DeleteItemsPerBatch
	maxItemsPerBatch := getBatchItemCountLimit(s.db, 2, storage.BlobRecordKeyLength+storage.LatestHeightKeyLength)
	if maxItemsPerBatch < itemsPerBatch {
		itemsPerBatch = maxItemsPerBatch
	}
	return itemsPerBatch
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
	itemsPerBatch := s.batchDeleteItemLimit()
	var batch []*storage.DeleteInfo

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.setPrunedHeight(height); err != nil {
		return err
	}

	if err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{
			PrefetchValues: false,
			Prefix:         blobRecordPrefix,
		})
		defer it.Close()

		// iterate over blob records, calling pruneCallback for any CIDs that should be pruned
		// and cleaning up the corresponding tracker records
		for it.Seek(blobRecordPrefix); it.ValidForPrefix(blobRecordPrefix); it.Next() {
			blobRecordItem := it.Item()
			blobRecordKey := blobRecordItem.Key()

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
			err = operation.RetrieveTrackerLatestHeight(blobCid, &latestHeight)(txn)
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
				if err := s.batchDelete(batch); err != nil {
					return err
				}
				batch = nil
			}
		}

		if len(batch) > 0 {
			if err := s.batchDelete(batch); err != nil {
				return err
			}
		}

		return nil
	}); err != nil {
		return err
	}

	// this is a good time to do garbage collection
	if err := s.db.RunValueLogGC(defaultDiscardRatio); err != nil {
		s.logger.Err(err).Msg("failed to run value log garbage collection")
	}

	return nil
}
