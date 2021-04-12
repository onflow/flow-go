package protocol

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// IsNodeStakedAt returns whether or not the node with the given ID is a valid
// staked, un-ejected node as of the given state snapshot.
func IsNodeStakedAt(snapshot Snapshot, id flow.Identifier) (bool, error) {
	identity, err := snapshot.Identity(id)
	if IsIdentityNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("could not retrieve node identity (id=%x): %w)", id, err)
	}

	staked := identity.Stake > 0
	ejected := identity.Ejected

	return staked && !ejected, nil
}
