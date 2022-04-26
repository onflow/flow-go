package tracker

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/module/blobs"
)

// badger key prefixes
const (
	prefixGlobalState  byte = iota + 1 // global state variables
	prefixLatestHeight                 // tracks the latest height containing each blob
	prefixBlobRecord                   // tracks the set of blobs at each height
)

const (
	globalStateFulfilledHeight = iota + 1 // latest fulfilled block height
)

func retryOnConflict(db *badger.DB, fn func(txn *badger.Txn) error) error {
	for {
		err := db.Update(fn)
		if errors.Is(err, badger.ErrConflict) {
			continue
		}
		return err
	}
}

const globalStateKeyLength = 2

func makeGlobalStateKey(state byte) []byte {
	globalStateKey := make([]byte, globalStateKeyLength)
	globalStateKey[0] = prefixGlobalState
	globalStateKey[1] = state
	return globalStateKey
}

const blobRecordKeyLength = 1 + 8 + blobs.CidLength

func makeBlobRecordKey(blockHeight uint64, c cid.Cid) []byte {
	blobRecordKey := make([]byte, blobRecordKeyLength)
	blobRecordKey[0] = prefixBlobRecord
	binary.LittleEndian.PutUint64(blobRecordKey[1:], blockHeight)
	copy(blobRecordKey[1+8:], c.Bytes())
	return blobRecordKey
}

func parseBlobRecordKey(key []byte) (uint64, cid.Cid, error) {
	blockHeight := binary.LittleEndian.Uint64(key[1:])
	c, err := cid.Cast(key[1+8:])
	return blockHeight, c, err
}

const latestHeightKeyLength = 1 + blobs.CidLength

func makeLatestHeightKey(c cid.Cid) []byte {
	latestHeightKey := make([]byte, latestHeightKeyLength)
	latestHeightKey[0] = prefixLatestHeight
	copy(latestHeightKey[1:], c.Bytes())
	return latestHeightKey
}

func parseLatestHeightKey(key []byte) (cid.Cid, error) {
	return cid.Cast(key[1:])
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

type Storage struct {
	mu sync.RWMutex
	db *badger.DB
}

func (s *Storage) ProtectDelete() {
	s.mu.Lock()
}

func (s *Storage) UnprotectDelete() {
	s.mu.Unlock()
}

func (s *Storage) ProtectTrackBlobs() {
	s.mu.RLock()
}

func (s *Storage) UnprotectTrackBlobs() {
	s.mu.RUnlock()
}

func (s *Storage) SetFulfilledHeight(height uint64) error {
	fulfilledHeightKey := makeGlobalStateKey(globalStateFulfilledHeight)
	fulfilledHeightValue := make([]byte, 8)
	binary.LittleEndian.PutUint64(fulfilledHeightValue, height)

	return s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(fulfilledHeightKey, fulfilledHeightValue); err != nil {
			return fmt.Errorf("failed to set fulfilled height value: %w", err)
		}

		return nil
	})
}

func (s *Storage) GetFulfilledHeight() (uint64, error) {
	fulfilledHeightKey := makeGlobalStateKey(globalStateFulfilledHeight)
	var fulfilledHeight uint64

	if err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(fulfilledHeightKey)
		if err != nil {
			return fmt.Errorf("failed to find fulfilled height entry: %w", err)
		}

		fulfilledHeightValue, err := item.ValueCopy(nil)
		if err != nil {
			return fmt.Errorf("failed to retrieve fulfilled height value: %w", err)
		}

		fulfilledHeight = binary.LittleEndian.Uint64(fulfilledHeightValue)

		return nil
	}); err != nil {
		return 0, err
	}

	return fulfilledHeight, nil
}

func (s *Storage) trackBlob(txn *badger.Txn, blockHeight uint64, c cid.Cid) error {
	if err := txn.Set(makeBlobRecordKey(blockHeight, c), nil); err != nil {
		return fmt.Errorf("failed to add blob record: %w", err)
	}

	latestHeightKey := makeLatestHeightKey(c)
	item, err := txn.Get(latestHeightKey)
	if err != nil {
		if !errors.Is(err, badger.ErrKeyNotFound) {
			return fmt.Errorf("failed to get latest height: %w", err)
		}
	} else {
		value, err := item.ValueCopy(nil)
		if err != nil {
			return fmt.Errorf("failed to retrieve latest height value: %w", err)
		}

		// don't update the latest height if there is already a higher block height containing this blob
		latestHeight := binary.LittleEndian.Uint64(value)
		if latestHeight >= blockHeight {
			return nil
		}
	}

	latestHeightValue := make([]byte, 8)
	binary.LittleEndian.PutUint64(latestHeightValue, blockHeight)

	if err := txn.Set(latestHeightKey, latestHeightValue); err != nil {
		return fmt.Errorf("failed to set latest height value: %w", err)
	}

	return nil
}

func (s *Storage) TrackBlobs(blockHeight uint64, cids []cid.Cid) error {
	cidsPerBatch := 16
	maxCidsPerBatch := getBatchItemCountLimit(s.db, 2, blobRecordKeyLength+latestHeightKeyLength+8)
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

func OpenStorage(db *badger.DB, startHeight uint64) (*Storage, error) {
	storage := &Storage{db: db}
	if err := storage.init(startHeight); err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	return storage, nil
}

func (s *Storage) init(startHeight uint64) error {
	if _, err := s.GetFulfilledHeight(); errors.Is(err, badger.ErrKeyNotFound) {
		// db is empty, we need to bootstrap it
		s.SetFulfilledHeight(startHeight)
	} else if err != nil {
		return fmt.Errorf("failed to check storage bootstrap state: %w", err)
	}

	return nil
}

func (s *Storage) batchDelete(blobRecordCids []cid.Cid, blobRecordHeights []uint64, blobDeletable []bool) error {
	return s.db.Update(func(txn *badger.Txn) error {
		for i, c := range blobRecordCids {
			if err := txn.Delete(makeBlobRecordKey(blobRecordHeights[i], c)); err != nil {
				return fmt.Errorf("failed to delete blob record for Cid %s: %w", c.String(), err)
			}

			if blobDeletable[i] {
				if err := txn.Delete(makeLatestHeightKey(c)); err != nil {
					return fmt.Errorf("failed to delete latest height record for Cid %s: %w", c.String(), err)
				}
			}
		}

		return nil
	})
}

func (s *Storage) deleteFunc(blobRecordCids []cid.Cid, blobRecordHeights []uint64, blobDeletable []bool) func() error {
	return func() error {
		recordsPerBatch := 256
		maxCidsPerBatch := getBatchItemCountLimit(s.db, 2, blobRecordKeyLength+latestHeightKeyLength)
		if maxCidsPerBatch < recordsPerBatch {
			recordsPerBatch = maxCidsPerBatch
		}

		for len(blobRecordCids) > 0 {
			batchSize := recordsPerBatch
			if len(blobRecordCids) < batchSize {
				batchSize = len(blobRecordCids)
			}

			if err := s.batchDelete(blobRecordCids[:batchSize], blobRecordHeights[:batchSize], blobDeletable[:batchSize]); err != nil {
				return fmt.Errorf("failed to delete batch: %w", err)
			}

			blobRecordCids = blobRecordCids[batchSize:]
			blobRecordHeights = blobRecordHeights[batchSize:]
			blobDeletable = blobDeletable[batchSize:]
		}

		return nil
	}
}

func (s *Storage) PrepareForDeletion(height uint64) ([]cid.Cid, func() error, error) {
	var deletableBlobs []cid.Cid
	var blobRecordCids []cid.Cid
	var blobRecordHeights []uint64
	var blobDeletable []bool
	blobRecordPrefix := []byte{prefixBlobRecord}

	if err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{
			PrefetchValues: false,
			Prefix:         blobRecordPrefix,
		})
		defer it.Close()

		for it.Seek(blobRecordPrefix); it.ValidForPrefix(blobRecordPrefix); it.Next() {
			blobRecordItem := it.Item()
			blobRecordKey := blobRecordItem.Key()

			blockHeight, blobCid, err := parseBlobRecordKey(blobRecordKey)
			if err != nil {
				return fmt.Errorf("malformed blob record key %v: %w", blobRecordKey, err)
			}

			if blockHeight > height {
				break
			}

			latestHeightKey := makeLatestHeightKey(blobCid)
			latestHeightItem, err := txn.Get(latestHeightKey)
			if err != nil {
				return fmt.Errorf("failed to get latest height entry for Cid %s: %w", blobCid.String(), err)
			}

			latestHeightValue, err := latestHeightItem.ValueCopy(nil)
			if err != nil {
				return fmt.Errorf("failed to retrieve latest height value for Cid %s: %w", blobCid.String(), err)
			}

			// a blob is only removable if it is not referenced by any blob tree at a higher height
			latestHeight := binary.LittleEndian.Uint64(latestHeightValue)
			if latestHeight < blockHeight {
				// this should never happen
				return fmt.Errorf(
					"inconsistency detected: latest height recorded for Cid %s is %d, but blob record exists at height %d",
					blobCid.String(), latestHeight, blockHeight,
				)
			} else if latestHeight == blockHeight {
				deletableBlobs = append(deletableBlobs, blobCid)
			}

			blobRecordCids = append(blobRecordCids, blobCid)
			blobRecordHeights = append(blobRecordHeights, blockHeight)
			blobDeletable = append(blobDeletable, latestHeight == blockHeight)
		}

		return nil
	}); err != nil {
		return nil, nil, err
	}

	return deletableBlobs, s.deleteFunc(blobRecordCids, blobRecordHeights, blobDeletable), nil
}
