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

// The ExecutionDataTracker component tracks the following information:
//   - the latest pruned height
//   - the latest fulfilled height
//   - the set of CIDs of the execution data blobs we know about at each height, so that
//     once we prune a fulfilled height we can remove the blob data from local storage
//   - for each CID, the most recent height that it was observed at, so that when pruning
//     a fulfilled height we don't remove any blob data that is still needed at higher heights
//
// The storage component calls the given prune callback for a CID when the last height
// at which that CID appears is pruned. The prune callback can be used to delete the
// corresponding blob data from the blob store.
type ExecutionDataTracker struct {
	// ensures that pruning operations are not run concurrently with any other db writes
	// we acquire the read lock when we want to perform a non-prune WRITE
	// we acquire the write lock when we want to perform a prune WRITE
	mu sync.RWMutex

	db            *badger.DB
	pruneCallback storage.PruneCallback
	logger        zerolog.Logger
}

func WithPruneCallback(callback storage.PruneCallback) StorageOption {
	return func(s *ExecutionDataTracker) {
		s.pruneCallback = callback
	}
}

func NewExecutionDataTracker(dbPath string, startHeight uint64, logger zerolog.Logger, opts ...StorageOption) (*ExecutionDataTracker, error) {
	lg := logger.With().Str("module", "tracker_storage").Logger()
	db, err := badger.Open(badger.LSMOnlyOptions(dbPath))
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
		if err := s.bootstrap(startHeight); err != nil {
			return fmt.Errorf("failed to bootstrap storage: %w", err)
		}
	} else {
		return multierror.Append(fulfilledHeightErr, prunedHeightErr).ErrorOrNil()
	}

	return nil
}

func (s *ExecutionDataTracker) bootstrap(startHeight uint64) error {
	err := s.db.Update(operation.InsertTrackerFulfilledHeight(startHeight))
	if err != nil {
		return fmt.Errorf("failed to set fulfilled height value: %w", err)
	}

	err = s.db.Update(operation.InsertTrackerPrunedHeight(startHeight))
	if err != nil {
		return fmt.Errorf("failed to set pruned height value: %w", err)
	}

	return nil
}

func (s *ExecutionDataTracker) Update(f storage.UpdateFn) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return f(s.trackBlobs)
}

func (s *ExecutionDataTracker) SetFulfilledHeight(height uint64) error {
	err := s.db.Update(operation.UpdateTrackerFulfilledHeight(height))
	if err != nil {
		return fmt.Errorf("failed to set fulfilled height value: %w", err)
	}

	return nil
}

func (s *ExecutionDataTracker) GetFulfilledHeight() (uint64, error) {
	var fulfilledHeight uint64

	err := s.db.View(operation.RetrieveTrackerFulfilledHeight(&fulfilledHeight))
	if err != nil {
		return 0, err
	}

	return fulfilledHeight, nil
}

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

func (s *ExecutionDataTracker) batchDelete(deleteInfos []*storage.DeleteInfo) error {
	for _, dInfo := range deleteInfos {
		err := s.db.Update(operation.RemoveBlob(dInfo.Height, dInfo.Cid))
		if err != nil {
			return fmt.Errorf("failed to delete blob record for Cid %s: %w", dInfo.Cid.String(), err)
		}

		if dInfo.DeleteLatestHeightRecord {
			err = s.db.Update(operation.RemoveTrackerLatestHeight(dInfo.Cid))
			if err != nil {
				return fmt.Errorf("failed to delete latest height record for Cid %s: %w", dInfo.Cid.String(), err)
			}
		}
	}

	return nil
}

func (s *ExecutionDataTracker) batchDeleteItemLimit() int {
	itemsPerBatch := storage.DeleteItemsPerBatch
	maxItemsPerBatch := getBatchItemCountLimit(s.db, 2, storage.BlobRecordKeyLength+storage.LatestHeightKeyLength)
	if maxItemsPerBatch < itemsPerBatch {
		itemsPerBatch = maxItemsPerBatch
	}
	return itemsPerBatch
}

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
	if err := s.db.RunValueLogGC(0.5); err != nil {
		s.logger.Err(err).Msg("failed to run value log garbage collection")
	}

	return nil
}

func (s *ExecutionDataTracker) setPrunedHeight(height uint64) error {
	err := s.db.Update(operation.UpdateTrackerPrunedHeight(height))
	if err != nil {
		return fmt.Errorf("failed to set pruned height value: %w", err)
	}

	return nil
}

func (s *ExecutionDataTracker) GetPrunedHeight() (uint64, error) {
	var prunedHeight uint64

	err := s.db.View(operation.RetrieveTrackerPrunedHeight(&prunedHeight))
	if err != nil {
		return 0, err
	}

	return prunedHeight, nil
}
