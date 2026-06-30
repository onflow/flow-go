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
// stores only the leaf hash for leaf nodes. This enables significant memory savings
// while preserving the same root hash as a full trie.
//
// DEFINITIONS:
//   - HEIGHT of a node v in a tree is the number of edges on the longest downward
//     path between v and a tree leaf (in hypothetical, fully expanded (perfect)
//     tree, i.e. without compactification or nil-pruning).
//
// Conceptually, an MTrie is a sparse Merkle Trie, which has two node types:
//   - INTERIM node: has at least one child (i.e. lChild or rChild is not
//     nil). Interim nodes do not store a path and have no leafHash.
//   - LEAF node: has _no_ children. Stores a path and (optionally) a leafHash.
//     Nodes at height 0 are always leafs: we call those uncompactified leafs.
//     Leafs might have positive height: this is an optimization, which may be
//     applied (on best effort basis), where we represent a leaf that has a trailing
//     path segment leading to it *without branches* as a single data structure.
//     We call those compactified leafs. Compactification is optional and leaves the
//     root hash unchanged.
//
// Per convention, we also consider nil as a leaf. Formally, nil is the generic
// representative for any empty (sub)-trie (i.e. a trie without allocated
// registers).
//
// Nodes are supposed to be treated as _immutable_ data structures.
// TODO: optimized data structures might be able to reduce memory consumption
type Node struct {
	// Implementation Comments:
	// Formally, a tree of height 𝒽 can hold up to 2^𝒽 number of registers. However,
	// the current implementation is designed to operate on a sparsely populated
	// tree, holding much less than 2^64 registers.

	height int         // height where the Node is at (root node has largest height of 256)
	lChild *Node       // Left Child (nil for all leaves, including compactified leaves of height > 0)
	rChild *Node       // Right Child (nil for all leaves, including compactified leaves of height > 0)
	path   ledger.Path // the storage path (dummy value for interim nodes)

	// leafHash is nil for all non-leaf nodes. For leafs (node with both children nil), leafHash
	// is nil if and only if the leaf represents an unallocated register (value is nil or empty).
	// Formally:
	//              ╭ hash(path, value)    if len(value) > 0
	//   leafHash = ┥
	//              ╰ nil                  otherwise
	leafHash *hash.Hash

	// hashValue is the cached hash of this node (always set for every node). By construction,
	// a node's hashValue equals the hash that the equivalent node would have in the fully-expanded
	// (perfect) tree; compactification and nil-pruning are chosen precisely to preserve this
	// value (see mtrie/README.md for details), which is why they leave the root hash unchanged.
	//
	// We specify hashValue per node *type* (independent of compactification or nil-pruning). Here
	// DefaultHashForHeight(𝒽) denotes the hash of a subtree with height 𝒽 holding only unallocated
	// registers (defined recursively in mtrie/README.md). DefaultHashForHeight is independent of
	// the sub-trie's location in the overall trie (identical for all empty subtrees of height 𝒽).
	//
	//  • LEAF node (both children nil) at height 𝒽, for register (path, value):
	//                  ╭ subtree-root hash for {(path, value)}   if len(value) > 0   (allocated)
	//      hashValue = ┥
	//                  ╰ DefaultHashForHeight(𝒽)                 if len(value) == 0  (unallocated)
	//    where the `subtree-root hash for {(path, value)}` is the hash of the height-𝒽 subtree that
	//    holds only this single register: start from the height-0 leaf with hash(path, value) and
	//    hash upward 𝒽 levels, at each level combining with the default hash of the (empty) sibling
	//    subtree. For an uncompactified leaf (𝒽 == 0) this is just hash(path, value).
	//
	//  • INTERIM node (at least one non-nil child) at height 𝒽 > 0:
	//      hashValue = hash( lChild.hashValue , rChild.hashValue )
	//    where a nil child contributes DefaultHashForHeight(𝒽-1) (the hash of the empty
	//    subtree of height 𝒽-1 it represents).
	//
	// Notes:
	//  • For an allocated register, the height-0 leaf's hashValue equals its leafHash by design.
	//  • hashValue == DefaultHashForHeight(𝒽)
	//      ⟺ the sub-tree rooted at this node holds only unallocated registers
	//      ⟺ every node in that sub-tree has leafHash == nil
	//    (the forward direction relies on our requirement of a collision-resistant hash function).
	hashValue hash.Hash
}

// NewNode creates a new Node.
// CAUTION: INSECURE! Only intended to reconstruct Nodes from their serialization!
// UNCHECKED requirement: combination of values must conform to a valid node type (see
// documentation of `Node` for details)
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

// NewLeaf constructs the leaf Node 𝓃 representing the single register (path, value) at the
// given height 𝒽. For 𝒽 = 0, 𝓃 is an ordinary leaf; for 𝒽 > 0, 𝓃 is a compactified leaf
// standing in for an otherwise-empty height-𝒽 subtree that holds only this register. By
// construction, the 𝓃.Hash() of leaf 𝓃 equals the hash that the height-𝒽 subtree
// containing the register (path, value) would have in the fully-expanded trie. In other
// words, tries assembled from these constructors share the same root hash as a fully-expanded
// trie (see the Node type and mtrie/README.md for details).
//
// A `nil` or empty `value` denotes an unallocated register; the result is a compactified leaf
// representing an empty subtree of the given height, whose hash is DefaultHashForHeight(height)
// independent of path.
//
// UNCHECKED requirement: height must be non-negative.
func NewLeaf(path ledger.Path, value []byte, height int) *Node {
	if len(value) == 0 { // For empty values, create a default node
		newDefaultLeaf(path, height)
	}

	// Leaf represent an allocated register:
	leafHash := hash.HashLeaf(hash.Hash(path), value) // we pre-compute leaf hash at height-0 here
	return NewLeafWithHash(path, leafHash, height)    // handles compactification up to given height if necessary
}

// NewDefaultLeaf constructs the default node, which represents an unallocated register (`nil` or empty value)
// compactifed to a trie node at the given height. Its leafHash is nil and its hash is DefaultHashForHeight(height).
//
// Note that the hash of an empty subtree (at any height) is technically independent of path. However, subtries
// containing only unallocated registers should be replaced by nil in a compactified trie. Creating explicitly a
// default node is only useful if the caller wants to explicitly represent represent a specific register that is
// not yet allocated. Doing so is useful as an interim simplification until we have non-inclusion proofs as a
// special case implemented (at the moment, non-inclusion proofs fall back on inclusion proofs of explicitly
// represented default nodes).
//
// UNCHECKED requirement: height must be non-negative.
func newDefaultLeaf(path ledger.Path, height int) *Node {
	return &Node{
		height:    height,
		path:      path,
		leafHash:  nil,
		hashValue: ledger.GetDefaultHashForHeight(height),
	}
}

// NewRelevelledLeaf creates a new compactified leaf 𝓁' for the same register (path, value) as the input
// leaf 𝓁, re-levelled to height `relevellingHeight`. This is needed when a register r is allocated or
// removed in the neighbourhood of 𝓁, changing the height at which 𝓁 can be compactified. Example:
//
//	     trie without r                 trie with r
//
//	        parent                        parent
//	       ╱    ╲                         ╱    ╲
//	      𝓁      △         ◀──▶          ◯      △        height 𝒽
//	      ┊                            ╱  ╲
//	      ┊                           𝓁'   𝓃             height 𝒽-1
//	      ┊                           ┊
//	      •                           •                  height 0 (fully-expanded perfect trie)
//
//	parent : genuine branch at height 𝒽+1; its other child △ is a non-empty sibling subtree
//	         (the reason 𝓁 sits at height 𝒽, not higher). Unchanged by allocating r.
//	◯      : interim node materialized at 𝓁's former position ( height-𝒽 ).
//	𝓁, 𝓁'  : the SAME register (path, value); 𝓁' is 𝓁 re-levelled to a lower height 𝒽' (𝒽-1 shown).
//	𝓃      : leaf of register r.
//	•      : the register's actual leaf at height 0 in the fully-expanded (perfect) trie.
//	dotted : single-child perfect-trie path (┊) that compactification collapses into one node.
//
// The nodeHash is computed by extending the leafHash (height-0) to the specified height.
//
// UNCHECKED requirement: `leaf.IsLeaf()` must be true
func NewRelevelledLeaf(leaf *Node, relevellingHeight int) *Node {
	if leaf.leafHash == nil { // leaf.leafHash is nil ⟺ unallocaled register ⟺ node is a default leaf
		return newDefaultLeaf(leaf.path, relevellingHeight)
	}

	// Leaf represent an allocated register:
	return NewLeafWithHash(leaf.path, *leaf.leafHash, relevellingHeight) // handles compactification up to given relevellingHeight if necessary
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

// NewInterimCompactifiedNode creates a new interim Node - compactified if possible. For compactification,
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
