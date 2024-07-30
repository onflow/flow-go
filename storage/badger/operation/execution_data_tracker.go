package operation

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/storage"
)

func UpdateTrackerFulfilledHeight(height uint64) func(*badger.Txn) error {
	return update(makePrefix(storage.PrefixGlobalState, storage.GlobalStateFulfilledHeight), height)
}

func InsertTrackerFulfilledHeight(height uint64) func(*badger.Txn) error {
	return insert(makePrefix(storage.PrefixGlobalState, storage.GlobalStateFulfilledHeight), height)
}

func RetrieveTrackerFulfilledHeight(height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(storage.PrefixGlobalState, storage.GlobalStateFulfilledHeight), height)
}

func UpdateTrackerPrunedHeight(height uint64) func(*badger.Txn) error {
	return update(makePrefix(storage.PrefixGlobalState, storage.GlobalStatePrunedHeight), height)
}

func InsertTrackerPrunedHeight(height uint64) func(*badger.Txn) error {
	return insert(makePrefix(storage.PrefixGlobalState, storage.GlobalStatePrunedHeight), height)
}

func RetrieveTrackerPrunedHeight(height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(storage.PrefixGlobalState, storage.GlobalStatePrunedHeight), height)
}

func UpsertTrackerLatestHeight(cid cid.Cid, height uint64) func(*badger.Txn) error {
	return upsert(makePrefix(storage.PrefixLatestHeight, cid), height)
}

func RetrieveTrackerLatestHeight(cid cid.Cid, height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(storage.PrefixLatestHeight, cid), height)
}

func RemoveTrackerLatestHeight(cid cid.Cid) func(*badger.Txn) error {
	return remove(makePrefix(storage.PrefixLatestHeight, cid))
}

func InsertBlob(blockHeight uint64, cid cid.Cid) func(*badger.Txn) error {
	return insert(makePrefix(storage.PrefixBlobRecord, blockHeight, cid), nil)
}

func RetrieveBlob(blockHeight uint64, cid cid.Cid) func(*badger.Txn) error {
	return retrieve(makePrefix(storage.PrefixBlobRecord, blockHeight, cid), nil)
}

func RemoveBlob(blockHeight uint64, cid cid.Cid) func(*badger.Txn) error {
	return remove(makePrefix(storage.PrefixBlobRecord, blockHeight, cid))
}
