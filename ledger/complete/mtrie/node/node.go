package node

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
)

// Node defines an Mtrie node
//
// DEFINITIONS:
//   - HEIGHT of a node v in a tree is the number of edges on the longest
//     downward path between v and a tree leaf.
//
// Conceptually, an MTrie is a sparse Merkle Trie, which has two node types:
//   - INTERIM node: has at least one child (i.e. lChild or rChild is not
//     nil). Interim nodes do not store a path and have no payload.
//   - LEAF node: has _no_ children.
//
// Per convention, we also consider nil as a leaf. Formally, nil is the generic
// representative for any empty (sub)-trie (i.e. a trie without allocated
// registers).
//
// Nodes are supposed to be treated as _immutable_ data structures.
// TODO: optimized data structures might be able to reduce memory consumption
type Node struct {
	// Implementation Comments:
	// Formally, a tree can hold up to 2^maxDepth number of registers. However,
	// the current implementation is designed to operate on a sparsely populated
	// tree, holding much less than 2^64 registers.

	lChild       *Node     // Left Child
	rChild       *Node     // Right Child
	height       int       // height where the Node is at
	hashValue    hash.Hash // hash value of node (cached)
	leafNodeHash hash.Hash // hash value of the fully expanded leaf node (leaf nodes only)
}

// NewNode creates a new Node.
// UNCHECKED requirement: combination of values must conform to
// a valid node type (see documentation of `Node` for details)
func NewNode(height int,
	lchild,
	rchild *Node,
	hashValue hash.Hash,
	leafNodeHash hash.Hash,
) *Node {
	n := &Node{
		lChild:       lchild,
		rChild:       rchild,
		height:       height,
		hashValue:    hashValue,
		leafNodeHash: leafNodeHash,
	}
	return n
}

// NewLeaf creates a compact leaf Node.
// UNCHECKED requirement: height must be non-negative
// UNCHECKED requirement: payload is non nil
// UNCHECKED requirement: payload should be deep copied if received from external sources
func NewLeaf(path ledger.Path,
	payload *ledger.Payload,
	height int,
) *Node {
	leafNodeHash := computeLeafNodeHash(path, payload, 0)

	hashValue := leafNodeHash
	if height > 0 {
		// TODO compute hashValue using the leafNodeHash, save 1 hash operation
		hashValue = computeLeafNodeHash(path, payload, height)
	}

	return &Node{
		lChild:       nil,
		rChild:       nil,
		height:       height,
		hashValue:    hashValue,
		leafNodeHash: leafNodeHash,
	}
}

func NewLeafWithHash(path ledger.Path, payload *ledger.Payload, height int, hashValue hash.Hash) *Node {
	leafNodeHash := hashValue
	if height > 0 {
		leafNodeHash = computeLeafNodeHash(path, payload, 0)
	}

	return &Node{
		lChild:       nil,
		rChild:       nil,
		height:       height,
		hashValue:    hashValue,
		leafNodeHash: leafNodeHash,
	}
}

func computeLeafNodeHash(path ledger.Path, payload *ledger.Payload, height int) hash.Hash {
	return ledger.ComputeCompactValue(hash.Hash(path), payload.Value(), height)
}

// computeInterimNodeHash takes two children and the height of the current node, and returns
// its hash.
// note: the height can not be inferred from lChild.height + 1, because both lChild and rChild
// could be nil
func computeInterimNodeHash(lChild, rChild *Node, height int) hash.Hash {
	h1 := hashOrDefault(lChild, height-1)
	h2 := hashOrDefault(rChild, height-1)
	return hash.HashInterNode(h1, h2)
}

func hashOrDefault(node *Node, height int) hash.Hash {
	if node == nil {
		return ledger.GetDefaultHashForHeight(height)
	}
	return node.Hash()
}

// NewInterimNode creates a new interim Node.
// UNCHECKED requirement:
//   - for any child `c` that is non-nil, its height must satisfy: height = c.height + 1
func NewInterimNode(height int, lchild, rchild *Node) *Node {
	hashValue := computeInterimNodeHash(lchild, rchild, height)
	return NewInterimNodeWithHash(height, lchild, rchild, hashValue)
}

func NewInterimNodeWithHash(height int, lchild, rchild *Node, hashValue hash.Hash) *Node {
	return &Node{
		lChild:       lchild,
		rChild:       rchild,
		height:       height,
		hashValue:    hashValue,
		leafNodeHash: hash.DummyHash, // interim node has no leaf node hash
	}
}

// NewInterimCompactifiedNode creates a new compactified interim Node. For compactification,
// we only consider the immediate children. When starting with a maximally pruned trie and
// creating only InterimCompactifiedNodes during an update, the resulting trie remains maximally
// pruned. Details on compactification:
//   - If _both_ immediate children represent completely unallocated sub-tries, then the sub-trie
//     with the new interim node is also completely empty. We return nil.
//   - If either child is a leaf (i.e. representing a single allocated register) _and_ the other
//     child represents a completely unallocated sub-trie, the new interim node also only holds
//     a single allocated register. In this case, we return a compactified leaf.
//
// UNCHECKED requirement:
//   - for any child `c` that is non-nil, its height must satisfy: height = c.height + 1
func NewInterimCompactifiedNode(height int, lChild, rChild *Node) *Node {
	if lChild.IsDefaultNode() {
		lChild = nil
	}
	if rChild.IsDefaultNode() {
		rChild = nil
	}

	// CASE (a): _both_ children do _not_ contain any allocated registers:
	if lChild == nil && rChild == nil {
		return nil // return nil representing as completely empty sub-trie
	}

	// CASE (b): one child is a compactified leaf (single allocated register) _and_ the other child represents
	// an empty subtrie => in total we have one allocated register, which we represent as single leaf node
	if rChild == nil && lChild.IsLeaf() {
		h := hash.HashInterNode(lChild.hashValue, ledger.GetDefaultHashForHeight(lChild.height))
		return &Node{lChild: nil, rChild: nil, height: height, hashValue: h, leafNodeHash: lChild.leafNodeHash}
	}
	if lChild == nil && rChild.IsLeaf() {
		h := hash.HashInterNode(ledger.GetDefaultHashForHeight(rChild.height), rChild.hashValue)
		return &Node{lChild: nil, rChild: nil, height: height, hashValue: h, leafNodeHash: rChild.leafNodeHash}
	}

	// CASE (b): both children contain some allocated registers => we can't compactify; return a full interim leaf
	return NewInterimNode(height, lChild, rChild)
}

// IsDefaultNode returns true iff the sub-trie represented by this root node contains
// only unallocated registers. This is the case, if the node is nil or the node's hash
// is equal to the default hash value at the respective height.
func (n *Node) IsDefaultNode() bool {
	if n == nil {
		return true
	}
	return n.hashValue == ledger.GetDefaultHashForHeight(n.height)
}

// VerifyCachedHash verifies the hash of a node is valid
func verifyCachedHashRecursive(n *Node) bool {
	if n == nil {
		return true
	}
	// since we no longer store payload on leaf node, its hash
	// can't be verified, we assume the hash is correct
	if n.IsLeaf() {
		return true
	}
	if !verifyCachedHashRecursive(n.lChild) || !verifyCachedHashRecursive(n.rChild) {
		return false
	}

	computedHash := computeInterimNodeHash(n.lChild, n.rChild, n.height)
	return n.hashValue == computedHash
}

// VerifyCachedHash verifies the hash of a node is valid
func (n *Node) VerifyCachedHash() bool {
	return verifyCachedHashRecursive(n)
}

// Hash returns the Node's hash value.
// Do NOT MODIFY returned slice!
func (n *Node) Hash() hash.Hash {
	return n.hashValue
}

// ExpandedLeafHash returns the hash of the fully expanded leaf node
func (n *Node) ExpandedLeafHash() hash.Hash {
	return n.leafNodeHash
}

// Height returns the Node's height.
// Per definition, the height of a node v in a tree is the number
// of edges on the longest downward path between v and a tree leaf.
func (n *Node) Height() int {
	return n.height
}

// LeftChild returns the the Node's left child.
// Only INTERIM nodes have children.
// Do NOT MODIFY returned Node!
func (n *Node) LeftChild() *Node { return n.lChild }

// RightChild returns the the Node's right child.
// Only INTERIM nodes have children.
// Do NOT MODIFY returned Node!
func (n *Node) RightChild() *Node { return n.rChild }

// IsLeaf returns true if and only if Node is a LEAF.
func (n *Node) IsLeaf() bool {
	// Per definition, a node is a leaf if and only it has no children
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
	hashStr := hex.EncodeToString(n.hashValue[:])
	hashStr = hashStr[:3] + "..." + hashStr[len(hashStr)-3:]
	return fmt.Sprintf("%v%v: (hash:%v)[%s] (obj %p) %v %v ", prefix, n.height, hashStr, subpath, n, left, right)
}

// AllLeafNodes returns all subtree leaf nodes from this subtrie
func (n *Node) AllLeafNodes() []*Node {
	return n.appendSubtreeLeafNodes([]*Node{})
}

// appendSubtreeLeafNodes recursively append the subtree leaf nodes to the
// provided result slice.
func (n *Node) appendSubtreeLeafNodes(result []*Node) []*Node {
	if n == nil {
		return result
	}

	if n.IsLeaf() {
		return append(result, n)
	}
	result = n.lChild.appendSubtreeLeafNodes(result)
	result = n.rChild.appendSubtreeLeafNodes(result)
	return result
}
