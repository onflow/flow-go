// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

// Identity represents a node identity.
type Identity struct {
	NodeID  string
	Address string
	Role    string
}

// IdentityList is a list of nodes.
type IdentityList []Identity

// NodeIDs returns the NodeIDs of the nodes in the list.
func (il IdentityList) NodeIDs() []string {
	ids := make([]string, 0, len(il))
	for _, identity := range il {
		ids = append(ids, identity.NodeID)
	}
	return ids
}
