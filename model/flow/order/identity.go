// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package order

import (
	"github.com/onflow/flow-go/model/flow"
)

// Canonical represents the canonical ordering for identity lists.
func Canonical(identity1 *flow.Identity, identity2 *flow.Identity) int {
	return IdentifierCanonical(identity1.NodeID, identity2.NodeID)
}

// ByReferenceOrder return a function for sorting identities based on the order
// of the given nodeIDs
func ByReferenceOrder(nodeIDs []flow.Identifier) func(*flow.Identity, *flow.Identity) int {
	indices := make(map[flow.Identifier]int)
	for index, nodeID := range nodeIDs {
		_, ok := indices[nodeID]
		if ok {
			panic("should never order by reference order with duplicate node IDs")
		}
		indices[nodeID] = index
	}
	return func(identity1 *flow.Identity, identity2 *flow.Identity) int {
		return indices[identity1.NodeID] - indices[identity2.NodeID]
	}
}

// IdentityListCanonical takes a list of identities and
// check if it's strictly ordered in canonical order.
// Strict ordering means that equality in ordering is not allowed.
func IdentityListCanonical(identities flow.IdentityList) bool {
	return identities.StrictlySorted(Canonical)
}
