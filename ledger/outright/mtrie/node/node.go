package node

import (
	"encoding/hex"
	"fmt"

	"github.com/dapperlabs/flow-go/ledger"
	"github.com/dapperlabs/flow-go/ledger/outright/mtrie/common"
)

// TODO RAMTIN update this documentation
// Node defines a memory forest node
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
	lChild    *Node          // Left Child
	rChild    *Node          // Right Child
	height    int            // height where the Node is at
	path      ledger.Path    // the storage path (leaf nodes only)
	payload   ledger.Payload // the payload this node is storing (leaf nodes only)
	hashValue []byte         // hash value of node (cached)
	maxDepth  uint16         // captures the longest path from this node to compacted leafs in the subtree
	regCount  uint64         // number of registers allocated in the subtree
}

// NewNode creates a new Node.
// UNCHECKED requirement: combination of values must conform to
// a valid node type (see documentation of `Node` for details)
func NewNode(height int,
	lchild,
	rchild *Node,
	path ledger.Path,
	payload ledger.Payload,
	hashValue []byte,
	maxDepth uint16,
	regCount uint64) *Node {
	n := &Node{
		lChild:    lchild,
		rChild:    rchild,
		height:    height,
		path:      path,
		payload:   payload,
		hashValue: hashValue,
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
	payload ledger.Payload,
	height int) *Node {

	regCount := uint64(0)
	if path != nil {
		regCount = uint64(1)
	}

	n := &Node{
		lChild:   nil,
		rChild:   nil,
		height:   height,
		path:     path,
		payload:  payload,
		maxDepth: 0,
		regCount: regCount,
	}
	n.hashValue = n.computeHash()
	return n
}

// NewInterimNode creates a new Node with the provided value and no children.
// UNCHECKED requirement: lchild.height and rchild.height must be smaller than height
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
		maxDepth: common.MaxUint16(lMaxDepth, rMaxDepth) + uint16(1),
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
			return common.ComputeCompactValue(n.path, n.payload.Encode(), n.height)
		}
		// ROOT NODE: no children, no key-value pair
		return common.GetDefaultHashForHeight(n.height)
	}

	// this is an INTERIOR node at least one of lChild or rChild is not nil.
	h1 := common.GetDefaultHashForHeight(n.height - 1)
	if n.lChild != nil {
		h1 = n.lChild.Hash()
	}
	h2 := common.GetDefaultHashForHeight(n.height - 1)
	if n.rChild != nil {
		h2 = n.rChild.Hash()
	}
	return common.HashInterNode(h1, h2)
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
func (n *Node) Payload() ledger.Payload { return n.payload }

// LeftChild returns the the Node's left child.
// Only INTERIOR nodes have children.
// Do NOT MODIFY returned Node!
func (n *Node) LeftChild() *Node { return n.lChild }

// RigthChild returns the the Node's right child.
// Only INTERIOR nodes have children.
// Do NOT MODIFY returned Node!
func (n *Node) RigthChild() *Node { return n.rChild }

// IsLeaf returns true if and only if Node is a LEAF.
func (n *Node) IsLeaf() bool {
	// Per definition, a node is a leaf if and only if it has defined path
	return n.path != nil
}

// FmtStr provides formatted string representation of the Node and sub tree
// TODO switch the subpath to local one
func (n Node) FmtStr(prefix string, subpath string) string {
	right := ""
	if n.rChild != nil {
		right = fmt.Sprintf("\n%v", n.rChild.FmtStr(prefix+"\t", subpath+"1"))
	}
	left := ""
	if n.lChild != nil {
		left = fmt.Sprintf("\n%v", n.lChild.FmtStr(prefix+"\t", subpath+"0"))
	}
	return fmt.Sprintf("%v%v: (path:%v, phash:%v)[%s] %v %v ", prefix, n.height, n.path, hex.EncodeToString(n.hashValue), subpath, left, right)
}
