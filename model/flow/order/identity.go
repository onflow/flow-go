// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package order

import (
	"github.com/onflow/flow-go/model/flow"
)

// Canonical represents the canonical ordering for identity lists.
func Canonical(identity1 *flow.Identity, identity2 *flow.Identity) int {
	return IdentifierCompare(identity1.NodeID, identity2.NodeID)
}

// ByReferenceOrder return a function for sorting identities based on the order
// of the given nodeIDs
func ByReferenceOrder(nodeIDs []flow.Identifier) func(*flow.Identity, *flow.Identity) bool {
	indices := make(map[flow.Identifier]uint)
	for index, nodeID := range nodeIDs {
		_, ok := indices[nodeID]
		if ok {
			panic("should never order by reference order with duplicate node IDs")
		}
		indices[nodeID] = uint(index)
	}
	return func(identity1 *flow.Identity, identity2 *flow.Identity) bool {
		return indices[identity1.NodeID] < indices[identity2.NodeID]
	}
}

// IdentityListCanonical takes a list of identities and
// check if it's ordered in canonical order.
func IdentityListCanonical(identities flow.IdentityList) bool {
	if len(identities) == 0 {
		return true
	}

	prev := identities[0].ID()
	for i := 1; i < len(identities); i++ {
		id := identities[i].ID()
		if !IdentifierCanonical(prev, id) {
			return false
		}
		prev = id
	}

	return true
}
