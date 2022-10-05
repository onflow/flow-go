package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// PurgeBlocklist removes the set of blocked nodes IDs from the data base.
// If no corresponding entry exists, this function is a no-op.
// No errors are expected during normal operations.
// TODO: TEMPORARY manual override for adding node IDs to list of ejected nodes, applies to networking layer only
func PurgeBlocklist() func(*badger.Txn) error {
	return remove(makePrefix(blockedNodeIDs))
}

// PersistBlocklist writes the set of blocked nodes IDs into the data base.
// If an entry already exists, it is overwritten; otherwise a new entry is created.
// No errors are expected during normal operations.
//
// TODO: TEMPORARY manual override for adding node IDs to list of ejected nodes, applies to networking layer only
func PersistBlocklist(blocklist map[flow.Identifier]struct{}) func(*badger.Txn) error {
	return upsert(makePrefix(blockedNodeIDs), blocklist)
}

// RetrieveBlocklist reads the set of blocked node IDs from the data base.
// Returns `storage.ErrNotFound` error in case no respective data base entry is present.
//
// TODO: TEMPORARY manual override for adding node IDs to list of ejected nodes, applies to networking layer only
func RetrieveBlocklist(blocklist *map[flow.Identifier]struct{}) func(*badger.Txn) error {
	return retrieve(makePrefix(blockedNodeIDs), blocklist)
}
