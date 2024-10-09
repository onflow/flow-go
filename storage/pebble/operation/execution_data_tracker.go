package operation

import (
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/storage"
)

// UpdateTrackerFulfilledHeight updates the fulfilled height in the execution data tracker storage.
func UpdateTrackerFulfilledHeight(height uint64) func(w pebble.Writer) error {
	return insert(makePrefix(storage.PrefixGlobalState, storage.GlobalStateFulfilledHeight), height)
}

// InitTrackerHeights initializes the fulfilled and pruned heights for the execution data tracker storage.
func InitTrackerHeights(height uint64) func(storage.PebbleReaderBatchWriter) error {
	return func(tx storage.PebbleReaderBatchWriter) error {
		_, w := tx.ReaderWriter()

		err := insert(makePrefix(storage.PrefixGlobalState, storage.GlobalStateFulfilledHeight), height)(w)
		if err != nil {
			return fmt.Errorf("failed to set fulfilled height value: %w", err)
		}

		err = insert(makePrefix(storage.PrefixGlobalState, storage.GlobalStatePrunedHeight), height)(w)
		if err != nil {
			return fmt.Errorf("failed to set pruned height value: %w", err)
		}

		return nil
	}
}

// RetrieveTrackerFulfilledHeight retrieves the fulfilled height from the execution data tracker storage.
func RetrieveTrackerFulfilledHeight(height *uint64) func(r pebble.Reader) error {
	return retrieve(makePrefix(storage.PrefixGlobalState, storage.GlobalStateFulfilledHeight), height)
}

// UpdateTrackerPrunedHeight updates the pruned height in the execution data tracker storage.
func UpdateTrackerPrunedHeight(height uint64) func(w pebble.Writer) error {
	return insert(makePrefix(storage.PrefixGlobalState, storage.GlobalStatePrunedHeight), height)
}

// RetrieveTrackerPrunedHeight retrieves the pruned height from the execution data tracker storage.
func RetrieveTrackerPrunedHeight(height *uint64) func(r pebble.Reader) error {
	return retrieve(makePrefix(storage.PrefixGlobalState, storage.GlobalStatePrunedHeight), height)
}

// UpsertTrackerLatestHeight set the latest height for the given CID in the execution data tracker storage.
func UpsertTrackerLatestHeight(cid cid.Cid, height uint64) func(w pebble.Writer) error {
	return insert(makePrefix(storage.PrefixLatestHeight, cid), height)
}

// RetrieveTrackerLatestHeight retrieves the latest height for the given CID from the execution data tracker storage.
func RetrieveTrackerLatestHeight(cid cid.Cid, height *uint64) func(r pebble.Reader) error {
	return retrieve(makePrefix(storage.PrefixLatestHeight, cid), height)
}

// RemoveTrackerLatestHeight removes the latest height for the given CID from the execution data tracker storage.
func RemoveTrackerLatestHeight(cid cid.Cid) func(w pebble.Writer) error {
	return remove(makePrefix(storage.PrefixLatestHeight, cid))
}

// InsertBlob inserts a blob record for the given block height and CID into the execution data tracker storage.
func InsertBlob(blockHeight uint64, cid cid.Cid) func(w pebble.Writer) error {
	return insert(makePrefix(storage.PrefixBlobRecord, blockHeight, cid), nil)
}

// BlobExist checks whether a blob record exists in the execution data tracker storage.
func BlobExist(blockHeight uint64, cid cid.Cid, blobExists *bool) func(r pebble.Reader) error {
	return exists(makePrefix(storage.PrefixBlobRecord, blockHeight, cid), blobExists)
}

// RemoveBlob removes a blob record for the given block height and CID from the execution data tracker storage.
func RemoveBlob(blockHeight uint64, cid cid.Cid) func(w pebble.Writer) error {
	return remove(makePrefix(storage.PrefixBlobRecord, blockHeight, cid))
}
