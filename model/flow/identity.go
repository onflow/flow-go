// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import "sort"

// Identity represents a node identity.
type Identity struct {
	NodeID  string
	Address string
	Role    string
}

// IdentityFilter is a function that returns true if we want to include a node.
type IdentityFilter func(Identity) bool

// IdentitySort is a function that sorts the nodes.
type IdentitySort func(Identity, Identity) bool

// IdentityList is a list of nodes.
type IdentityList []Identity

// Filter will filter the identity list.
func (il IdentityList) Filter(filter IdentityFilter) IdentityList {
	var dup IdentityList
	for _, identity := range il {
		if filter(identity) {
			dup = append(dup, identity)
		}
	}
	return dup
}

// Sort will sort the identity list.
func (il IdentityList) Sort(less IdentitySort) IdentityList {
	sort.Slice(il, func(i int, j int) bool {
		return less(il[i], il[j])
	})
	return il
}

// NodeIDs returns the NodeIDs of the nodes in the list.
func (il IdentityList) NodeIDs() []string {
	ids := make([]string, 0, len(il))
	for _, identity := range il {
		ids = append(ids, identity.NodeID)
	}
	return ids
}
