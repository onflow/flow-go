// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package order

import (
	"github.com/onflow/flow-go/model/flow"
)

// Canonical represents the canonical ordering for identity lists.
func Canonical[T flow.GenericIdentity](identity1 *T, identity2 *T) bool {
	return IdentifierCanonical((*identity1).GetNodeID(), (*identity2).GetNodeID())
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
func IdentityListCanonical[T flow.GenericIdentity](identities flow.GenericIdentityList[T]) bool {
	if len(identities) == 0 {
		return true
	}
	return identities.Sorted(Canonical[T])
}
