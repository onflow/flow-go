package operation

import (
	"github.com/dgraph-io/badger/v2"
	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/module/executiondatasync/tracker"
)

func UpdateTrackerFulfilledHeight(height uint64) func(*badger.Txn) error {
	return update(makePrefix(tracker.PrefixGlobalState, tracker.GlobalStateFulfilledHeight), height)
}

func InsertTrackerFulfilledHeight(height uint64) func(*badger.Txn) error {
	return insert(makePrefix(tracker.PrefixGlobalState, tracker.GlobalStateFulfilledHeight), height)
}

func RetrieveTrackerFulfilledHeight(height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(tracker.PrefixGlobalState, tracker.GlobalStateFulfilledHeight), height)
}

func UpdateTrackerPrunedHeight(height uint64) func(*badger.Txn) error {
	return update(makePrefix(tracker.PrefixGlobalState, tracker.GlobalStatePrunedHeight), height)
}

func InsertTrackerPrunedHeight(height uint64) func(*badger.Txn) error {
	return insert(makePrefix(tracker.PrefixGlobalState, tracker.GlobalStatePrunedHeight), height)
}

func RetrieveTrackerPrunedHeight(height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(tracker.PrefixGlobalState, tracker.GlobalStatePrunedHeight), height)
}

func UpsertTrackerLatestHeight(cid cid.Cid, height uint64) func(*badger.Txn) error {
	return upsert(makePrefix(tracker.PrefixLatestHeight, cid), height)
}

func RetrieveTrackerLatestHeight(cid cid.Cid, height *uint64) func(*badger.Txn) error {
	return retrieve(makePrefix(tracker.PrefixLatestHeight, cid), height)
}

func RemoveTrackerLatestHeight(cid cid.Cid) func(*badger.Txn) error {
	return remove(makePrefix(tracker.PrefixLatestHeight, cid))
}

func InsertBlob(blockHeight uint64, cid cid.Cid) func(*badger.Txn) error {
	return insert(makePrefix(tracker.PrefixBlobRecord, blockHeight, cid), nil)
}

func RetrieveBlob(blockHeight uint64, cid cid.Cid) func(*badger.Txn) error {
	return retrieve(makePrefix(tracker.PrefixBlobRecord, blockHeight, cid), nil)
}

func RemoveBlob(blockHeight uint64, cid cid.Cid) func(*badger.Txn) error {
	return remove(makePrefix(tracker.PrefixBlobRecord, blockHeight, cid))
}
