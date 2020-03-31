// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package filter

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Any will always be true.
func Any(*flow.Identity) bool {
	return true
}

// And combines two or more filters that all need to be true.
func And(filters ...flow.IdentityFilter) flow.IdentityFilter {
	return func(identity *flow.Identity) bool {
		for _, filter := range filters {
			if !filter(identity) {
				return false
			}
		}
		return true
	}
}

// Or combines two or more filters and only needs one of them to be true.
func Or(filters ...flow.IdentityFilter) flow.IdentityFilter {
	return func(identity *flow.Identity) bool {
		for _, filter := range filters {
			if filter(identity) {
				return true
			}
		}
		return false
	}
}

// Not returns a filter equivalent to the inverse of the input filter.
func Not(filter flow.IdentityFilter) flow.IdentityFilter {
	return func(identity *flow.Identity) bool {
		return !filter(identity)
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
	return func(identity *flow.Identity) bool {
		_, ok := lookup[identity.NodeID]
		return ok
	}
}

// HasStake returns a filter for nodes with non-zero stake.
func HasStake(hasStake bool) flow.IdentityFilter {
	return func(identity *flow.Identity) bool {
		return (identity.Stake > 0) == hasStake
	}
}

// HasRole returns a filter for nodes with one of the input roles.
func HasRole(roles ...flow.Role) flow.IdentityFilter {
	lookup := make(map[flow.Role]struct{})
	for _, role := range roles {
		lookup[role] = struct{}{}
	}
	return func(identity *flow.Identity) bool {
		_, ok := lookup[identity.Role]
		return ok
	}
}
