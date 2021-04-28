package protocol

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

// IsNodeStakedAt returns whether or not the node with the given ID is a valid
// staked, un-ejected node as of the given state snapshot.
func IsNodeStakedAt(snapshot Snapshot, id flow.Identifier) (bool, error) {
	return CheckNodeStatusAt(
		snapshot,
		id,
		filter.HasStake(true),
		filter.Not(filter.Ejected),
	)
}

// IsNodeStakedWithRoleAt returns whether or not the node with the given ID is a valid
// staked, un-ejected node with the specified role as of the given state snapshot.
func IsNodeStakedWithRoleAt(snapshot Snapshot, id flow.Identifier, role flow.Role) (bool, error) {
	return CheckNodeStatusAt(
		snapshot,
		id,
		filter.HasStake(true),
		filter.Not(filter.Ejected),
		filter.HasRole(role),
	)
}

// CheckNodeStatusAt returns whether or not the node with the given ID is a valid identity at the given
// state snapshot, and satisfies all checks.
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
