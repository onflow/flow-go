// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package order

import (
	"github.com/onflow/flow-go/model/flow"
)

// Canonical represents the canonical ordering for identity lists.
func Canonical(identity1 *flow.Identity, identity2 *flow.Identity) bool {
	return IdentifierCanonical(identity1.NodeID, identity2.NodeID)
}

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
