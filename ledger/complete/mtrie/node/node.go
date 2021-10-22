package node

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/utils"
)

// Node defines an Mtrie node
//
// DEFINITIONS:
//   * HEIGHT of a node v in a tree is the number of edges on the longest
//     downward path between v and a tree leaf.
//
// Conceptually, an MTrie is a sparse Merkle Trie, which has two node types:
//   * INTERIM node: has at least one child (i.e. lChild or rChild is not
//     nil). Interim nodes do not store a path and have no payload.
//   * LEAF node: has _no_ children.
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

	lChild    *Node           // Left Child
	rChild    *Node           // Right Child
	height    int             // height where the Node is at
	path      ledger.Path     // the storage path (dummy value for interim nodes)
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
		maxDepth: utils.MaxUint16(lMaxDepth, rMaxDepth) + 1,
		regCount: lRegCount + rRegCount,
	}
	n.computeAndStoreHash()
	return n
}

// deepCopy
func (n *Node) deepCopy() *Node {
	var h hash.Hash
	copy(h[:], n.hashValue[:])

	var p ledger.Path
	copy(p[:], n.path[:])

	var lChildCopy, rChildCopy *Node
	if n.lChild != nil {
		lChildCopy = n.lChild.deepCopy()
	}
	if n.rChild != nil {
		rChildCopy = n.rChild.deepCopy()
	}

	return &Node{
		lChild:    lChildCopy,
		rChild:    rChildCopy,
		height:    n.height,
		path:      p,
		payload:   n.payload.DeepCopy(),
		maxDepth:  n.maxDepth,
		regCount:  n.regCount,
		hashValue: h,
	}
}

// computeAndStoreHash computes the node's hash value and
// stores the result in the nodes internal `hashValue` field
func (n *Node) computeAndStoreHash() {
	n.hashValue = n.computeHash()
}

// Pruned would look at the child of a node and if it can be pruned
// it constructs a new subtrie and return that one instead of n
// otherwise it would return the node itself if no change is required
func (n *Node) Pruned() (*Node, bool) {
	if n.IsLeaf() {
		return n, false
	}

	// if leaf return it as if
	// if non leaf bubble up
	lChildEmpty := true
	rChildEmpty := true
	var lChildChanged, rChildChanged bool
	if n.lChild != nil {
		lChildEmpty = n.lChild.IsADefaultNode()
	}
	if n.rChild != nil {
		rChildEmpty = n.rChild.IsADefaultNode()
	}
	if rChildEmpty && lChildEmpty {
		// is like a leaf
		return NewLeaf(ledger.Path{}, ledger.EmptyPayload(), n.height), true
	}

	// if childNode is non empty
	if !lChildEmpty && rChildEmpty && n.lChild.IsLeaf() {
		// TODO merge these two methods
		l := n.lChild.deepCopy()
		l.hashValue = hash.HashInterNode(l.hashValue, ledger.GetDefaultHashForHeight(l.height))
		l.height = l.height + 1

		// bubble up all children and return as parent
		return l, true
	}
	if lChildEmpty && !rChildEmpty && n.rChild.IsLeaf() {
		// TODO merge these two methods
		r := n.rChild.deepCopy()
		r.hashValue = hash.HashInterNode(ledger.GetDefaultHashForHeight(r.height), r.hashValue)
		r.height = r.height + 1
		// bubble up all children and return as parent
		return r, true
	}

	if lChildChanged || rChildChanged {
		return NewInterimNode(n.height, n.lChild, n.rChild), true
	}
	// else no change needed
	return n, false
}

func (n *Node) IsADefaultNode() bool {
	// TODO make this optimize by caching if the node is a default node
	defaultHashValue := ledger.GetDefaultHashForHeight(n.height)
	return bytes.Equal(n.hashValue[:], defaultHashValue[:])
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
	return n.hashValue
}

// Height returns the Node's height.
// Per definition, the height of a node v in a tree is the number
// of edges on the longest downward path between v and a tree leaf.
func (n *Node) Height() int {
	return n.height
}

// MaxDepth returns the longest path from this node to compacted leafs in the subtree.
// in contrast to the Height, this value captures compactness of the subtrie.
func (n *Node) MaxDepth() uint16 {
	return n.maxDepth
}

// RegCount returns number of registers allocated in the subtrie of this node.
func (n *Node) RegCount() uint64 {
	return n.regCount
}

// Path returns a pointer to the Node's register storage path.
// If the node is not a leaf, the function returns `nil`.
func (n *Node) Path() *ledger.Path {
	if n.IsLeaf() {
		return &n.path
	}
	return nil
}

// Payload returns the the Node's payload.
// Do NOT MODIFY returned slices!
func (n *Node) Payload() *ledger.Payload {
	return n.payload
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
