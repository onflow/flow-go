package operation

import (
	"github.com/cockroachdb/pebble"
	"github.com/ipfs/go-cid"

	"github.com/onflow/flow-go/storage"
)

func UpdateTrackerFulfilledHeight(height uint64) func(w pebble.Writer) error {
	return insert(makePrefix(storage.PrefixGlobalState, storage.GlobalStateFulfilledHeight), height)
}

func InitTrackerFulfilledHeight(height uint64) func(w pebble.Writer) error {
	return insert(makePrefix(storage.PrefixGlobalState, storage.GlobalStateFulfilledHeight), height)
}

func RetrieveTrackerFulfilledHeight(height *uint64) func(r pebble.Reader) error {
	return retrieve(makePrefix(storage.PrefixGlobalState, storage.GlobalStateFulfilledHeight), height)
}

func UpdateTrackerPrunedHeight(height uint64) func(w pebble.Writer) error {
	return insert(makePrefix(storage.PrefixGlobalState, storage.GlobalStatePrunedHeight), height)
}

func InitTrackerPrunedHeight(height uint64) func(w pebble.Writer) error {
	return insert(makePrefix(storage.PrefixGlobalState, storage.GlobalStatePrunedHeight), height)
}

func RetrieveTrackerPrunedHeight(height *uint64) func(r pebble.Reader) error {
	return retrieve(makePrefix(storage.PrefixGlobalState, storage.GlobalStatePrunedHeight), height)
}

func UpsertTrackerLatestHeight(cid cid.Cid, height uint64) func(w pebble.Writer) error {
	return insert(makePrefix(storage.PrefixLatestHeight, cid), height)
}

func RetrieveTrackerLatestHeight(cid cid.Cid, height *uint64) func(r pebble.Reader) error {
	return retrieve(makePrefix(storage.PrefixLatestHeight, cid), height)
}

func RemoveTrackerLatestHeight(cid cid.Cid) func(w pebble.Writer) error {
	return remove(makePrefix(storage.PrefixLatestHeight, cid))
}

func InsertBlob(blockHeight uint64, cid cid.Cid) func(w pebble.Writer) error {
	return insert(makePrefix(storage.PrefixBlobRecord, blockHeight, cid), nil)
}

func RetrieveBlob(blockHeight uint64, cid cid.Cid) func(r pebble.Reader) error {
	return retrieve(makePrefix(storage.PrefixBlobRecord, blockHeight, cid), nil)
}

func RemoveBlob(blockHeight uint64, cid cid.Cid) func(w pebble.Writer) error {
	return remove(makePrefix(storage.PrefixBlobRecord, blockHeight, cid))
}
