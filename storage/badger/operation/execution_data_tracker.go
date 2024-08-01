package operation

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/storage"
)

// UpdateTrackerFulfilledHeight updates the fulfilled height in the execution data tracker storage.
func UpdateTrackerFulfilledHeight(height uint64) func(*badger.Txn) error {
	return update(makePrefix(storage.PrefixGlobalState, storage.GlobalStateFulfilledHeight), height)
}

// InitTrackerFulfilledHeight initializes the fulfilled height for the execution data tracker storage.
func InitTrackerFulfilledHeight(height uint64) func(*badger.Txn) error {
	return insert(makePrefix(storage.PrefixGlobalState, storage.GlobalStateFulfilledHeight), height)
}

// RetrieveTrackerFulfilledHeight retrieves the fulfilled height from the execution data tracker storage.
func RetrieveTrackerFulfilledHeight(height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(storage.PrefixGlobalState, storage.GlobalStateFulfilledHeight), height)
}

// UpdateTrackerPrunedHeight updates the pruned height in the execution data tracker storage.
func UpdateTrackerPrunedHeight(height uint64) func(*badger.Txn) error {
	return update(makePrefix(storage.PrefixGlobalState, storage.GlobalStatePrunedHeight), height)
}

// InitTrackerPrunedHeight initializes the pruned height for the execution data tracker storage.
func InitTrackerPrunedHeight(height uint64) func(*badger.Txn) error {
	return insert(makePrefix(storage.PrefixGlobalState, storage.GlobalStatePrunedHeight), height)
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

// RemoveTrackerLatestHeight removes the latest height for the given CID from the execution data tracker storage.
func RemoveTrackerLatestHeight(cid cid.Cid) func(*badger.Txn) error {
	return remove(makePrefix(storage.PrefixLatestHeight, cid))
}

// InsertBlob inserts a blob record for the given block height and CID into the execution data tracker storage.
func InsertBlob(blockHeight uint64, cid cid.Cid) func(*badger.Txn) error {
	return insert(makePrefix(storage.PrefixBlobRecord, blockHeight, cid), nil)
}

// RetrieveBlob retrieves a blob record for the given block height and CID from the execution data tracker storage.
func RetrieveBlob(blockHeight uint64, cid cid.Cid) func(*badger.Txn) error {
	return retrieve(makePrefix(storage.PrefixBlobRecord, blockHeight, cid), nil)
}

// RemoveBlob removes a blob record for the given block height and CID from the execution data tracker storage.
func RemoveBlob(blockHeight uint64, cid cid.Cid) func(*badger.Txn) error {
	return remove(makePrefix(storage.PrefixBlobRecord, blockHeight, cid))
}
