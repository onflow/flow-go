package operation

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// PurgeNodeDisallowList removes the set of disallowed nodes IDs from the database.
// If no corresponding entry exists, this function is a no-op.
// No errors are expected during normal operations.
//
// TODO: TEMPORARY manual override for adding node IDs to list of ejected nodes, applies to networking layer only
func PurgeNodeDisallowList(w storage.Writer) error {
	err := RemoveByKey(w, MakePrefix(disallowedNodeIDs))
	if err != nil {
		return fmt.Errorf("unexpected error while purging disallow list: %w", err)
	}
	return nil
}

// PersistNodeDisallowList writes the set of disallowed nodes IDs into the database.
// If an entry already exists, it is overwritten; otherwise a new entry is created.
// No errors are expected during normal operations.
//
// TODO: TEMPORARY manual override for adding node IDs to list of ejected nodes, applies to networking layer only
func PersistNodeDisallowList(w storage.Writer, disallowList map[flow.Identifier]struct{}) error {
	return UpsertByKey(w, MakePrefix(disallowedNodeIDs), disallowList)
}

// RetrieveNodeDisallowList reads the set of disallowed node IDs from the database.
// Returns `storage.ErrNotFound` error in case no respective database entry is present.
//
// TODO: TEMPORARY manual override for adding node IDs to list of ejected nodes, applies to networking layer only
func RetrieveNodeDisallowList(r storage.Reader, disallowList *map[flow.Identifier]struct{}) error {
	return RetrieveByKey(r, MakePrefix(disallowedNodeIDs), disallowList)
}
