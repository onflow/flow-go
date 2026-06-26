package payloadless

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
)

// Node defines a payloadless Mtrie node.
//
// Unlike the regular mtrie Node which stores full payloads, a payloadless Node
// stores only the leaf hash (HashLeaf(path, value)) for leaf nodes. This enables
// significant memory savings while preserving the same root hash as a full trie.
//
// DEFINITIONS:
//   - HEIGHT of a node v in a tree is the number of edges on the longest
//     downward path between v and a tree leaf.
//
// Conceptually, an MTrie is a sparse Merkle Trie, which has two node types:
//   - INTERIM node: has at least one child (i.e. lChild or rChild is not
//     nil). Interim nodes do not store a path and have no leafHash.
//   - LEAF node: has _no_ children. Stores a path and (optionally) a leafHash.
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

	lChild    *Node       // Left Child
	rChild    *Node       // Right Child
	height    int         // height where the Node is at
	path      ledger.Path // the storage path (dummy value for interim nodes)
	leafHash  *hash.Hash  // HashLeaf(path, value) - the height-0 leaf hash (leaf nodes only; nil for unallocated registers)
	hashValue hash.Hash   // hash value of node (cached)
}

// NewNode creates a new Node.
// UNCHECKED requirement: combination of values must conform to
// a valid node type (see documentation of `Node` for details)
func NewNode(height int,
	lchild,
	rchild *Node,
	path ledger.Path,
	leafHash *hash.Hash,
	hashValue hash.Hash,
) *Node {
	n := &Node{
		lChild:    lchild,
		rChild:    rchild,
		height:    height,
		path:      path,
		leafHash:  leafHash,
		hashValue: hashValue,
	}
	return n
}

// NewLeaf creates a leaf Node from a path and the original payload value.
// The leafHash is computed as HashLeaf(path, value), and the node hash is
// computed using the original value to ensure the same root hash as a full trie.
//
// UNCHECKED requirement: height must be non-negative
func NewLeaf(path ledger.Path, value []byte, height int) *Node {
	// For empty values, create a default node
	if len(value) == 0 {
		return &Node{
			height:    height,
			path:      path,
			leafHash:  nil,
			hashValue: ledger.GetDefaultHashForHeight(height),
		}
	}

	// Compute the leaf hash (height-0)
	leafHash := hash.HashLeaf(hash.Hash(path), value)

	return NewLeafWithHash(path, leafHash, height)
}

// NewLeafWithHash creates a leaf Node from a pre-computed leaf hash.
// This is used when converting from a full trie or loading from a payloadless checkpoint.
//
// The nodeHash is computed by extending the leafHash (height-0) to the specified height.
//
// UNCHECKED requirement: height must be non-negative
// UNCHECKED requirement: leafHash must be HashLeaf(path, originalValue)
func NewLeafWithHash(path ledger.Path, leafHash hash.Hash, height int) *Node {
	// Compute the node hash by extending the leaf hash to the target height
	nodeHash := ledger.ComputeCompactValueFromLeafHash(hash.Hash(path), leafHash, height)

	return &Node{
		height:    height,
		path:      path,
		leafHash:  &leafHash,
		hashValue: nodeHash,
	}
}

// NewInterimNode creates a new interim Node.
// UNCHECKED requirement:
//   - for any child `c` that is non-nil, its height must satisfy: height = c.height + 1
func NewInterimNode(height int, lChild, rChild *Node) *Node {
	n := &Node{
		lChild: lChild,
		rChild: rChild,
		height: height,
	}
	n.hashValue = n.computeHash()
	return n
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
		return &Node{height: height, path: lChild.path, leafHash: lChild.leafHash, hashValue: h}
	}
	if lChild == nil && rChild.IsLeaf() {
		h := hash.HashInterNode(ledger.GetDefaultHashForHeight(rChild.height), rChild.hashValue)
		return &Node{height: height, path: rChild.path, leafHash: rChild.leafHash, hashValue: h}
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

// computeHash returns the hashValue of the node
func (n *Node) computeHash() hash.Hash {
	// check for leaf node
	if n.lChild == nil && n.rChild == nil {
		// if leafHash is non-nil, extend the height-0 leaf hash to the node's height
		if n.leafHash != nil {
			return ledger.ComputeCompactValueFromLeafHash(hash.Hash(n.path), *n.leafHash, n.height)
		}
		// if leafHash is nil, return the default hash
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
	if !verifyCachedHashRecursive(n.lChild) || !verifyCachedHashRecursive(n.rChild) {
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
	return n.hashValue
}

// Height returns the Node's height.
// Per definition, the height of a node v in a tree is the number
// of edges on the longest downward path between v and a tree leaf.
func (n *Node) Height() int {
	return n.height
}

// Path returns a pointer to the Node's register storage path.
// If the node is not a leaf, the function returns `nil`.
func (n *Node) Path() *ledger.Path {
	if n.IsLeaf() {
		return &n.path
	}
	return nil
}

// LeafHash returns the Node's leaf hash HashLeaf(path, value).
// Returns nil for interim nodes and for leaves that represent unallocated registers.
// Do NOT MODIFY returned hash!
func (n *Node) LeafHash() *hash.Hash {
	return n.leafHash
}

// LeftChild returns the Node's left child.
// Only INTERIM nodes have children.
// Do NOT MODIFY returned Node!
func (n *Node) LeftChild() *Node { return n.lChild }

// RightChild returns the Node's right child.
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
	leafHashStr := "nil"
	if n.leafHash != nil {
		leafHashStr = hex.EncodeToString(n.leafHash[:])[:6] + "..."
	}
	hashStr := hex.EncodeToString(n.hashValue[:])
	hashStr = hashStr[:3] + "..." + hashStr[len(hashStr)-3:]
	return fmt.Sprintf("%v%v: (path:%v, leafHash:%s, hash:%v)[%s] (obj %p) %v %v",
		prefix, n.height, n.path, leafHashStr, hashStr, subpath, n, left, right)
}

// AllLeafHashes returns the leaf hash of this node and all leaf hashes of the subtrie.
// Empty leaves (unallocated registers) are skipped.
func (n *Node) AllLeafHashes() []*hash.Hash {
	return n.appendSubtreeLeafHashes([]*hash.Hash{})
}

// appendSubtreeLeafHashes appends the leaf hashes of the subtree with this node as root
// to the provided slice. Follows same pattern as Go's native append method.
// Empty leaves (unallocated registers) are skipped.
func (n *Node) appendSubtreeLeafHashes(result []*hash.Hash) []*hash.Hash {
	if n == nil {
		return result
	}
	if n.IsLeaf() {
		if n.leafHash != nil {
			return append(result, n.leafHash)
		}
		return result
	}
	result = n.lChild.appendSubtreeLeafHashes(result)
	result = n.rChild.appendSubtreeLeafHashes(result)
	return result
}
