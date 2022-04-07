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

var _ StatusTrackerFactory = (*storage)(nil)

const (
	prefixGlobalState byte = iota + 1
	prefixExecutionDataStatus
	prefixBlobRecord
	prefixBlobTreeRecord
)

const (
	executionDataStatusInProgress byte = iota + 1
	executionDataStatusComplete
)

const (
	globalStateLatestIncorporatedHeight = iota + 1
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

func makeGlobalStateKey(state byte) []byte {
	globalStateKey := make([]byte, 2)
	globalStateKey[0] = prefixGlobalState
	globalStateKey[1] = state
	return globalStateKey
}

func makeExecutionDataStatusKey(blockHeight uint64) []byte {
	executionDataStatusKey := make([]byte, 1+8)
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

// TODO: cleanup all error messages

type ExecutionDataRecord struct {
	BlockID     flow.Identifier
	BlockHeight uint64
	RootID      flow.Identifier
	Completed   bool
}
type storage struct {
	db *badger.DB
}

type CheckOptions struct {
	Extended         bool
	RemoveExtraneous bool
}

func (s *storage) checkBlobTreeRecords() (map[cid.Cid]uint64, uint64, *multierror.Error, error) {
	blobTreeRecordPrefix := []byte{prefixBlobTreeRecord}
	latestHeightSeen := make(map[cid.Cid]uint64)
	var latestBlobTreeRecordHeight uint64
	var errors *multierror.Error

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{
			PrefetchValues: false,
			Prefix:         blobTreeRecordPrefix,
		})
		defer it.Close()

		var previousHeight uint64

		for it.Seek(blobTreeRecordPrefix); it.ValidForPrefix(blobTreeRecordPrefix); it.Next() {
			height, c, err := parseBlobTreeRecordKey(it.Item().Key())
			if err != nil {
				multierror.Append(errors, fmt.Errorf("invalid blob tree record key %v: %w", it.Item().Key(), err))
				continue
			}

			latestHeightSeen[c] = height

			if previousHeight != 0 && height-previousHeight > 1 {
				multierror.Append(errors, fmt.Errorf("missing blob tree records from height %d to %d", previousHeight, height))
			}

			previousHeight = height
		}

		latestBlobTreeRecordHeight = previousHeight

		return nil
	})

	if err != nil {
		return nil, 0, nil, err
	}

	return latestHeightSeen, latestBlobTreeRecordHeight, errors, nil
}

func (s *storage) checkBlobRecords(latestHeightSeen map[cid.Cid]uint64) (*multierror.Error, error) {
	blobRecordPrefix := []byte{prefixBlobRecord}
	var errors *multierror.Error

	err := s.db.View(func(txn *badger.Txn) error {
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
				multierror.Append(errors, fmt.Errorf("invalid blob record key %v: %w", item.Key(), err))
				continue
			}

			expectedHeight, ok := latestHeightSeen[c]
			if !ok {
				multierror.Append(errors, fmt.Errorf("no blob tree record matching blob record %s", c.String()))
				continue
			}

			delete(latestHeightSeen, c)

			blobRecordValue, err := item.ValueCopy(nil)
			if err != nil {
				return fmt.Errorf("failed to get blob record value for Cid %s: %w", c.String(), err)
			}

			actualHeight := binary.LittleEndian.Uint64(blobRecordValue)
			if actualHeight != expectedHeight {
				multierror.Append(errors, fmt.Errorf("blob record height for Cid %s is %d, but should be %d", c.String(), actualHeight, expectedHeight))
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	for c := range latestHeightSeen {
		multierror.Append(errors, fmt.Errorf("blob record for Cid %s is missing", c.String()))
		delete(latestHeightSeen, c)
	}

	return errors, nil
}

func (s *storage) checkExecutionDataStatuses(latestBlobTreeRecordHeight uint64) (*multierror.Error, error) {
	executionDataStatusPrefix := []byte{prefixExecutionDataStatus}
	latestIncorporatedHeightKey := makeGlobalStateKey(globalStateLatestIncorporatedHeight)
	var minExecutionDataStatusHeight uint64
	var maxExecutionDataStatusHeight uint64
	var latestIncorporatedHeight uint64
	var iterated bool
	var errors *multierror.Error

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{
			PrefetchValues: false,
			Prefix:         executionDataStatusPrefix,
		})
		defer it.Close()

		for it.Seek(executionDataStatusPrefix); it.ValidForPrefix(executionDataStatusPrefix); it.Next() {
			item := it.Item()
			height := parseExecutionDataStatusKey(item.Key())

			if item.UserMeta() != executionDataStatusInProgress && item.UserMeta() != executionDataStatusComplete {
				multierror.Append(errors, fmt.Errorf("invalid execution data status for height %d: %v", height, item.UserMeta()))
			}

			if !iterated {
				iterated = true
				minExecutionDataStatusHeight = height
			}

			maxExecutionDataStatusHeight = height
		}

		txn.Get(latestIncorporatedHeightKey)

		return nil
	})

	if err != nil {
		return nil, err
	}

	if iterated {
		if maxExecutionDataStatusHeight != latestBlobTreeRecordHeight {
			multierror.Append(errors, fmt.Errorf(
				"highest execution data status height %d did not match highest blob tree record height is %d",
				maxExecutionDataStatusHeight,
				latestBlobTreeRecordHeight,
			))
		}

		if minExecutionDataStatusHeight <= latestIncorporatedHeight {
			multierror.Append(errors, fmt.Errorf(
				"lowest execution data status height %d is less than or equal to highest incorporated height %d",
				minExecutionDataStatusHeight,
				latestIncorporatedHeight,
			))
		}
	} else if latestIncorporatedHeight != latestBlobTreeRecordHeight {
		multierror.Append(errors, fmt.Errorf(
			"highest incorporated height %d did not match highest blob tree record height %d",
			latestIncorporatedHeight,
			latestBlobTreeRecordHeight,
		))
	}

	return errors, nil
}

func (s *storage) checkBlobs(blobCids map[cid.Cid]uint64) (*multierror.Error, error) {
	blobPrefix := []byte{network.BlobServiceDatastoreKeyPrefix}
	var errors *multierror.Error

	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.IteratorOptions{
			PrefetchValues: false,
			Prefix:         blobPrefix,
		})
		defer it.Close()

		for it.Seek(blobPrefix); it.ValidForPrefix(blobPrefix); it.Next() {
			item := it.Item()

			c, err := network.CidFromBlobServiceDatastoreKey(datastore.NewKey(string(item.Key())))
			if err != nil {
				return fmt.Errorf("could not parse Cid from blob service datastore key %v: %w", item.Key(), err)
			}

			_, ok := blobCids[c]
			if !ok {
				multierror.Append(errors, fmt.Errorf("missing blob tree record for blob %s", c.String()))
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return errors, nil
}

func (s *storage) Check(options CheckOptions) error {
	latestHeightSeen, latestBlobTreeRecordHeight, errors1, err := s.checkBlobTreeRecords()
	if err != nil {
		return err
	}

	var errors2 *multierror.Error
	if options.Extended {
		errors2, err = s.checkBlobs(latestHeightSeen)
		if err != nil {
			return err
		}

		// TODO: check validity of each blob tree:
		// - the set of cids tracked by the blob tree records for a given height should match
		//   the set of cids that result from actually constructing the blob tree
		// - this probably requires us to mark the blob tree record corresponding to the root CID
		//   for each height using user metadata
	}

	errors3, err := s.checkBlobRecords(latestHeightSeen)
	if err != nil {
		return err
	}

	errors4, err := s.checkExecutionDataStatuses(latestBlobTreeRecordHeight)
	if err != nil {
		return err
	}

	if options.RemoveExtraneous {
		// TODO: purge all extraneous keys / blobs
	}

	return multierror.Append(errors1, errors2, errors3, errors4).ErrorOrNil()
}

func (s *storage) LoadState() (
	trackedItems []*ExecutionDataRecord,
	pendingHeights []uint64,
	err error,
) {
	latestIncorporatedHeightKey := makeGlobalStateKey(globalStateLatestIncorporatedHeight)
	executionDataStatusPrefix := []byte{prefixExecutionDataStatus}

	err = s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(latestIncorporatedHeightKey)
		if err != nil {
			return fmt.Errorf("failed to get latest incorporated height: %w", err)
		}

		latestIncorporatedHeightValue, err := item.ValueCopy(nil)
		if err != nil {
			return fmt.Errorf("failed to retrieve latest incorporated height value: %w", err)
		}

		previousHeight := binary.LittleEndian.Uint64(latestIncorporatedHeightValue)

		it := txn.NewIterator(badger.IteratorOptions{
			PrefetchValues: true,
			PrefetchSize:   10,
			Prefix:         executionDataStatusPrefix,
		})
		defer it.Close()

		for it.Seek(executionDataStatusPrefix); it.ValidForPrefix(executionDataStatusPrefix); it.Next() {
			item := it.Item()
			executionDataStatusKey := item.Key()
			executionDataStatusValue, err := item.ValueCopy(nil)
			if err != nil {
				// TODO: format
				return err
			}

			height := parseExecutionDataStatusKey(executionDataStatusKey)
			for i := previousHeight + 1; i < height; i++ {
				pendingHeights = append(pendingHeights, i)
			}

			blockID, rootID := parseExecutionDataStatusValue(executionDataStatusValue)
			trackedItems = append(trackedItems, &ExecutionDataRecord{
				BlockID:     blockID,
				BlockHeight: height,
				RootID:      rootID,
				Completed:   item.UserMeta() == executionDataStatusComplete,
			})

			previousHeight = height
		}

		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	return trackedItems, pendingHeights, nil
}

func (s *storage) batchDeleteCIDs(cids []cid.Cid, blockHeights []uint64) error {
	return retryOnConflict(s.db, func(txn *badger.Txn) error {
		for i, c := range cids {
			blockHeight := blockHeights[i]

			err := txn.Delete(makeBlobTreeRecordKey(blockHeight, c))
			if err != nil {
				return fmt.Errorf("failed to delete blob tree record: %w", err)
			}

			blobRecordKey := makeBlobRecordKey(c)
			item, err := txn.Get(blobRecordKey)
			if err != nil {
				return fmt.Errorf("failed to get blob record: %w", err)
			}

			blobRecordValue, err := item.ValueCopy(nil)
			if err != nil {
				return fmt.Errorf("failed to get blob record value: %w", err)
			}

			err = txn.Delete(blobRecordKey)
			if err != nil {
				return fmt.Errorf("failed to delete blob record: %w", err)
			}

			latestHeightContainingBlob := binary.LittleEndian.Uint64(blobRecordValue)
			if latestHeightContainingBlob < blockHeight {
				// this should never happen
				return fmt.Errorf(
					"inconsistency detected: blob record height for %s is %d, but blob tree record exists at height %d",
					c.String(), latestHeightContainingBlob, blockHeight,
				)
			} else if latestHeightContainingBlob == blockHeight {
				blobKey := network.BlobServiceDatastoreKeyFromCid(c).Bytes()
				err = txn.Delete(blobKey)
				if err != nil {
					return fmt.Errorf("failed to delete blob: %w", err)
				}
			}
		}

		return nil
	})
}

func (s *storage) getCidsToDelete(height uint64) ([]cid.Cid, []uint64, error) {
	var cidsToDelete []cid.Cid
	var blockHeights []uint64

	blobTreeRecordPrefix := []byte{prefixBlobTreeRecord}

	err := s.db.View(func(txn *badger.Txn) error {
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
				return fmt.Errorf("failed to parse blob tree record: %w", err)
			}

			if blockHeight > height {
				break
			}

			cidsToDelete = append(cidsToDelete, blobCid)
			blockHeights = append(blockHeights, blockHeight)
		}

		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	return cidsToDelete, blockHeights, nil
}

func (s *storage) Bootstrap(startHeight uint64) error {
	latestIncorporatedHeightKey := makeGlobalStateKey(globalStateLatestIncorporatedHeight)
	latestIncorporatedHeightValue := make([]byte, 8)
	binary.LittleEndian.PutUint64(latestIncorporatedHeightValue, startHeight)

	return s.db.Update(func(txn *badger.Txn) error {
		err := txn.Set(latestIncorporatedHeightKey, latestIncorporatedHeightValue)
		if err != nil {
			return fmt.Errorf("failed to set latest incorporated height: %w", err)
		}

		return nil
	})
}

// Prune removes all data from storage corresponding to block heights lower than or equal to the given height.
// This includes all blob tree records, blob records, and blob data.
// This should only be called with height <= latest incorporated height.
func (s *storage) Prune(height uint64) error {
	cidsToDelete, blockHeights, err := s.getCidsToDelete(height)
	if err != nil {
		return fmt.Errorf("failed to gather list of CIDs for deletion: %w", err)
	}

	writeCountPerCid := 3
	writeSizePerCid := 3*2 + blobTreeRecordKeyLength + blobRecordKeyLength + network.BlobServiceDatastoreKeyLength
	maxCidsByWriteCount := int(s.db.MaxBatchCount() / int64(writeCountPerCid))
	maxCidsByWriteSize := int(s.db.MaxBatchSize() / int64(writeSizePerCid))

	cidsPerBatch := 16
	if maxCidsByWriteCount < cidsPerBatch {
		cidsPerBatch = maxCidsByWriteCount
	} else if maxCidsByWriteSize < cidsPerBatch {
		cidsPerBatch = maxCidsByWriteSize
	}

	for len(cidsToDelete) > 0 {
		batchSize := cidsPerBatch
		if len(cidsToDelete) < batchSize {
			batchSize = len(cidsToDelete)
		}

		err := s.batchDeleteCIDs(cidsToDelete[:batchSize], blockHeights[:batchSize])
		if err != nil {
			return fmt.Errorf("failed to delete CIDs: %w", err)
		}

		cidsToDelete = cidsToDelete[batchSize:]
		blockHeights = blockHeights[batchSize:]
	}

	return nil
}

func (s *storage) GetStatusTracker(blockID flow.Identifier, blockHeight uint64, executionDataID flow.Identifier) StatusTracker {
	return &executionDataStatusTracker{
		db:          s.db,
		blockID:     blockID,
		blockHeight: blockHeight,
		rootID:      executionDataID,
	}
}

type executionDataStatusTracker struct {
	db          *badger.DB
	blockID     flow.Identifier
	blockHeight uint64
	rootID      flow.Identifier
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
			return fmt.Errorf("failed to get blob record value: %w", err)
		}

		latestHeightContainingBlob := binary.LittleEndian.Uint64(value)
		if latestHeightContainingBlob >= s.blockHeight {
			return nil
		}
	}

	blobRecordValue := make([]byte, 8)
	binary.LittleEndian.PutUint64(blobRecordValue, s.blockHeight)

	if err := txn.Set(blobRecordKey, blobRecordValue); err != nil {
		return fmt.Errorf("failed to add blob record: %w", err)
	}

	return nil
}

func (s *executionDataStatusTracker) StartTransfer() error {
	executionDataStatusKey := makeExecutionDataStatusKey(s.blockHeight)
	executionDataStatusValue := makeExecutionDataStatusValue(s.blockID, s.rootID)

	return retryOnConflict(s.db, func(txn *badger.Txn) error {
		if err := txn.SetEntry(
			badger.NewEntry(
				executionDataStatusKey,
				executionDataStatusValue,
			).WithMeta(executionDataStatusInProgress),
		); err != nil {
			return fmt.Errorf("failed to set execution data status: %w", err)
		}

		if err := s.trackBlob(txn, flow.IdToCid(s.rootID)); err != nil {
			return fmt.Errorf("failed to set blob record: %w", err)
		}

		return nil
	})
}

func (s *executionDataStatusTracker) TrackBlobs(cids []cid.Cid) error {
	// TODO: handle ErrTxnTooBig

	return retryOnConflict(s.db, func(txn *badger.Txn) error {
		for _, c := range cids {
			if err := s.trackBlob(txn, c); err != nil {
				return fmt.Errorf("failed to set blob record: %w", err)
			}
		}

		return nil
	})
}

// TODO: handle ErrTxnTooBig
func (s *executionDataStatusTracker) FinishTransfer() (latestIncorporatedHeight uint64, err error) {
	executionDataStatusKey := makeExecutionDataStatusKey(s.blockHeight)
	executionDataStatusValue := makeExecutionDataStatusValue(s.blockID, s.rootID)
	latestIncorporatedHeightKey := makeGlobalStateKey(globalStateLatestIncorporatedHeight)

	err = retryOnConflict(s.db, func(txn *badger.Txn) error {
		item, err := txn.Get(latestIncorporatedHeightKey)
		if err != nil {
			return fmt.Errorf("failed to get latest incorporated height: %w", err)
		}

		latestIncorporatedHeightValue, err := item.ValueCopy(nil)
		if err != nil {
			return fmt.Errorf("failed to retrieve latest incorporated height value: %w", err)
		}

		latestIncorporatedHeight = binary.LittleEndian.Uint64(latestIncorporatedHeightValue)

		if latestIncorporatedHeight == s.blockHeight-1 {
			nextExecutionDataStatusKey := executionDataStatusKey

		incrementLatestIncorporatedHeight:
			latestIncorporatedHeight++

			if err := txn.Delete(nextExecutionDataStatusKey); err != nil {
				return fmt.Errorf("failed to delete execution status key: %w", err)
			}

			nextExecutionDataStatusKey = makeExecutionDataStatusKey(latestIncorporatedHeight + 1)

			item, err := txn.Get(nextExecutionDataStatusKey)
			if err != nil {
				if !errors.Is(err, badger.ErrKeyNotFound) {
					return fmt.Errorf("failed to get execution data status: %w", err)
				}
			} else if item.UserMeta() == executionDataStatusComplete {
				goto incrementLatestIncorporatedHeight
			}

			binary.LittleEndian.PutUint64(latestIncorporatedHeightValue, latestIncorporatedHeight)
			if err := txn.Set(latestIncorporatedHeightKey, latestIncorporatedHeightValue); err != nil {
				return fmt.Errorf("failed to set latest incorporated height: %w", err)
			}
		} else {
			if err := txn.SetEntry(
				badger.NewEntry(
					executionDataStatusKey,
					executionDataStatusValue,
				).WithMeta(executionDataStatusComplete),
			); err != nil {
				return fmt.Errorf("failed to set execution data status: %w", err)
			}
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	return latestIncorporatedHeight, nil
}
