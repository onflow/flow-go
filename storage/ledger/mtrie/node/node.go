package node

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/common"
)

// Node of a RamSafe MTrie.
//
// DEFINITIONS:
//     * HEIGHT of a node v in a tree is the number of edges on the longest
//       downward path between v and a tree leaf.
//
// Conceptually, an MTrie is a sparse Merkle Trie, which has three different types of nodes:
//    * LEAF node: fully defined by key-value pair and a height
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
	lChild    *Node // Left Child
	rChild    *Node // Right Child
	height    int   // height where the Node is at
	key       []byte
	value     []byte
	hashValue []byte
}

// NewLeaf creates a compact leaf Node
func NewEmptyTreeRoot(height int) *Node {
	n := &Node{
		lChild: nil,
		rChild: nil,
		height: height,
		key:    nil,
		value:  nil,
	}
	n.hashValue = n.computeNodeHash()
	return n
}

// NewLeaf creates a compact leaf Node
func NewLeaf(key, value []byte, height int) *Node {
	n := &Node{
		lChild: nil,
		rChild: nil,
		height: height,
		key:    key,
		value:  value,
	}
	n.hashValue = n.computeNodeHash()
	return n
}

// newNode creates a new Node with the provided value and no children
func NewInterimNode(height int, lchild, rchild *Node) *Node {
	n := &Node{
		lChild: lchild,
		rChild: rchild,
		height: height,
		key:    nil,
		value:  nil,
	}
	n.hashValue = n.computeNodeHash()
	return n
}

// computeNodeHash computes the hashValue for the given Node
// if forced it set it won't trust hash Values of children and
// recomputes it.
func (n *Node) computeNodeHash() []byte {
	if n.lChild == nil && n.rChild == nil {
		// both ROOT NODE and LEAF NODE have n.lChild == n.rChild == nil
		if len(n.value) > 0 {
			// LEAF node: defined by key-value pair
			return common.ComputeCompactValue(n.key, n.value, n.height)
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

// Height returns the Node's hash value.
// Do NOT MODIFY returned slice!
func (n *Node) Hash() []byte { return n.hashValue }

// Height returns the Node's height.
// Per definition, the height of a node v in a tree is the number
// of edges on the longest downward path between v and a tree leaf.
func (n *Node) Height() int { return n.height }

// Key returns the the Node's register key.
// The present node is a LEAF node, if and only if the returned key is NOT NULL.
// Do NOT MODIFY returned slices!
func (n *Node) Key() []byte { return n.key }

// Value returns the the Node's register values.
// The present node is a LEAF node, if and only if the returned value is NOT NULL.
// Do NOT MODIFY returned slices!
func (n *Node) Value() []byte { return n.value }

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
	// Per definition, a node is a leaf if and only if it has defined by key-value pair
	return n.key != nil
}

// FmtStr provides formatted string representation of the Node and sub tree
func (n Node) FmtStr(prefix string, path string) string {
	right := ""
	if n.rChild != nil {
		right = fmt.Sprintf("\n%v", n.rChild.FmtStr(prefix+"\t", path+"1"))
	}
	left := ""
	if n.lChild != nil {
		left = fmt.Sprintf("\n%v", n.lChild.FmtStr(prefix+"\t", path+"0"))
	}
	return fmt.Sprintf("%v%v: (k:%v, v:%v, h:%v)[%s] %v %v ", prefix, n.height, n.key, hex.EncodeToString(n.value), hex.EncodeToString(n.hashValue), path, left, right)
}

// DeepCopy returns a deep copy of the Node (including deep copy of children)
func (n *Node) DeepCopy() *Node {
	newNode := &Node{height: n.height}

	if n.value != nil {
		value := make([]byte, len(n.value))
		copy(value, n.value)
		newNode.value = value
	}
	if n.key != nil {
		key := make([]byte, len(n.key))
		copy(key, n.key)
		newNode.key = key
	}
	if n.hashValue != nil {
		h := make([]byte, len(n.hashValue))
		copy(h, n.key)
		newNode.key = h
	}
	if n.lChild != nil {
		newNode.lChild = n.lChild.DeepCopy()
	}
	if n.rChild != nil {
		newNode.rChild = n.rChild.DeepCopy()
	}
	if n.rChild != nil {
		newNode.rChild = n.rChild.DeepCopy()
	}
	return newNode
}

// Equals compares two nodes and all subsequent children
// this is an expensive call and should only be used
// for limited cases (e.g. testing)
func (n *Node) Equals(o *Node) bool {

	// height don't match
	if n.height != o.height {
		return false
	}

	// Values don't match
	if (n.value == nil) != (o.value == nil) {
		return false
	}
	if n.value != nil && o.value != nil && !bytes.Equal(n.value, o.value) {
		return false
	}

	// keys don't match
	if (n.key == nil) != (o.key == nil) {
		return false
	}
	if n.key != nil && o.key != nil && !bytes.Equal(n.key, o.key) {
		return false
	}

	// hashValues don't match
	if (n.hashValue == nil) != (o.hashValue == nil) {
		return false
	}
	if n.hashValue != nil && o.hashValue != nil && !bytes.Equal(n.hashValue, o.hashValue) {
		return false
	}

	// left children don't match
	if (n.lChild == nil) != (o.lChild == nil) {
		return false
	}
	if n.lChild != nil && o.lChild != nil && !n.lChild.Equals(o.lChild) {
		return false
	}

	// right children don't match
	if (n.rChild == nil) != (o.rChild == nil) {
		return false
	}
	if n.rChild != nil && o.rChild != nil && !n.rChild.Equals(o.rChild) {
		return false
	}

	return true
}
