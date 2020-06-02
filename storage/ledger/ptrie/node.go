package ptrie

import (
	"encoding/hex"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/common"
)

type Root []byte

func (r Root) String() string {
	return hex.EncodeToString(r)
}

// node is a struct for constructing our Tree.
// HEIGHT DEFINITION: the height of a node v in a tree is the number
// of edges on the longest downward path between v and a tree leaf.
type node struct {
	value  Root   // Hash
	lChild *node  // Left Child
	rChild *node  // Right Child
	height int    // Height where the node is at
	key    []byte // key this node is pointing at
}

// newNode creates a new node with the provided value and no children
func newNode(value []byte, height int) *node {
	n := new(node)
	n.value = value
	n.height = height
	n.lChild = nil
	n.rChild = nil
	n.key = nil
	return n
}

// ComputeValue recomputes value for this node in recursive manner
func (n *node) ComputeValue() []byte {
	// leaf node
	if n.lChild == nil && n.rChild == nil {
		if n.value != nil {
			return n.value
		}
		return common.GetDefaultHashForHeight(n.height)
	}
	// otherwise compute
	h1 := common.GetDefaultHashForHeight(n.height - 1)
	if n.lChild != nil {
		h1 = n.lChild.ComputeValue()
	}
	h2 := common.GetDefaultHashForHeight(n.height - 1)
	if n.rChild != nil {
		h2 = n.rChild.ComputeValue()
	}
	// For debugging purpose uncomment this
	// n.value = HashInterNode(h1, h2)
	return common.HashInterNode(h1, h2)
}
