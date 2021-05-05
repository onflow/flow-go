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

// Hash returns the node's pre-computed hash value
func (n *node) Hash() hash.Hash { return n.hashValue }

// forceComputeHash computes (and updates) the hashes of _all interior nodes_
// of this sub-trie. Caution: this is an expensive operation!
func (n *node) forceComputeHash() hash.Hash {
	// leaf node
	if n.lChild == nil && n.rChild == nil {
		return n.hashValue
	}
	// otherwise compute
	var h1, h2 hash.Hash
	if n.lChild != nil {
		h1 = n.lChild.forceComputeHash()
	} else {
		h1 = ledger.GetDefaultHashForHeight(n.height - 1)
	}

	if n.rChild != nil {
		h2 = n.rChild.forceComputeHash()
	} else {
		h2 = ledger.GetDefaultHashForHeight(n.height - 1)
	}
	n.hashValue = hash.HashInterNode(h1, h2)
	return n.hashValue
}
