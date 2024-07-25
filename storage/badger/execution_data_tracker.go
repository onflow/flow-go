package badger

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/ipfs/go-cid"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/executiondatasync/tracker"
)

func getUint64Value(item *badger.Item) (uint64, error) {
	value, err := item.ValueCopy(nil)
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint64(value), nil
}

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

func retryOnConflict(db *badger.DB, fn func(txn *badger.Txn) error) error {
	for {
		err := db.Update(fn)
		if errors.Is(err, badger.ErrConflict) {
			continue
		}
		return err
	}
}

type StorageOption func(*Storage)

var _ tracker.Storage = (*Storage)(nil)

// The Storage component tracks the following information:
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
type Storage struct {
	// ensures that pruning operations are not run concurrently with any other db writes
	// we acquire the read lock when we want to perform a non-prune WRITE
	// we acquire the write lock when we want to perform a prune WRITE
	mu sync.RWMutex

	db            *badger.DB
	pruneCallback tracker.PruneCallback
	logger        zerolog.Logger
}

func WithPruneCallback(callback tracker.PruneCallback) StorageOption {
	return func(s *Storage) {
		s.pruneCallback = callback
	}
}

func NewStorageTracker(dbPath string, startHeight uint64, logger zerolog.Logger, opts ...StorageOption) (*Storage, error) {
	lg := logger.With().Str("module", "tracker_storage").Logger()
	db, err := badger.Open(badger.LSMOnlyOptions(dbPath))
	if err != nil {
		return nil, fmt.Errorf("could not open tracker db: %w", err)
	}

	storage := &Storage{
		db:            db,
		pruneCallback: func(c cid.Cid) error { return nil },
		logger:        lg,
	}

	for _, opt := range opts {
		opt(storage)
	}

	lg.Info().Msgf("initialize storage with start height: %d", startHeight)

	if err := storage.init(startHeight); err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	lg.Info().Msgf("storage initialized")

	return storage, nil
}

func (s *Storage) init(startHeight uint64) error {
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
	} else if errors.Is(fulfilledHeightErr, badger.ErrKeyNotFound) && errors.Is(prunedHeightErr, badger.ErrKeyNotFound) {
		// db is empty, we need to bootstrap it
		if err := s.bootstrap(startHeight); err != nil {
			return fmt.Errorf("failed to bootstrap storage: %w", err)
		}
	} else {
		return multierror.Append(fulfilledHeightErr, prunedHeightErr).ErrorOrNil()
	}

	return nil
}

func (s *Storage) bootstrap(startHeight uint64) error {
	fulfilledHeightKey := tracker.MakeGlobalStateKey(tracker.GlobalStateFulfilledHeight)
	fulfilledHeightValue := tracker.MakeUint64Value(startHeight)

	prunedHeightKey := tracker.MakeGlobalStateKey(tracker.GlobalStatePrunedHeight)
	prunedHeightValue := tracker.MakeUint64Value(startHeight)

	return s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(fulfilledHeightKey, fulfilledHeightValue); err != nil {
			return fmt.Errorf("failed to set fulfilled height value: %w", err)
		}

		if err := txn.Set(prunedHeightKey, prunedHeightValue); err != nil {
			return fmt.Errorf("failed to set pruned height value: %w", err)
		}

		return nil
	})
}

func (s *Storage) Update(f tracker.UpdateFn) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return f(s.trackBlobs)
}

func (s *Storage) SetFulfilledHeight(height uint64) error {
	fulfilledHeightKey := tracker.MakeGlobalStateKey(tracker.GlobalStateFulfilledHeight)
	fulfilledHeightValue := tracker.MakeUint64Value(height)

	return s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(fulfilledHeightKey, fulfilledHeightValue); err != nil {
			return fmt.Errorf("failed to set fulfilled height value: %w", err)
		}

		return nil
	})
}

func (s *Storage) GetFulfilledHeight() (uint64, error) {
	fulfilledHeightKey := tracker.MakeGlobalStateKey(tracker.GlobalStateFulfilledHeight)
	var fulfilledHeight uint64

	if err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(fulfilledHeightKey)
		if err != nil {
			return fmt.Errorf("failed to find fulfilled height entry: %w", err)
		}

		fulfilledHeight, err = getUint64Value(item)
		if err != nil {
			return fmt.Errorf("failed to retrieve fulfilled height value: %w", err)
		}

		return nil
	}); err != nil {
		return 0, err
	}

	return fulfilledHeight, nil
}

func (s *Storage) trackBlob(txn *badger.Txn, blockHeight uint64, c cid.Cid) error {
	if err := txn.Set(tracker.MakeBlobRecordKey(blockHeight, c), nil); err != nil {
		return fmt.Errorf("failed to add blob record: %w", err)
	}

	latestHeightKey := tracker.MakeLatestHeightKey(c)
	item, err := txn.Get(latestHeightKey)
	if err != nil {
		if !errors.Is(err, badger.ErrKeyNotFound) {
			return fmt.Errorf("failed to get latest height: %w", err)
		}
	} else {
		latestHeight, err := getUint64Value(item)
		if err != nil {
			return fmt.Errorf("failed to retrieve latest height value: %w", err)
		}

		// don't update the latest height if there is already a higher block height containing this blob
		if latestHeight >= blockHeight {
			return nil
		}
	}

	latestHeightValue := tracker.MakeUint64Value(blockHeight)

	if err := txn.Set(latestHeightKey, latestHeightValue); err != nil {
		return fmt.Errorf("failed to set latest height value: %w", err)
	}

	return nil
}

func (s *Storage) trackBlobs(blockHeight uint64, cids ...cid.Cid) error {
	cidsPerBatch := tracker.CidsPerBatch
	maxCidsPerBatch := getBatchItemCountLimit(s.db, 2, tracker.BlobRecordKeyLength+tracker.LatestHeightKeyLength+8)
	if maxCidsPerBatch < cidsPerBatch {
		cidsPerBatch = maxCidsPerBatch
	}

	for len(cids) > 0 {
		batchSize := cidsPerBatch
		if len(cids) < batchSize {
			batchSize = len(cids)
		}
		batch := cids[:batchSize]

		if err := retryOnConflict(s.db, func(txn *badger.Txn) error {
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

func (s *Storage) batchDelete(deleteInfos []*deleteInfo) error {
	return s.db.Update(func(txn *badger.Txn) error {
		for _, dInfo := range deleteInfos {
			if err := txn.Delete(tracker.MakeBlobRecordKey(dInfo.height, dInfo.cid)); err != nil {
				return fmt.Errorf("failed to delete blob record for Cid %s: %w", dInfo.cid.String(), err)
			}

			if dInfo.deleteLatestHeightRecord {
				if err := txn.Delete(tracker.MakeLatestHeightKey(dInfo.cid)); err != nil {
					return fmt.Errorf("failed to delete latest height record for Cid %s: %w", dInfo.cid.String(), err)
				}
			}
		}

		return nil
	})
}

func (s *Storage) batchDeleteItemLimit() int {
	itemsPerBatch := 256
	maxItemsPerBatch := getBatchItemCountLimit(s.db, 2, tracker.BlobRecordKeyLength+tracker.LatestHeightKeyLength)
	if maxItemsPerBatch < itemsPerBatch {
		itemsPerBatch = maxItemsPerBatch
	}
	return itemsPerBatch
}

func (s *Storage) PruneUpToHeight(height uint64) error {
	blobRecordPrefix := []byte{tracker.PrefixBlobRecord}
	itemsPerBatch := s.batchDeleteItemLimit()
	var batch []*deleteInfo

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

			blockHeight, blobCid, err := tracker.ParseBlobRecordKey(blobRecordKey)
			if err != nil {
				return fmt.Errorf("malformed blob record key %v: %w", blobRecordKey, err)
			}

			// iteration occurs in key order, so block heights are guaranteed to be ascending
			if blockHeight > height {
				break
			}

			dInfo := &deleteInfo{
				cid:    blobCid,
				height: blockHeight,
			}

			latestHeightKey := tracker.MakeLatestHeightKey(blobCid)
			latestHeightItem, err := txn.Get(latestHeightKey)
			if err != nil {
				return fmt.Errorf("failed to get latest height entry for Cid %s: %w", blobCid.String(), err)
			}

			latestHeight, err := getUint64Value(latestHeightItem)
			if err != nil {
				return fmt.Errorf("failed to retrieve latest height value for Cid %s: %w", blobCid.String(), err)
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
				dInfo.deleteLatestHeightRecord = true
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

func (s *Storage) setPrunedHeight(height uint64) error {
	prunedHeightKey := tracker.MakeGlobalStateKey(tracker.GlobalStatePrunedHeight)
	prunedHeightValue := tracker.MakeUint64Value(height)

	return s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(prunedHeightKey, prunedHeightValue); err != nil {
			return fmt.Errorf("failed to set pruned height value: %w", err)
		}

		return nil
	})
}

func (s *Storage) GetPrunedHeight() (uint64, error) {
	prunedHeightKey := tracker.MakeGlobalStateKey(tracker.GlobalStatePrunedHeight)
	var prunedHeight uint64

	if err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(prunedHeightKey)
		if err != nil {
			return fmt.Errorf("failed to find pruned height entry: %w", err)
		}

		prunedHeight, err = getUint64Value(item)
		if err != nil {
			return fmt.Errorf("failed to retrieve pruned height value: %w", err)
		}

		return nil
	}); err != nil {
		return 0, err
	}

	return prunedHeight, nil
}

type deleteInfo struct {
	cid                      cid.Cid
	height                   uint64
	deleteLatestHeightRecord bool
}
