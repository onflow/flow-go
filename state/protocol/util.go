package protocol

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

// IsNodeAuthorizedAt returns whether the node with the given ID is a valid
// un-ejected network participant as of the given state snapshot.
func IsNodeAuthorizedAt(snapshot Snapshot, id flow.Identifier) (bool, error) {
	return CheckNodeStatusAt(
		snapshot,
		id,
		filter.HasWeight(true),
		filter.Not(filter.Ejected),
	)
}

// IsNodeAuthorizedWithRoleAt returns whether the node with the given ID is a valid
// un-ejected network participant with the specified role as of the given state snapshot.
// Expected errors during normal operations:
//  * storage.ErrNotFound if snapshot references an unknown block
// All other errors are unexpected and potential symptoms of internal state corruption.
func IsNodeAuthorizedWithRoleAt(snapshot Snapshot, id flow.Identifier, role flow.Role) (bool, error) {
	return CheckNodeStatusAt(
		snapshot,
		id,
		filter.HasWeight(true),
		filter.Not(filter.Ejected),
		filter.HasRole(role),
	)
}

// CheckNodeStatusAt returns whether the node with the given ID is a valid identity at the given
// state snapshot, and satisfies all checks.
// Expected errors during normal operations:
//  * storage.ErrNotFound if snapshot references an unknown block
// All other errors are unexpected and potential symptoms of internal state corruption.
func CheckNodeStatusAt(snapshot Snapshot, id flow.Identifier, checks ...flow.IdentityFilter) (bool, error) {
	identity, err := snapshot.Identity(id)
	if IsIdentityNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("could not retrieve node identity (id=%x): %w)", id, err)
	}

	for _, check := range checks {
		if !check(identity) {
			return false, nil
		}
	}

	return true, nil
}

// IsSporkRootSnapshot returns whether the given snapshot is the state snapshot
// representing the initial state for a spork.
func IsSporkRootSnapshot(snapshot Snapshot) (bool, error) {
	segment, err := snapshot.SealingSegment()
	if err != nil {
		return false, fmt.Errorf("could not get snapshot head: %w", err)
	}
	if len(segment.Blocks) > 1 {
		// spork root snapshots uniquely have only one block in the sealing segment
		return false, nil
	}
	return true, nil
}
