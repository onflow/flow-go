package node

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common"
	"github.com/onflow/flow-go/ledger/common/utils"
)

// Node defines an Mtrie node
//
// DEFINITIONS:
//     * HEIGHT of a node v in a tree is the number of edges on the longest
//       downward path between v and a tree leaf.
//
// Conceptually, an MTrie is a sparse Merkle Trie, which has three different types of nodes:
//    * LEAF node: fully defined by a storage path, a key-value pair and a height
//      hash is pre-computed, lChild and rChild are nil)
//    * INTERIOR node: at least one of lChild or rChild is not nil.
//      Height, and Hash value are set; (key-value is nil)
//    * ROOT of empty trie node: this is a special case, where the node
//      has no children, and no key-value
// Currently, we represent both data structures by Node instances
//
// Nodes are supposed to be used in READ-ONLY fashion. However,
// for performance reasons, we not not copy read.
// TODO: optimized data structures might be able to reduce memory consumption
type Node struct {
	lChild    *Node           // Left Child
	rChild    *Node           // Right Child
	height    int             // height where the Node is at
	path      ledger.Path     // the storage path (leaf nodes only)
	payload   *ledger.Payload // the payload this node is storing (leaf nodes only)
	hashValue []byte          // hash value of node (cached)
	maxDepth  uint16          // captures the longest path from this node to compacted leafs in the subtree
	regCount  uint64          // number of registers allocated in the subtree
}

// why depth is in 16 bits?
// is 64 bits enough for regCount?

// NewNode creates a new Node.
// UNCHECKED requirement: combination of values must conform to
// a valid node type (see documentation of `Node` for details)
func NewNode(height int,
	lchild,
	rchild *Node,
	path ledger.Path,
	payload *ledger.Payload,
	hashValue []byte,
	maxDepth uint16,
	regCount uint64) *Node {

	var pl *ledger.Payload
	var p ledger.Path
	if payload != nil {
		pl = payload.DeepCopy()
	}
	if len(path) > 0 {
		p = path.DeepCopy()
	}
	n := &Node{
		lChild:    lchild,
		rChild:    rchild,
		height:    height,
		path:      p,
		payload:   pl,
		hashValue: hashValue,
		maxDepth:  maxDepth,
		regCount:  regCount,
	}
	return n
}

// NewEmptyTreeRoot creates a compact leaf Node
// UNCHECKED requirement: height must be non-negative
func NewEmptyTreeRoot(height int) *Node {
	n := &Node{
		lChild:    nil,
		rChild:    nil,
		height:    height,
		path:      nil,
		payload:   nil,
		hashValue: nil,
		maxDepth:  0,
		regCount:  0,
	}
	n.hashValue = n.computeHash()
	return n
}

// NewLeaf creates a compact leaf Node
// UNCHECKED requirement: height must be non-negative
func NewLeaf(path ledger.Path,
	payload *ledger.Payload,
	height int) *Node {

	regCount := uint64(0)
	if path != nil {
		regCount = uint64(1)
	}

	n := &Node{
		lChild:   nil,
		rChild:   nil,
		height:   height,
		path:     path.DeepCopy(),
		payload:  payload.DeepCopy(),
		maxDepth: 0,
		regCount: regCount,
	}
	n.hashValue = n.computeHash()
	return n
}

// NewInterimNode creates a new Node with the provided value and no children.
// UNCHECKED requirement: lchild.height and rchild.height must be smaller than height
// UNCHECKED requirement: if lchild != nil then height = lchild.height + 1, and same for rchild
func NewInterimNode(height int, lchild, rchild *Node) *Node {
	var lMaxDepth, rMaxDepth uint16
	var lRegCount, rRegCount uint64
	if lchild != nil {
		lMaxDepth = lchild.maxDepth
		lRegCount = lchild.regCount
	}
	if rchild != nil {
		rMaxDepth = rchild.maxDepth
		rRegCount = rchild.regCount
	}

	n := &Node{
		lChild:   lchild,
		rChild:   rchild,
		height:   height,
		path:     nil,
		payload:  nil,
		maxDepth: utils.MaxUint16(lMaxDepth, rMaxDepth) + uint16(1),
		regCount: lRegCount + rRegCount,
	}
	n.hashValue = n.computeHash()
	return n
}

// computeHash computes the hashValue for the given Node
// we kept it this way to stay compatible with the previous versions
func (n *Node) computeHash() []byte {
	if n.lChild == nil && n.rChild == nil {
		// both ROOT NODE and LEAF NODE have n.lChild == n.rChild == nil
		if n.payload != nil {
			// LEAF node: defined by key-value pair
			return common.ComputeCompactValue(n.path, n.payload, n.height)
		}
		// ROOT NODE: no children, no key-value pair
		return common.GetDefaultHashForHeight(n.height)
	}

	// this is an INTERIOR node at least one of lChild or rChild is not nil.
	var h1, h2 []byte
	if n.lChild != nil {
		h1 = n.lChild.Hash()
	} else {
		h1 = common.GetDefaultHashForHeight(n.height - 1)
	}

	if n.rChild != nil {
		h2 = n.rChild.Hash()
	} else {
		h2 = common.GetDefaultHashForHeight(n.height - 1)
	}
	return common.HashInterNode(h1, h2)
}

func (n *Node) VerifyCachedHash() bool {
	if n.lChild != nil {
		if !n.lChild.VerifyCachedHash() {
			return false
		}
	}
	if n.rChild != nil {
		if !n.rChild.VerifyCachedHash() {
			return false
		}
	}
	if n.hashValue != nil {
		return bytes.Equal(n.hashValue, n.computeHash())
	}
	return true
}

// Hash returns the Node's hash value.
// Do NOT MODIFY returned slice!
func (n *Node) Hash() []byte { return n.hashValue }

// Height returns the Node's height.
// Per definition, the height of a node v in a tree is the number
// of edges on the longest downward path between v and a tree leaf.
func (n *Node) Height() int { return n.height }

// MaxDepth returns the longest path from this node to compacted leafs in the subtree.
// in contrast to the Height, this value captures compactness of the subtrie.
func (n *Node) MaxDepth() uint16 { return n.maxDepth }

// RegCount returns number of registers allocated in the subtrie of this node.
func (n *Node) RegCount() uint64 { return n.regCount }

// Path returns the the Node's register storage path.
func (n *Node) Path() ledger.Path { return n.path }

// Payload returns the the Node's payload.
// only leaf nodes have children.
// Do NOT MODIFY returned slices!
func (n *Node) Payload() *ledger.Payload { return n.payload }

// LeftChild returns the the Node's left child.
// Only INTERIOR nodes have children.
// Do NOT MODIFY returned Node!
func (n *Node) LeftChild() *Node { return n.lChild }

// RightChild returns the the Node's right child.
// Only INTERIOR nodes have children.
// Do NOT MODIFY returned Node!
func (n *Node) RightChild() *Node { return n.rChild }

// IsLeaf returns true if and only if Node is a LEAF.
func (n *Node) IsLeaf() bool {
	// Per definition, a node is a leaf if and only if it has defined path
	return n.path != nil && len(n.path) > 0
	// shouldn't n.path != nil be enough?
}

// FmtStr provides formatted string representation of the Node and sub tree
func (n *Node) FmtStr(prefix string, subpath string) string {
	right := ""
	if n.rChild != nil {
		right = fmt.Sprintf("\n%v", n.rChild.FmtStr(prefix+"\t", subpath+"1"))
	}
	left := ""
	if n.lChild != nil {
		left = fmt.Sprintf("\n%v", n.lChild.FmtStr(prefix+"\t", subpath+"0"))
	}
	payloadSize := 0
	if n.payload != nil {
		payloadSize = n.payload.Size()
	}
	hashStr := hex.EncodeToString(n.hashValue)
	hashStr = hashStr[:3] + "..." + hashStr[len(hashStr)-3:]
	return fmt.Sprintf("%v%v: (path:%v, payloadSize:%d hash:%v)[%s] (obj %p) %v %v ", prefix, n.height, n.path, payloadSize, hashStr, subpath, n, left, right)
}

// AllPayloads returns the payload of this node and all payloads of the subtrie
// UNCHECKED : n != nil
func (n *Node) AllPayloads() []ledger.Payload {
	payloads := make([]ledger.Payload, 0)
	recursiveAllPayload(&payloads, n)
	return payloads
}

func recursiveAllPayload(payloads *[]ledger.Payload, n *Node) {
	if n == nil { // TODO : this check is not needed if all tests are using a full tree (either 0 or 2 children)
		return
	}
	if n.IsLeaf() {
		*payloads = append(*payloads, *n.Payload())
		return
	}
	recursiveAllPayload(payloads, n.lChild)
	recursiveAllPayload(payloads, n.rChild)
}
