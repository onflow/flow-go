// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

// Node represents a node identity.
type Node struct {
	ID      string
	Address string
	Role    string
}

// NodeList is a list of nodes.
type NodeList []Node

// IDs returns the IDs of the nodes in the list.
func (nl NodeList) IDs() []string {
	ids := make([]string, 0, len(nl))
	for _, node := range nl {
		ids = append(ids, node.ID)
	}
	return ids
}
