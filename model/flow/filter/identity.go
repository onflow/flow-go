// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package filter

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Not returns a filter equivalent to the inverse of the input filter.
func Not(filter flow.IdentityFilter) flow.IdentityFilter {
	return func(id *flow.Identity) bool {
		return !filter(id)
	}
}

// In returns a filter for identities within the input list. This is equivalent
// to HasNodeID, but for list-typed inputs.
func In(list flow.IdentityList) flow.IdentityFilter {
	return HasNodeID(list.NodeIDs()...)
}

// HasNodeID returns a filter that returns true for any identity with an ID
// matching any of the inputs.
func HasNodeID(nodeIDs ...flow.Identifier) flow.IdentityFilter {
	lookup := make(map[flow.Identifier]struct{})
	for _, nodeID := range nodeIDs {
		lookup[nodeID] = struct{}{}
	}
	return func(id *flow.Identity) bool {
		_, ok := lookup[id.NodeID]
		return ok
	}
}

// HasStake returns a filter for nodes with non-zero stake.
func HasStake(id *flow.Identity) bool {
	return id.Stake > 0
}

// HasRole returns a filter for nodes with one of the input roles.
func HasRole(roles ...flow.Role) flow.IdentityFilter {
	lookup := make(map[flow.Role]struct{})
	for _, role := range roles {
		lookup[role] = struct{}{}
	}
	return func(id *flow.Identity) bool {
		_, ok := lookup[id.Role]
		return ok
	}
}
