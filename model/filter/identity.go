// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package filter

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
)

// Address filters nodes for the given addresses.
func Address(addresses ...string) module.IdentityFilter {
	lookup := make(map[string]struct{})
	for _, address := range addresses {
		lookup[address] = struct{}{}
	}
	return func(identity flow.Identity) bool {
		_, ok := lookup[identity.Address]
		return ok
	}
}

// NodeID ids nodes for the given roles.
func NodeID(nodeIDs ...string) module.IdentityFilter {
	lookup := make(map[string]struct{})
	for _, nodeID := range nodeIDs {
		lookup[nodeID] = struct{}{}
	}
	return func(identity flow.Identity) bool {
		_, ok := lookup[identity.NodeID]
		return ok
	}
}

// Not filters nodes that are the opposite of the wrapped filter.
func Not(filter module.IdentityFilter) module.IdentityFilter {
	return func(identity flow.Identity) bool {
		return !filter(identity)
	}
}

// Role filters nodes for the given roles.
func Role(roles ...string) module.IdentityFilter {
	lookup := make(map[string]struct{})
	for _, role := range roles {
		lookup[role] = struct{}{}
	}
	return func(identity flow.Identity) bool {
		_, ok := lookup[identity.Role]
		return ok
	}
}
