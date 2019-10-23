// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package filter

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
)

// Address filters nodes for the given addresses.
func Address(addresses ...string) module.NodeFilter {
	lookup := make(map[string]struct{})
	for _, address := range addresses {
		lookup[address] = struct{}{}
	}
	return func(n flow.Node) bool {
		_, ok := lookup[n.Address]
		return ok
	}
}

// ID ids nodes for the given roles.
func ID(ids ...string) module.NodeFilter {
	lookup := make(map[string]struct{})
	for _, id := range ids {
		lookup[id] = struct{}{}
	}
	return func(n flow.Node) bool {
		_, ok := lookup[n.ID]
		return ok
	}
}

// Not filters nodes that are the opposite of the wrapped filter.
func Not(filter module.NodeFilter) module.NodeFilter {
	return func(n flow.Node) bool {
		return !filter(n)
	}
}

// Role filters nodes for the given roles.
func Role(roles ...string) module.NodeFilter {
	lookup := make(map[string]struct{})
	for _, role := range roles {
		lookup[role] = struct{}{}
	}
	return func(n flow.Node) bool {
		_, ok := lookup[n.Role]
		return ok
	}
}
