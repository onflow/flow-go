package operation

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
)

// UpdateBlockedNodes updates the list of blocked nodes IDs in the data base
// TODO: TEMPORARY manual override for adding node IDs to list of ejected nodes, applies to networking layer only
func UpdateBlockedNodes(blockedIDs []flow.Identifier) func(*badger.Txn) error {
	return update(makePrefix(blockedNodeIDs), blockedIDs)
}

// RetrieveBlockedNodes reads the list of blocked node IDs from the data base
// TODO: TEMPORARY manual override for adding node IDs to list of ejected nodes, applies to networking layer only
func RetrieveBlockedNodes(blockedIDs *[]flow.Identifier) func(*badger.Txn) error {
	return retrieve(makePrefix(blockedNodeIDs), blockedIDs)
}
