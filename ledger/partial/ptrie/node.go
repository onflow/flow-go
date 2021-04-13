package ptrie

import (
	"encoding/hex"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common"
)

type Root []byte

func (r Root) String() string {
	return hex.EncodeToString(r)
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
	hashValue []byte          // hash value
}

// newNode creates a new node with the provided value and no children
func newNode(hashValue []byte, height int) *node {
	n := new(node)
	n.hashValue = make([]byte, 0, common.HashLen)
	n.hashValue = append(n.hashValue, hashValue...)
	// pad the hash to Hashlen
	paddingLen := common.HashLen - len(hashValue)
	n.hashValue = append(n.hashValue, common.EmptyHash[:paddingLen]...)

	n.height = height
	n.lChild = nil
	n.rChild = nil
	return n
}

// TODO: revisit this
// ComputeValue recomputes value for this node in recursive manner
func (n *node) HashValue() []byte {
	// leaf node
	if n.lChild == nil && n.rChild == nil {
		if n.hashValue != nil {
			return n.hashValue
		}
		return common.GetDefaultHashForHeight(n.height)
	}
	// otherwise compute
	h1 := common.GetDefaultHashForHeight(n.height - 1)
	if n.lChild != nil {
		h1 = n.lChild.HashValue()
	}
	h2 := common.GetDefaultHashForHeight(n.height - 1)
	if n.rChild != nil {
		h2 = n.rChild.HashValue()
	}
	// For debugging purpose uncomment this
	// n.value = HashInterNode(h1, h2)
	return common.HashInterNode(h1, h2)
}
