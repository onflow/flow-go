package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// PurgeBlacklistedNodes removes the set of blacklisted nodes IDs from the data base.
// If no corresponding entry exists, this function is a no-op.
// No errors are expected during normal operations.
// TODO: TEMPORARY manual override for adding node IDs to list of ejected nodes, applies to networking layer only
func PurgeBlacklistedNodes() func(*badger.Txn) error {
	return remove(makePrefix(blockedNodeIDs))
}

// PersistBlacklistedNodes writes the set of blacklisted nodes IDs into the data base.
// If an entry already exists, it is overwritten; otherwise a new entry is created.
// No errors are expected during normal operations.
//
// TODO: TEMPORARY manual override for adding node IDs to list of ejected nodes, applies to networking layer only
func PersistBlacklistedNodes(blacklist map[flow.Identifier]struct{}) func(*badger.Txn) error {
	return upsert(makePrefix(blockedNodeIDs), blacklist)
}

// RetrieveBlacklistedNodes reads the set of blacklisted node IDs from the data base.
// Returns `storage.ErrNotFound` error in case no respective data base entry is present.
//
// TODO: TEMPORARY manual override for adding node IDs to list of ejected nodes, applies to networking layer only
func RetrieveBlacklistedNodes(blacklist *map[flow.Identifier]struct{}) func(*badger.Txn) error {
	return retrieve(makePrefix(blockedNodeIDs), blacklist)
}
