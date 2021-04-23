package ptrie

import (
	"encoding/hex"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
)

// Root hash of a node
type Root hash.Hash

func (r Root) String() string {
	return hex.EncodeToString(r[:])
}

// node is a struct for constructing our Tree.
// Height Definition: the height of a node v in a tree is the number
// of edges on the longest downward path between v and a tree leaf.
type node struct {
	lChild    *node           // Left Child
	rChild    *node           // Right Child
	height    int             // Height where the node is at
	path      ledger.Path     // path
	payload   *ledger.Payload // payload
	hashValue hash.Hash       // hash value
}

// newNode creates a new node with the provided height and hash
func newNode(v hash.Hash, height int) *node {
	n := new(node)
	n.hashValue = v
	n.height = height
	n.lChild = nil
	n.rChild = nil
	return n
}

// TODO: revisit this
// ComputeValue recomputes value for this node in recursive manner
func (n *node) HashValue() hash.Hash {
	// leaf node
	if n.lChild == nil && n.rChild == nil {
		return n.hashValue
	}
	// otherwise compute
	var h1, h2 hash.Hash
	if n.lChild != nil {
		h1 = n.lChild.HashValue()
	} else {
		h1 = ledger.GetDefaultHashForHeight(n.height - 1)
	}

	if n.rChild != nil {
		h2 = n.rChild.HashValue()
	} else {
		h2 = ledger.GetDefaultHashForHeight(n.height - 1)
	}
	// For debugging purpose uncomment this
	// n.value = HashInterNode(h1, h2)
	return hash.HashInterNode(h1, h2)
}
