package node

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
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
// Currently, we represent both data structures by Node instances
//
// Nodes are supposed to be used in READ-ONLY fashion. However,
// for performance reasons, we not not copy read.
// TODO: optimized data structures might be able to reduce memory consumption
type Node struct {
	// Implementation Comments:
	// Formally, a tree can hold up to 2^maxDepth number of registers. However,
	// the current implementation is designed to operate on a sparsely populated
	// tree, holding much less than 2^64 registers.

	lChild    *Node           // Left Child
	rChild    *Node           // Right Child
	height    int             // height where the Node is at
	path      ledger.Path     // the storage path (leaf nodes only)
	payload   *ledger.Payload // the payload this node is storing (leaf nodes only)
	hashValue hash.Hash       // hash value of node (cached)
	// TODO : Atm, we don't support trees with dynamic depth.
	//        Instead, this should be a forest-wide constant
	maxDepth uint16 // captures the longest path from this node to compacted leafs in the subtree
	// TODO : migrate to book-keeping only in the tree root.
	//        Update can just return the _change_ of regCount.
	regCount uint64 // number of registers allocated in the subtree
}

// NewNode creates a new Node.
// UNCHECKED requirement: combination of values must conform to
// a valid node type (see documentation of `Node` for details)
func NewNode(height int,
	lchild,
	rchild *Node,
	path ledger.Path,
	payload *ledger.Payload,
	hashValue hash.Hash,
	maxDepth uint16,
	regCount uint64) *Node {

	var pl *ledger.Payload
	if payload != nil {
		pl = payload.DeepCopy()
	}

	n := &Node{
		lChild:    lchild,
		rChild:    rchild,
		height:    height,
		path:      path,
		hashValue: hashValue,
		payload:   pl,
		maxDepth:  maxDepth,
		regCount:  regCount,
	}
	return n
}

// NewLeaf creates a compact leaf Node.
// UNCHECKED requirement: height must be non-negative
// UNCHECKED requirement: payload is non nil
func NewLeaf(path ledger.Path,
	payload *ledger.Payload,
	height int) *Node {

	n := &Node{
		lChild:   nil,
		rChild:   nil,
		height:   height,
		path:     path,
		payload:  payload.DeepCopy(),
		maxDepth: 0,
		regCount: uint64(1),
	}
	n.computeAndStoreHash()
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
		payload:  nil,
		maxDepth: utils.MaxUint16(lMaxDepth, rMaxDepth) + uint16(1),
		regCount: lRegCount + rRegCount,
	}
	n.computeAndStoreHash()
	return n
}

// computeAndStoreHash computes the node's hash value and
// stores the result in the nodes internal `hashValue` field
func (n *Node) computeAndStoreHash() {
	n.hashValue = n.computeHash()
}

// computeHash returns the hashValue of the node
func (n *Node) computeHash() hash.Hash {
	// check for leaf node
	if n.lChild == nil && n.rChild == nil {
		// if payload is non-nil, compute the hash based on the payload content
		if n.payload != nil {
			return ledger.ComputeCompactValue(hash.Hash(n.path), n.payload.Value, n.height)
		}
		// if payload is nil, return the default hash
		return ledger.GetDefaultHashForHeight(n.height)
	}

	// this is an interim node at least one of lChild or rChild is not nil.
	var h1, h2 hash.Hash
	if n.lChild != nil {
		h1 = n.lChild.Hash()
	} else {
		h1 = ledger.GetDefaultHashForHeight(n.height - 1)
	}

	if n.rChild != nil {
		h2 = n.rChild.Hash()
	} else {
		h2 = ledger.GetDefaultHashForHeight(n.height - 1)
	}
	return hash.HashInterNode(h1, h2)
}

// VerifyCachedHash verifies the hash of a node is valid
func verifyCachedHashRecursive(n *Node) bool {
	if n == nil {
		return true
	}
	if !verifyCachedHashRecursive(n.lChild) {
		return false
	}

	if !verifyCachedHashRecursive(n.rChild) {
		return false
	}

	computedHash := n.computeHash()
	return n.hashValue == computedHash
}

// VerifyCachedHash verifies the hash of a node is valid
func (n *Node) VerifyCachedHash() bool {
	return verifyCachedHashRecursive(n)
}

// Hash returns the Node's hash value.
// Do NOT MODIFY returned slice!
func (n *Node) Hash() hash.Hash {
	if n == nil {
		// case of an empty trie root
		return ledger.GetDefaultHashForHeight(ledger.NodeMaxHeight)
	}
	return n.hashValue
}

// Height returns the Node's height.
// Per definition, the height of a node v in a tree is the number
// of edges on the longest downward path between v and a tree leaf.
func (n *Node) Height() int {
	if n == nil {
		return ledger.NodeMaxHeight
	}
	return n.height
}

// MaxDepth returns the longest path from this node to compacted leafs in the subtree.
// in contrast to the Height, this value captures compactness of the subtrie.
func (n *Node) MaxDepth() uint16 {
	if n == nil {
		return 0
	}
	return n.maxDepth
}

// RegCount returns number of registers allocated in the subtrie of this node.
func (n *Node) RegCount() uint64 {
	if n == nil {
		return 0
	}
	return n.regCount
}

// Path returns the the Node's register storage path.
//
// the path of an nil node is defined as the arbitrary value 00..00, although
// no node or trie algorithm is relying on this value.
func (n *Node) Path() ledger.Path {
	if n == nil {
		return ledger.DummyPath
	}
	return n.path
}

// Payload returns the the Node's payload.
// Do NOT MODIFY returned slices!
func (n *Node) Payload() *ledger.Payload {
	if n == nil {
		return nil
	}
	return n.payload
}

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
	return n == nil || (n.lChild == nil && n.rChild == nil)
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
	hashStr := hex.EncodeToString(n.hashValue[:])
	hashStr = hashStr[:3] + "..." + hashStr[len(hashStr)-3:]
	return fmt.Sprintf("%v%v: (path:%v, payloadSize:%d hash:%v)[%s] (obj %p) %v %v ", prefix, n.height, n.path, payloadSize, hashStr, subpath, n, left, right)
}

// AllPayloads returns the payload of this node and all payloads of the subtrie
func (n *Node) AllPayloads() []ledger.Payload {
	return n.appendSubtreePayloads([]ledger.Payload{})
}

// appendSubtreePayloads appends the payloads of the subtree with this node as root
// to the provided Payload slice. Follows same pattern as Go's native append method.
func (n *Node) appendSubtreePayloads(result []ledger.Payload) []ledger.Payload {
	if n == nil {
		return result
	}
	if n.IsLeaf() {
		return append(result, *n.Payload())
	}
	result = n.lChild.appendSubtreePayloads(result)
	result = n.rChild.appendSubtreePayloads(result)
	return result
}
