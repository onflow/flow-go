package operation

import (
	"errors"
	"fmt"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// PurgeBlocklist removes the set of blocked nodes IDs from the data base.
// If no corresponding entry exists, this function is a no-op.
// No errors are expected during normal operations.
// TODO: TEMPORARY manual override for adding node IDs to list of ejected nodes, applies to networking layer only
func PurgeBlocklist() func(pebble.Writer) error {
	return func(tx pebble.Writer) error {
		err := remove(makePrefix(blockedNodeIDs))(tx)
		if err != nil && !errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("enexpected error while purging blocklist: %w", err)
		}
		return nil
	}
}

// PersistBlocklist writes the set of blocked nodes IDs into the data base.
// If an entry already exists, it is overwritten; otherwise a new entry is created.
// No errors are expected during normal operations.
//
// TODO: TEMPORARY manual override for adding node IDs to list of ejected nodes, applies to networking layer only
func PersistBlocklist(blocklist map[flow.Identifier]struct{}) func(pebble.Writer) error {
	return insert(makePrefix(blockedNodeIDs), blocklist)
}

// RetrieveBlocklist reads the set of blocked node IDs from the data base.
// Returns `storage.ErrNotFound` error in case no respective data base entry is present.
//
// TODO: TEMPORARY manual override for adding node IDs to list of ejected nodes, applies to networking layer only
func RetrieveBlocklist(blocklist *map[flow.Identifier]struct{}) func(pebble.Reader) error {
	return retrieve(makePrefix(blockedNodeIDs), blocklist)
}
