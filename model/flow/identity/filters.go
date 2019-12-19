// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package identity

import (
	"github.com/dapperlabs/flow-go/model"
	"github.com/dapperlabs/flow-go/model/flow"
)

func Not(filter flow.IdentityFilter) flow.IdentityFilter {
	return func(id flow.Identity) bool {
		return !filter(id)
	}
}

func HasNodeID(nodeIDs ...model.Identifier) flow.IdentityFilter {
	lookup := make(map[model.Identifier]struct{})
	for _, nodeID := range nodeIDs {
		lookup[nodeID] = struct{}{}
	}
	return func(id flow.Identity) bool {
		_, ok := lookup[id.NodeID]
		return ok
	}
}

func HasRole(roles ...flow.Role) flow.IdentityFilter {
	lookup := make(map[flow.Role]struct{})
	for _, role := range roles {
		lookup[role] = struct{}{}
	}
	return func(id flow.Identity) bool {
		_, ok := lookup[id.Role]
		return ok
	}
}
