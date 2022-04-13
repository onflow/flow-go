package state_synchronization

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
	"github.com/onflow/flow-go/network"
)

var _ StatusTrackerFactory = (*Storage)(nil)

// badger key prefixes
const (
	prefixGlobalState         byte = iota + 1 // global state variables
	prefixExecutionDataStatus                 // tracks the status of each execution data
	prefixBlobRecord                          // tracks the latest height among all blob trees containing a blob
	prefixBlobTreeRecord                      // tracks the set of blobs for each blob tree
)

const (
	executionDataStatusInProgress byte = iota + 1 // tracked but not yet fully downloaded
	executionDataStatusComplete                   // fully downloaded but not yet incorporated (ancestors not yet downloaded)
)

const (
	globalStateLatestIncorporatedHeight = iota + 1 // block height of the latest incorporated blob tree
	globalStateStoredDataLowerBound                // block height lower bound for data stored in the DB
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

const executionDataStatusKeyLength = 1 + 8

func makeExecutionDataStatusKey(blockHeight uint64) []byte {
	executionDataStatusKey := make([]byte, executionDataStatusKeyLength)
	executionDataStatusKey[0] = prefixExecutionDataStatus
	binary.LittleEndian.PutUint64(executionDataStatusKey[1:], blockHeight)
	return executionDataStatusKey
}

func parseExecutionDataStatusKey(key []byte) uint64 {
	return binary.LittleEndian.Uint64(key[1:])
}

const blobTreeRecordKeyLength = 1 + 8 + blobs.CidLength

func makeBlobTreeRecordKey(blockHeight uint64, c cid.Cid) []byte {
	blobTreeRecordKey := make([]byte, blobTreeRecordKeyLength)
	blobTreeRecordKey[0] = prefixBlobTreeRecord
	binary.LittleEndian.PutUint64(blobTreeRecordKey[1:], blockHeight)
	copy(blobTreeRecordKey[1+8:], c.Bytes())
	return blobTreeRecordKey
}

func parseBlobTreeRecordKey(key []byte) (uint64, cid.Cid, error) {
	blockHeight := binary.LittleEndian.Uint64(key[1:])
	c, err := cid.Cast(key[1+8:])
	return blockHeight, c, err
}

const blobRecordKeyLength = 1 + blobs.CidLength

func makeBlobRecordKey(c cid.Cid) []byte {
	blobRecordKey := make([]byte, blobRecordKeyLength)
	blobRecordKey[0] = prefixBlobRecord
	copy(blobRecordKey[1:], c.Bytes())
	return blobRecordKey
}

func parseBlobRecordKey(key []byte) (cid.Cid, error) {
	return cid.Cast(key[1:])
}

func makeExecutionDataStatusValue(blockID, rootID flow.Identifier) []byte {
	executionDataStatusValue := make([]byte, 2*flow.IdentifierLen)
	copy(executionDataStatusValue, blockID[:])
	copy(executionDataStatusValue[flow.IdentifierLen:], rootID[:])
	return executionDataStatusValue
}

func parseExecutionDataStatusValue(val []byte) (flow.Identifier, flow.Identifier) {
	return flow.HashToID(val[:flow.IdentifierLen]), flow.HashToID(val[flow.IdentifierLen:])
}

// getBatchItemCountLimit returns the maximum number of items that can be included in a single batch
// transaction based on the number / total size of updates per item.
func getBatchItemCountLimit(
	db *badger.DB,
	baseWriteCount int64,
	baseWriteSize int64,
	writeCountPerItem int64,
	writeSizePerItem int64,
) int {
	maxItemCountByWriteCount := (db.MaxBatchCount() - baseWriteCount) / writeCountPerItem
	maxItemCountByWriteSize := (db.MaxBatchSize() - (2*baseWriteCount + baseWriteSize)) / (2*writeCountPerItem + writeSizePerItem)

	if maxItemCountByWriteCount < maxItemCountByWriteSize {
		return int(maxItemCountByWriteCount)
	} else {
		return int(maxItemCountByWriteSize)
	}
}

type ExecutionDataRecord struct {
	BlockID     flow.Identifier
	BlockHeight uint64
	RootID      flow.Identifier // ID of the Execution Data root
	Completed   bool            // whether or not the Execution Data is fully downloaded
}
type Storage struct {
	db *badger.DB
}

type CheckOptions struct {
	Extended bool // extended mode: check all blobstore data
}

func OpenStorage(db *badger.DB, startHeight uint64) (*Storage, error) {
	storage := &Storage{db: db}
	err := storage.init(startHeight)

	return storage, err
}

func (s *Storage) init(startHeight uint64) error {
	storedDataLowerBound, err1 := s.GetStoredDataLowerBound()
	latestIncorporatedHeight, err2 := s.GetLatestIncorporatedHeight()

	if err1 == nil && err2 == nil {
		// db is already bootstrapped, we should complete any operations that were interrupted
		// during the previous shutdown
		if err := s.replay(storedDataLowerBound, latestIncorporatedHeight); err != nil {
			return fmt.Errorf("failed to restore db: %w", err)
		}
	} else if errors.Is(err1, badger.ErrKeyNotFound) && errors.Is(err2, badger.ErrKeyNotFound) {
		// db is empty, we need to bootstrap it
		if err := s.bootstrap(startHeight); err != nil {
			return fmt.Errorf("failed to bootstrap db: %w", err)
		}
	} else if (errors.Is(err1, badger.ErrKeyNotFound) && err2 == nil) || (err1 == nil && errors.Is(err2, badger.ErrKeyNotFound)) {
		// db is partially bootstrapped
		return fmt.Errorf("db is in inconsistent state")
	} else {
		return fmt.Errorf("failed to initialize storage: %w", err1)
	}

	return nil
}

func (s *Storage) replay(storedDataLowerBound uint64, latestIncorporatedHeight uint64) error {
	if _, err := s.Prune(storedDataLowerBound); err != nil {
		return err
	}

	if _, err := s.tryIncorporate(latestIncorporatedHeight + 1); err != nil {
		return err
	}

	return nil
}

func (s *Storage) bootstrap(startHeight uint64) error {
	latestIncorporatedHeightKey := makeGlobalStateKey(globalStateLatestIncorporatedHeight)
	latestIncorporatedHeightValue := make([]byte, 8)
	binary.LittleEndian.PutUint64(latestIncorporatedHeightValue, startHeight)

	storedDataLowerBoundKey := makeGlobalStateKey(globalStateStoredDataLowerBound)
	storedDataLowerBoundValue := make([]byte, 8)
	binary.LittleEndian.PutUint64(storedDataLowerBoundValue, startHeight)

	return s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(latestIncorporatedHeightKey, latestIncorporatedHeightValue); err != nil {
			return fmt.Errorf("failed to set latest incorporated height: %w", err)
		}

		if err := txn.Set(storedDataLowerBoundKey, storedDataLowerBoundValue); err != nil {
			return fmt.Errorf("failed to set stored data lower bound: %w", err)
		}

		return nil
	})
}

func (s *Storage) checkBlobTreeRecords() (map[cid.Cid]uint64, uint64, int, *multierror.Error, error) {
	blobTreeRecordPrefix := []byte{prefixBlobTreeRecord}
	latestHeightSeen := make(map[cid.Cid]uint64) // block height of the latest blob tree containing each Cid
	var highestBlobTreeRecordHeight uint64
	var earliestBlobTreeRecordHeight uint64
	var numBlobTreeRecords int
	var errors *multierror.Error

	if err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{
			PrefetchValues: false,
			Prefix:         blobTreeRecordPrefix,
		})
		defer it.Close()

		var previousHeight uint64
		var i int

		for it.Seek(blobTreeRecordPrefix); it.ValidForPrefix(blobTreeRecordPrefix); it.Next() {
			height, c, err := parseBlobTreeRecordKey(it.Item().Key())
			if err != nil {
				multierror.Append(errors, fmt.Errorf("malformed blob tree record key %v: %w", it.Item().Key(), err))
				continue
			}

			if i == 0 {
				earliestBlobTreeRecordHeight = height
			}

			latestHeightSeen[c] = height

			if i > 0 && height-previousHeight > 1 {
				// there should not be any gaps in blob tree record heights
				multierror.Append(errors, fmt.Errorf("missing blob tree records between heights %d and %d", previousHeight, height))
			}

			previousHeight = height
			i++
		}

		highestBlobTreeRecordHeight = previousHeight
		numBlobTreeRecords = i

		return nil
	}); err != nil {
		return nil, 0, 0, nil, err
	}

	if numBlobTreeRecords > 0 {
		storedDataLowerBound, err := s.GetStoredDataLowerBound()
		if err != nil {
			return nil, 0, 0, nil, fmt.Errorf("failed to get stored data lower bound: %w", err)
		}

		if earliestBlobTreeRecordHeight <= storedDataLowerBound {
			// there should not be any data stored for heights lower than the lower bound
			multierror.Append(errors, fmt.Errorf("stored data lower bound %d is not less than earliest blob tree record height %d", storedDataLowerBound, earliestBlobTreeRecordHeight))
		}
	}

	return latestHeightSeen, highestBlobTreeRecordHeight, numBlobTreeRecords, errors, nil
}

func (s *Storage) checkBlobRecords(latestHeightSeen map[cid.Cid]uint64) (*multierror.Error, error) {
	blobRecordPrefix := []byte{prefixBlobRecord}
	var errors *multierror.Error

	if err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{
			PrefetchValues: true,
			PrefetchSize:   100,
			Prefix:         blobRecordPrefix,
		})
		defer it.Close()

		for it.Seek(blobRecordPrefix); it.ValidForPrefix(blobRecordPrefix); it.Next() {
			item := it.Item()
			c, err := parseBlobRecordKey(item.Key())
			if err != nil {
				multierror.Append(errors, fmt.Errorf("malformed blob record key %v: %w", item.Key(), err))
				continue
			}

			expectedHeight, ok := latestHeightSeen[c]
			if !ok {
				multierror.Append(errors, fmt.Errorf("found blob record with no corresponding blob tree record: %s", c.String()))
				continue
			}

			delete(latestHeightSeen, c)

			blobRecordValue, err := item.ValueCopy(nil)
			if err != nil {
				return fmt.Errorf("failed to retrieve blob record value for Cid %s: %w", c.String(), err)
			}

			actualHeight := binary.LittleEndian.Uint64(blobRecordValue)
			if actualHeight != expectedHeight {
				multierror.Append(errors, fmt.Errorf("blob record height for Cid %s is %d, but should be %d", c.String(), actualHeight, expectedHeight))
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	for c := range latestHeightSeen {
		multierror.Append(errors, fmt.Errorf("missing blob record for Cid %s", c.String()))
		delete(latestHeightSeen, c)
	}

	return errors, nil
}

func (s *Storage) checkExecutionDataStatuses(numBlobTreeRecords int, highestBlobTreeRecordHeight uint64) (*multierror.Error, error) {
	executionDataStatusPrefix := []byte{prefixExecutionDataStatus}
	var firstExecutionDataStatus byte
	var minExecutionDataStatusHeight uint64
	var maxExecutionDataStatusHeight uint64
	var numExecutionDataStatuses int
	var errors *multierror.Error

	if err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{
			PrefetchValues: false,
			Prefix:         executionDataStatusPrefix,
		})
		defer it.Close()

		var i int
		for it.Seek(executionDataStatusPrefix); it.ValidForPrefix(executionDataStatusPrefix); it.Next() {
			item := it.Item()
			height := parseExecutionDataStatusKey(item.Key())

			status := item.UserMeta()
			if status != executionDataStatusInProgress && status != executionDataStatusComplete {
				multierror.Append(errors, fmt.Errorf("invalid execution data status for height %d: %v", height, status))
			}

			if i == 0 {
				firstExecutionDataStatus = status
				minExecutionDataStatusHeight = height
			}

			maxExecutionDataStatusHeight = height
			i++
		}

		numExecutionDataStatuses = i

		return nil
	}); err != nil {
		return nil, err
	}

	latestIncorporatedHeight, err := s.GetLatestIncorporatedHeight()
	if err != nil {
		return nil, fmt.Errorf("failed to get latest incorporated height: %w", err)
	}

	if numExecutionDataStatuses > 0 {
		if numBlobTreeRecords == 0 {
			// there should be at least one blob tree record per tracked execution data
			multierror.Append(errors, fmt.Errorf("found %d execution data statuses but no blob tree records", numExecutionDataStatuses))
		} else if maxExecutionDataStatusHeight != highestBlobTreeRecordHeight {
			multierror.Append(errors, fmt.Errorf(
				"max execution data status height %d did not match highest blob tree record height %d",
				maxExecutionDataStatusHeight,
				highestBlobTreeRecordHeight,
			))
		}

		if minExecutionDataStatusHeight <= latestIncorporatedHeight {
			// once an execution data is incorporated, the execution data status entry should be removed
			multierror.Append(errors, fmt.Errorf(
				"min execution data status height %d is less than or equal to latest incorporated height %d",
				minExecutionDataStatusHeight,
				latestIncorporatedHeight,
			))
		} else if minExecutionDataStatusHeight == latestIncorporatedHeight+1 && firstExecutionDataStatus == executionDataStatusComplete {
			// the latest incorporated height value is lower than it should be
			multierror.Append(errors, fmt.Errorf("latest incorporated height is %d, but execution data status at height %d is complete", latestIncorporatedHeight, minExecutionDataStatusHeight))
		}
	} else if numBlobTreeRecords > 0 && latestIncorporatedHeight != highestBlobTreeRecordHeight {
		multierror.Append(errors, fmt.Errorf(
			"latest incorporated height %d did not match highest blob tree record height %d",
			latestIncorporatedHeight,
			highestBlobTreeRecordHeight,
		))
	}

	return errors, nil
}

func (s *Storage) checkBlobs(blobCids map[cid.Cid]uint64) (*multierror.Error, error) {
	blobPrefix := network.BlobServiceDatastoreKeyPrefix
	var errors *multierror.Error

	if err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{
			PrefetchValues: false,
			Prefix:         blobPrefix,
		})
		defer it.Close()

		for it.Seek(blobPrefix); it.ValidForPrefix(blobPrefix); it.Next() {
			item := it.Item()

			c, err := network.CidFromBlobServiceDatastoreKey(datastore.NewKey(string(item.Key())))
			if err != nil {
				return fmt.Errorf("malformed blob service datastore key %v: %w", item.Key(), err)
			}

			_, ok := blobCids[c]
			if !ok {
				multierror.Append(errors, fmt.Errorf("no corresponding blob tree record for blob %s", c.String()))
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return errors, nil
}

// Check checks the integrity of the storage and returns any inconsistencies that were found.
func (s *Storage) Check(options CheckOptions) error {
	latestHeightSeen, highestBlobTreeRecordHeight, numBlobTreeRecords, errors1, err := s.checkBlobTreeRecords()
	if err != nil {
		return fmt.Errorf("failed checking blob tree records: %w", err)
	}

	var errors2 *multierror.Error
	if options.Extended {
		errors2, err = s.checkBlobs(latestHeightSeen)
		if err != nil {
			return fmt.Errorf("failed checking blobstore: %w", err)
		}

		// TODO: check validity of each blob tree:
		// - the set of cids tracked by the blob tree records for a given height should match
		//   the set of cids that result from actually constructing the blob tree
		// - this probably requires us to mark the blob tree record corresponding to the root CID
		//   for each height using user metadata
	}

	errors3, err := s.checkBlobRecords(latestHeightSeen)
	if err != nil {
		return fmt.Errorf("failed checking blob records: %w", err)
	}

	errors4, err := s.checkExecutionDataStatuses(numBlobTreeRecords, highestBlobTreeRecordHeight)
	if err != nil {
		return fmt.Errorf("failed checking execution data statuses: %w", err)
	}

	return multierror.Append(errors1, errors2, errors3, errors4).ErrorOrNil()
}

// LoadState loads the state from the storage. It returns:
// - the list of Execution Data records that are tracked by the storage
// - the latest incorporated height
func (s *Storage) LoadState() ([]*ExecutionDataRecord, uint64, error) {
	latestIncorporatedHeightKey := makeGlobalStateKey(globalStateLatestIncorporatedHeight)
	executionDataStatusPrefix := []byte{prefixExecutionDataStatus}
	var trackedItems []*ExecutionDataRecord
	var latestIncorporatedHeight uint64

	if err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(latestIncorporatedHeightKey)
		if err != nil {
			return fmt.Errorf("failed to get latest incorporated height: %w", err)
		}

		latestIncorporatedHeightValue, err := item.ValueCopy(nil)
		if err != nil {
			return fmt.Errorf("failed to retrieve latest incorporated height value: %w", err)
		}

		latestIncorporatedHeight = binary.LittleEndian.Uint64(latestIncorporatedHeightValue)

		it := txn.NewIterator(badger.IteratorOptions{
			PrefetchValues: true,
			PrefetchSize:   10,
			Prefix:         executionDataStatusPrefix,
		})
		defer it.Close()

		for it.Seek(executionDataStatusPrefix); it.ValidForPrefix(executionDataStatusPrefix); it.Next() {
			item := it.Item()
			height := parseExecutionDataStatusKey(item.Key())
			executionDataStatusValue, err := item.ValueCopy(nil)
			if err != nil {
				return fmt.Errorf("failed to retrieve execution data status value at height %d: %w", height, err)
			}

			blockID, rootID := parseExecutionDataStatusValue(executionDataStatusValue)
			trackedItems = append(trackedItems, &ExecutionDataRecord{
				BlockID:     blockID,
				BlockHeight: height,
				RootID:      rootID,
				Completed:   item.UserMeta() == executionDataStatusComplete,
			})

		}

		return nil
	}); err != nil {
		return nil, 0, err
	}

	return trackedItems, latestIncorporatedHeight, nil
}

// batchDelete accepts a list of blob CIDs and removes all deletable data associated with them.
// It fills `deletedCIDs` with the CIDs that were completely removed, and returns the removed count.
func (s *Storage) batchDeleteCIDs(cids []cid.Cid, blockHeights []uint64, deletedCIDs []cid.Cid) (int, error) {
	var numDeleted int

	blobTreeRecordKeys := make([][]byte, len(cids))
	for i, c := range cids {
		blobTreeRecordKeys[i] = makeBlobTreeRecordKey(blockHeights[i], c)
	}

	blobRecordKeys := make([][]byte, len(cids))
	for i, c := range cids {
		blobRecordKeys[i] = makeBlobRecordKey(c)
	}

	if err := retryOnConflict(s.db, func(txn *badger.Txn) error {
		n := 0

		for i, c := range cids {
			blockHeight := blockHeights[i]
			blobTreeRecordKey := blobTreeRecordKeys[i]
			blobRecordKey := blobRecordKeys[i]

			// remove blob tree record
			if err := txn.Delete(blobTreeRecordKey); err != nil {
				return fmt.Errorf("failed to delete blob tree record for Cid %s at height %d: %w", c.String(), blockHeight, err)
			}

			item, err := txn.Get(blobRecordKey)
			if err != nil {
				return fmt.Errorf("failed to get blob record for Cid %s: %w", c.String(), err)
			}

			blobRecordValue, err := item.ValueCopy(nil)
			if err != nil {
				return fmt.Errorf("failed to retrieve blob record value for Cid %s: %w", c.String(), err)
			}

			// only remove blob record and blob data if it is not referenced by any other blob tree
			latestHeightContainingBlob := binary.LittleEndian.Uint64(blobRecordValue)
			if latestHeightContainingBlob < blockHeight {
				// this should never happen
				return fmt.Errorf(
					"inconsistency detected: blob record height for Cid %s is %d, but blob tree record exists at height %d",
					c.String(), latestHeightContainingBlob, blockHeight,
				)
			} else if latestHeightContainingBlob == blockHeight {
				if err := txn.Delete(blobRecordKey); err != nil {
					return fmt.Errorf("failed to delete blob record for Cid %s: %w", c.String(), err)
				}

				blobKey := network.BlobServiceDatastoreKeyFromCid(c).Bytes()
				if err := txn.Delete(blobKey); err != nil {
					return fmt.Errorf("failed to delete blob data %s: %w", c.String(), err)
				}

				deletedCIDs[n] = c
				n += 1
			}
		}

		numDeleted = n

		return nil
	}); err != nil {
		return 0, err
	}

	return numDeleted, nil
}

// getCidsToDelete returns the list of CIDs and corresponding block heights that are candidates
// for deletion when pruning up to the given height.
func (s *Storage) getCidsToDelete(height uint64) ([]cid.Cid, []uint64, error) {
	var cidsToDelete []cid.Cid
	var blockHeights []uint64

	blobTreeRecordPrefix := []byte{prefixBlobTreeRecord}

	if err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{
			PrefetchValues: false,
			Prefix:         blobTreeRecordPrefix,
		})
		defer it.Close()

		for it.Seek(blobTreeRecordPrefix); it.ValidForPrefix(blobTreeRecordPrefix); it.Next() {
			item := it.Item()
			blobTreeRecordKey := item.Key()

			blockHeight, blobCid, err := parseBlobTreeRecordKey(blobTreeRecordKey)
			if err != nil {
				return fmt.Errorf("malformed blob tree record key %v: %w", blobTreeRecordKey, err)
			}

			if blockHeight > height {
				break
			}

			cidsToDelete = append(cidsToDelete, blobCid)
			blockHeights = append(blockHeights, blockHeight)
		}

		return nil
	}); err != nil {
		return nil, nil, err
	}

	return cidsToDelete, blockHeights, nil
}

// Prune removes all data from storage up to the given height, including all blob tree records, blob records, and blob data,
// and returns the list of deleted CIDs. It should only be called with height <= latest incorporated height.
// This method should not be called concurrently from multiple goroutines, but is safe to call concurrently with other methods.
func (s *Storage) Prune(height uint64) ([]cid.Cid, error) {
	storedDataLowerBoundKey := makeGlobalStateKey(globalStateStoredDataLowerBound)
	storedDataLowerBoundValue := make([]byte, 8)
	binary.LittleEndian.PutUint64(storedDataLowerBoundValue, height)

	// update stored data lower bound first, so that we can resume later if the operation is interrupted by a crash / restart
	if err := s.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(storedDataLowerBoundKey, storedDataLowerBoundValue); err != nil {
			return fmt.Errorf("failed to set stored data lower bound: %w", err)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	// get candidate CIDs for deletion
	cidsToDelete, blockHeights, err := s.getCidsToDelete(height)
	if err != nil {
		return nil, fmt.Errorf("failed to gather list of CIDs for deletion: %w", err)
	}

	deletedCIDs := make([]cid.Cid, len(cidsToDelete)) // stores actually deleted CIDs
	numDeleted := 0

	cidsPerBatch := 16
	maxCidsPerBatch := getBatchItemCountLimit(
		s.db,
		0,
		0,
		3, // delete blob tree record, blob record, and blob data
		blobTreeRecordKeyLength+blobRecordKeyLength+int64(network.BlobServiceDatastoreKeyLength),
	)
	if maxCidsPerBatch < cidsPerBatch {
		cidsPerBatch = maxCidsPerBatch
	}

	for len(cidsToDelete) > 0 {
		batchSize := cidsPerBatch
		if len(cidsToDelete) < batchSize {
			batchSize = len(cidsToDelete)
		}

		n, err := s.batchDeleteCIDs(cidsToDelete[:batchSize], blockHeights[:batchSize], deletedCIDs[numDeleted:])
		if err != nil {
			return nil, fmt.Errorf("failed to delete CIDs: %w", err)
		}

		numDeleted += n
		cidsToDelete = cidsToDelete[batchSize:]
		blockHeights = blockHeights[batchSize:]
	}

	return deletedCIDs[:numDeleted], nil
}

// tryIncorporate tries to incorporate as many heights as possible starting from the given height.
func (s *Storage) tryIncorporate(height uint64) (uint64, error) {
	var latestIncorporatedHeight uint64
	latestIncorporatedHeightKey := makeGlobalStateKey(globalStateLatestIncorporatedHeight)

	batchSize := 16
	maxBatchSize := getBatchItemCountLimit(s.db, 1, globalStateKeyLength+8, 1, executionDataStatusKeyLength)
	if maxBatchSize < batchSize {
		batchSize = maxBatchSize
	}

	var finished bool
	for startHeight := height; !finished; startHeight += uint64(batchSize) {
		// incorporate up to batchSize heights starting from startHeight
		if err := retryOnConflict(s.db, func(txn *badger.Txn) error {
			var nextHeight uint64
			var exitedEarly bool

			for nextHeight = startHeight; nextHeight < startHeight+uint64(batchSize); nextHeight++ {
				executionDataStatusKey := makeExecutionDataStatusKey(nextHeight)

				item, err := txn.Get(executionDataStatusKey)
				if err != nil {
					if errors.Is(err, badger.ErrKeyNotFound) {
						// execution data for this height is either not yet tracked,
						// or has already been incorporated
						exitedEarly = true
						break
					}

					return fmt.Errorf("failed to get execution data status at height %d: %w", nextHeight, err)
				}

				status := item.UserMeta()
				if status == executionDataStatusInProgress {
					exitedEarly = true
					break
				} else if status != executionDataStatusComplete {
					return fmt.Errorf("invalid execution data status at height %d: %v", nextHeight, status)
				}

				if err := txn.Delete(executionDataStatusKey); err != nil {
					return fmt.Errorf("failed to delete execution data status: %w", err)
				}
			}

			if nextHeight > startHeight {
				// update latest incorporated height
				latestIncorporatedHeightValue := make([]byte, 8)
				binary.LittleEndian.PutUint64(latestIncorporatedHeightValue, nextHeight-1)
				if err := txn.Set(latestIncorporatedHeightKey, latestIncorporatedHeightValue); err != nil {
					return fmt.Errorf("failed to set latest incorporated height: %w", err)
				}
			}

			latestIncorporatedHeight = nextHeight - 1
			finished = exitedEarly

			return nil
		}); err != nil {
			return 0, err
		}
	}

	return latestIncorporatedHeight, nil
}

// GetLatestIncorporatedHeight returns the latest incorporated height.
func (s *Storage) GetLatestIncorporatedHeight() (uint64, error) {
	latestIncorporatedHeightKey := makeGlobalStateKey(globalStateLatestIncorporatedHeight)
	var latestIncorporatedHeight uint64

	if err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(latestIncorporatedHeightKey)
		if err != nil {
			return fmt.Errorf("failed to find latest incorporated height entry: %w", err)
		}

		latestIncorporatedHeightValue, err := item.ValueCopy(nil)
		if err != nil {
			return fmt.Errorf("failed to retrieve latest incorporated height value: %w", err)
		}

		latestIncorporatedHeight = binary.LittleEndian.Uint64(latestIncorporatedHeightValue)

		return nil
	}); err != nil {
		return 0, err
	}

	return latestIncorporatedHeight, nil
}

// GetStoredDataLowerBound returns the maximum h such that the storage contains no data for any block heights <= h
// TODO: please help me think of a better name for this function
func (s *Storage) GetStoredDataLowerBound() (uint64, error) {
	storedDataLowerBoundKey := makeGlobalStateKey(globalStateStoredDataLowerBound)
	var storedDataLowerBound uint64

	if err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(storedDataLowerBoundKey)
		if err != nil {
			return fmt.Errorf("failed to find stored data lower bound entry: %w", err)
		}

		storedDataLowerBoundValue, err := item.ValueCopy(nil)
		if err != nil {
			return fmt.Errorf("failed to retrieve stored data lower bound value: %w", err)
		}

		storedDataLowerBound = binary.LittleEndian.Uint64(storedDataLowerBoundValue)

		return nil
	}); err != nil {
		return 0, err
	}

	return storedDataLowerBound, nil
}

// GetStatusTracker returns a status tracker which can be used to track the Execution Data download progress for the given block.
func (s *Storage) GetStatusTracker(blockID flow.Identifier, blockHeight uint64, executionDataID flow.Identifier) StatusTracker {
	return &executionDataStatusTracker{
		storage:     s,
		blockID:     blockID,
		blockHeight: blockHeight,
		rootID:      executionDataID,
	}
}

var (
	ErrStatusTrackerAlreadyStarted  = errors.New("status tracker already started")
	ErrStatusTrackerNotStarted      = errors.New("status tracker not started")
	ErrStatusTrackerAlreadyFinished = errors.New("status tracker already finished")
)

type executionDataStatusTracker struct {
	storage     *Storage
	blockID     flow.Identifier
	blockHeight uint64
	rootID      flow.Identifier
	started     bool
	finished    bool
}

func (s *executionDataStatusTracker) trackBlob(txn *badger.Txn, c cid.Cid) error {
	if err := txn.Set(makeBlobTreeRecordKey(s.blockHeight, c), nil); err != nil {
		return fmt.Errorf("failed to add blob tree record: %w", err)
	}

	blobRecordKey := makeBlobRecordKey(c)
	item, err := txn.Get(blobRecordKey)
	if err != nil {
		if !errors.Is(err, badger.ErrKeyNotFound) {
			return fmt.Errorf("failed to get blob record: %w", err)
		}
	} else {
		value, err := item.ValueCopy(nil)
		if err != nil {
			return fmt.Errorf("failed to retrieve blob record value: %w", err)
		}

		// don't update the blob record if there is already a higher block height containing this blob
		latestHeightContainingBlob := binary.LittleEndian.Uint64(value)
		if latestHeightContainingBlob >= s.blockHeight {
			return nil
		}
	}

	blobRecordValue := make([]byte, 8)
	binary.LittleEndian.PutUint64(blobRecordValue, s.blockHeight)

	if err := txn.Set(blobRecordKey, blobRecordValue); err != nil {
		return fmt.Errorf("failed to set blob record value: %w", err)
	}

	return nil
}

// StartTransfer tracks the start of the Execution Data transfer.
func (s *executionDataStatusTracker) StartTransfer() error {
	if s.started {
		return ErrStatusTrackerAlreadyStarted
	}

	executionDataStatusKey := makeExecutionDataStatusKey(s.blockHeight)
	executionDataStatusValue := makeExecutionDataStatusValue(s.blockID, s.rootID)

	if err := retryOnConflict(s.storage.db, func(txn *badger.Txn) error {
		if err := txn.SetEntry(
			badger.NewEntry(
				executionDataStatusKey,
				executionDataStatusValue,
			).WithMeta(executionDataStatusInProgress),
		); err != nil {
			return fmt.Errorf("failed to set execution data status: %w", err)
		}

		if err := s.trackBlob(txn, flow.IdToCid(s.rootID)); err != nil {
			return fmt.Errorf("failed to track root blob: %w", err)
		}

		return nil
	}); err != nil {
		return err
	}

	s.started = true
	return nil
}

// TrackBlobs tracks the given cids as part of the Execution Data.
func (s *executionDataStatusTracker) TrackBlobs(cids []cid.Cid) error {
	if !s.started {
		return ErrStatusTrackerNotStarted
	} else if s.finished {
		return ErrStatusTrackerAlreadyFinished
	}

	cidsPerBatch := 32
	maxCidsPerBatch := getBatchItemCountLimit(s.storage.db, 0, 0, 2, blobTreeRecordKeyLength+blobRecordKeyLength+8)
	if maxCidsPerBatch < cidsPerBatch {
		cidsPerBatch = maxCidsPerBatch
	}

	for len(cids) > 0 {
		batchSize := cidsPerBatch
		if len(cids) < batchSize {
			batchSize = len(cids)
		}
		batch := cids[:batchSize]

		if err := retryOnConflict(s.storage.db, func(txn *badger.Txn) error {
			for _, c := range batch {
				if err := s.trackBlob(txn, c); err != nil {
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

// FinishTransfer marks the transfer of the Execution Data as complete, triggers incorporation
// if possible starting from the current height, and returns the resulting latest incorporated height.
func (s *executionDataStatusTracker) FinishTransfer() (uint64, error) {
	if !s.started {
		return 0, ErrStatusTrackerNotStarted
	} else if s.finished {
		return 0, ErrStatusTrackerAlreadyFinished
	}

	executionDataStatusKey := makeExecutionDataStatusKey(s.blockHeight)
	executionDataStatusValue := makeExecutionDataStatusValue(s.blockID, s.rootID)
	latestIncorporatedHeightKey := makeGlobalStateKey(globalStateLatestIncorporatedHeight)
	var latestIncorporatedHeight uint64

	if err := retryOnConflict(s.storage.db, func(txn *badger.Txn) error {
		item, err := txn.Get(latestIncorporatedHeightKey)
		if err != nil {
			return fmt.Errorf("failed to get latest incorporated height: %w", err)
		}

		latestIncorporatedHeightValue, err := item.ValueCopy(nil)
		if err != nil {
			return fmt.Errorf("failed to retrieve latest incorporated height value: %w", err)
		}

		latestIncorporatedHeight = binary.LittleEndian.Uint64(latestIncorporatedHeightValue)

		if err := txn.SetEntry(
			badger.NewEntry(
				executionDataStatusKey,
				executionDataStatusValue,
			).WithMeta(executionDataStatusComplete),
		); err != nil {
			return fmt.Errorf("failed to set execution data status: %w", err)
		}

		return nil
	}); err != nil {
		return 0, err
	}

	s.finished = true

	// only trigger incorporation if the previous height is incorporated
	if latestIncorporatedHeight == s.blockHeight-1 {
		return s.storage.tryIncorporate(s.blockHeight)
	}

	return latestIncorporatedHeight, nil
}
