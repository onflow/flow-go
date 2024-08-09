package operation

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/storage"
)

// InitTrackerHeights initializes the fulfilled and the pruned heights for the execution data tracker storage.
//
// No errors are expected during normal operations.
func InitTrackerHeights(height uint64) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {
		if err := insert(makePrefix(storage.PrefixGlobalState, storage.GlobalStateFulfilledHeight), height)(tx); err != nil {
			return fmt.Errorf("failed to set fulfilled height value: %w", err)
		}
		if err := insert(makePrefix(storage.PrefixGlobalState, storage.GlobalStatePrunedHeight), height)(tx); err != nil {
			return fmt.Errorf("failed to set pruned height value: %w", err)
		}
		return nil
	}
}

// UpdateTrackerFulfilledHeight updates the fulfilled height in the execution data tracker storage.
func UpdateTrackerFulfilledHeight(height uint64) func(*badger.Txn) error {
	return update(makePrefix(storage.PrefixGlobalState, storage.GlobalStateFulfilledHeight), height)
}

// RetrieveTrackerFulfilledHeight retrieves the fulfilled height from the execution data tracker storage.
func RetrieveTrackerFulfilledHeight(height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(storage.PrefixGlobalState, storage.GlobalStateFulfilledHeight), height)
}

// UpdateTrackerPrunedHeight updates the pruned height in the execution data tracker storage.
func UpdateTrackerPrunedHeight(height uint64) func(*badger.Txn) error {
	return update(makePrefix(storage.PrefixGlobalState, storage.GlobalStatePrunedHeight), height)
}

// RetrieveTrackerPrunedHeight retrieves the pruned height from the execution data tracker storage.
func RetrieveTrackerPrunedHeight(height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(storage.PrefixGlobalState, storage.GlobalStatePrunedHeight), height)
}

// UpsertTrackerLatestHeight set the latest height for the given CID in the execution data tracker storage.
func UpsertTrackerLatestHeight(cid cid.Cid, height uint64) func(*badger.Txn) error {
	return upsert(makePrefix(storage.PrefixLatestHeight, cid), height)
}

// RetrieveTrackerLatestHeight retrieves the latest height for the given CID from the execution data tracker storage.
func RetrieveTrackerLatestHeight(cid cid.Cid, height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(storage.PrefixLatestHeight, cid), height)
}

// BatchRemoveTrackerLatestHeight removes the latest height for the given CID in provided batch from the execution data
// tracker storage.
func BatchRemoveTrackerLatestHeight(cid cid.Cid) func(batch *badger.WriteBatch) error {
	return batchRemove(makePrefix(storage.PrefixLatestHeight, cid))
}

// InsertBlob inserts a blob record for the given block height and CID into the execution data tracker storage.
func InsertBlob(blockHeight uint64, cid cid.Cid) func(*badger.Txn) error {
	return insert(makePrefix(storage.PrefixBlobRecord, blockHeight, cid), nil)
}

// BlobExist checks whether a blob record exists in the execution data tracker storage.
func BlobExist(blockHeight uint64, cid cid.Cid, blobExists *bool) func(*badger.Txn) error {
	return exists(makePrefix(storage.PrefixBlobRecord, blockHeight, cid), blobExists)
}

// BatchRemoveBlob removes a blob record for the given block height and CID in provided batch from the execution data
// tracker storage.
func BatchRemoveBlob(blockHeight uint64, cid cid.Cid) func(batch *badger.WriteBatch) error {
	return batchRemove(makePrefix(storage.PrefixBlobRecord, blockHeight, cid))
}
