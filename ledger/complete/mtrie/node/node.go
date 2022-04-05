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
//
// Implementation Comments:
// Formally, a tree can hold up to 2^maxDepth number of registers. However,
// the current implementation is designed to operate on a sparsely populated
// tree, holding much less than 2^64 registers.
type Node interface {
	// Height returns the Node's height.
	// Per definition, the height of a node v in a tree is the number
	// of edges on the longest downward path between v and a tree leaf.
	Height() int

	// Hash returns the Node's hash value.
	Hash() hash.Hash

	// Path returns a pointer to the Node's register storage path.
	// Do NOT MODIFY returned path!
	Path() *ledger.Path

	// Payload returns the Node's payload.
	// Do NOT MODIFY returned payload!
	Payload() *ledger.Payload

	// LeftChild returns the Node's left child.
	// Do NOT MODIFY returned Node!
	LeftChild() Node

	// RightChild returns the Node's right child.
	// Do NOT MODIFY returned Node!
	RightChild() Node
}

type LeafNode struct {
	height    int             // height where the Node is at
	hashValue hash.Hash       // hash value of node (cached)
	path      ledger.Path     // the storage path
	payload   *ledger.Payload // the payload this node is storing
}

var _ Node = (*LeafNode)(nil)

type InterimNode struct {
	height    int       // height where the Node is at
	hashValue hash.Hash // hash value of node (cached)
	lChild    Node      // Left Child
	rChild    Node      // Right Child
}

var _ Node = (*InterimNode)(nil)

// NewLeafNodeWithHash creates a new LeafNode with pre-computed hash.
// UNCHECKED requirement: height must be non-negative
// UNCHECKED requirement: payload is non nil
// NOTE: payload should be deep copied if received from external sources
func NewLeafNodeWithHash(path ledger.Path, payload *ledger.Payload, height int, hashValue hash.Hash) *LeafNode {
	return &LeafNode{
		height:    height,
		path:      path,
		payload:   payload,
		hashValue: hashValue,
	}
}

// NewLeafNode creates a compact leaf Node.
// UNCHECKED requirement: height must be non-negative
// UNCHECKED requirement: payload is non nil
// NOTE: payload should be deep copied if received from external sources
func NewLeafNode(path ledger.Path, payload *ledger.Payload, height int) *LeafNode {
	n := &LeafNode{
		height:  height,
		path:    path,
		payload: payload,
	}
	n.hashValue = n.computeHash()
	return n
}

// computeHash returns the hashValue of the leaf node
func (n *LeafNode) computeHash() hash.Hash {
	// if payload is non-nil, compute the hash based on the payload content
	if n.payload != nil {
		return ledger.ComputeCompactValue(hash.Hash(n.path), n.payload.Value, n.height)
	}
	// if payload is nil, return the default hash
	return ledger.GetDefaultHashForHeight(n.height)
}

// Height returns the Node's height.
// Per definition, the height of a node v in a tree is the number
// of edges on the longest downward path between v and a tree leaf.
func (n *LeafNode) Height() int {
	return n.height
}

// Hash returns the Node's hash value.
func (n *LeafNode) Hash() hash.Hash {
	return n.hashValue
}

// Path returns a pointer to the Node's register storage path.
// Do NOT MODIFY returned path!
func (n *LeafNode) Path() *ledger.Path {
	return &n.path
}

// Payload returns the Node's payload.
// Do NOT MODIFY returned payload!
func (n *LeafNode) Payload() *ledger.Payload {
	return n.payload
}

// LeftChild returns nil for LeafNode.
func (n *LeafNode) LeftChild() Node {
	return nil
}

// RightChild returns nil for LeafNode.
func (n *LeafNode) RightChild() Node {
	return nil
}

// NewInterimNodeWithHash creates a new InterimNode with pre-computed hash.
// UNCHECKED requirement:
//  * at least one child is not nil
//  * for any child that is non-nil, its height must satisfy: height = c.height + 1
func NewInterimNodeWithHash(height int, lchild, rchild Node, hashValue hash.Hash) *InterimNode {
	return &InterimNode{
		lChild:    lchild,
		rChild:    rchild,
		height:    height,
		hashValue: hashValue,
	}
}

// NewInterimNode creates a new interim Node.
// UNCHECKED requirement:
//  * at least one child is not nil
//  * for any child `c` that is non-nil, its height must satisfy: height = c.height + 1
func NewInterimNode(height int, lchild, rchild Node) *InterimNode {
	n := &InterimNode{
		lChild: lchild,
		rChild: rchild,
		height: height,
	}
	n.hashValue = n.computeHash()
	return n
}

// NewInterimCompactifiedNode creates a new compactified interim Node. For compactification,
// we only consider the immediate children. When starting with a maximally pruned trie and
// creating only InterimCompactifiedNodes during an update, the resulting trie remains maximally
// pruned. Details on compactification:
//  * If _both_ immediate children represent completely unallocated sub-tries, then the sub-trie
//    with the new interim node is also completely empty. We return nil.
//  * If either child is a leaf (i.e. representing a single allocated register) _and_ the other
//    child represents a completely unallocated sub-trie, the new interim node also only holds
//    a single allocated register. In this case, we return a compactified leaf.
// UNCHECKED requirement:
//  * for any child `c` that is non-nil, its height must satisfy: height = c.height + 1
func NewInterimCompactifiedNode(height int, lChild, rChild Node) Node {
	if IsDefaultNode(lChild) {
		lChild = nil
	}
	if IsDefaultNode(rChild) {
		rChild = nil
	}

	// CASE (a): _both_ children do _not_ contain any allocated registers:
	if lChild == nil && rChild == nil {
		return nil // return nil representing as completely empty sub-trie
	}

	// CASE (b): one child is a compactified leaf (single allocated register) _and_ the other child represents
	// an empty subtrie => in total we have one allocated register, which we represent as single leaf node
	if rChild == nil {
		if leftLeaf, ok := lChild.(*LeafNode); ok {
			h := hash.HashInterNode(leftLeaf.hashValue, ledger.GetDefaultHashForHeight(leftLeaf.height))
			return NewLeafNodeWithHash(leftLeaf.path, leftLeaf.payload, height, h)
		}
	}
	if lChild == nil {
		if rightLeaf, ok := rChild.(*LeafNode); ok {
			h := hash.HashInterNode(ledger.GetDefaultHashForHeight(rightLeaf.height), rightLeaf.hashValue)
			return NewLeafNodeWithHash(rightLeaf.path, rightLeaf.payload, height, h)
		}
	}

	// CASE (b): both children contain some allocated registers => we can't compactify; return a full interim leaf
	return NewInterimNode(height, lChild, rChild)
}

// computeHash returns the hashValue of the interim node.
func (n *InterimNode) computeHash() hash.Hash {
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

// Height returns the Node's height.
// Per definition, the height of a node v in a tree is the number
// of edges on the longest downward path between v and a tree leaf.
func (n *InterimNode) Height() int {
	return n.height
}

// Hash returns the Node's hash value.
func (n *InterimNode) Hash() hash.Hash {
	return n.hashValue
}

// Path returns nil for InterimNode.
func (n *InterimNode) Path() *ledger.Path {
	return nil
}

// Payload returns nil for InterimNode.
func (n *InterimNode) Payload() *ledger.Payload {
	return nil
}

// LeftChild returns the Node's left child.
// Do NOT MODIFY returned Node!
func (n *InterimNode) LeftChild() Node {
	return n.lChild
}

// RightChild returns the Node's right child.
// Do NOT MODIFY returned Node!
func (n *InterimNode) RightChild() Node {
	return n.rChild
}

// VerifyCachedHash verifies the Node's cached hash.
func VerifyCachedHash(n Node) error {
	if n == nil {
		return nil
	}
	switch nd := n.(type) {
	case *LeafNode:
		computedHash := nd.computeHash()
		if nd.hashValue == computedHash {
			return nil
		}
		return fmt.Errorf("invalid hash in LeafNode %+v", nd)
	case *InterimNode:
		if err := VerifyCachedHash(nd.lChild); err != nil {
			return err
		}
		if err := VerifyCachedHash(nd.rChild); err != nil {
			return err
		}
		computedHash := nd.computeHash()
		if nd.hashValue == computedHash {
			return nil
		}
		return fmt.Errorf("invalid hash in InterimNode %+v", nd)
	}
	return fmt.Errorf("invalid node type %T", n)
}

// IsLeaf returns true if and only if Node is a LEAF.
func IsLeaf(n Node) bool {
	// Per definition, a node is a leaf if and only it has no children
	if n == nil {
		return true
	}
	_, ok := n.(*LeafNode)
	return ok
}

// FmtStr provides formatted string representation of the Node and sub tree
func FmtStr(n Node) string {
	return fmtStr(n, "", "")
}

func fmtStr(n Node, prefix string, subpath string) string {
	right := ""
	if rChild := n.RightChild(); rChild != nil {
		right = fmt.Sprintf("\n%v", fmtStr(rChild, prefix+"\t", subpath+"1"))
	}
	left := ""
	if lChild := n.LeftChild(); lChild != nil {
		left = fmt.Sprintf("\n%v", fmtStr(lChild, prefix+"\t", subpath+"0"))
	}
	payloadSize := 0
	if n.Payload() != nil {
		payloadSize = n.Payload().Size()
	}
	hashValue := n.Hash()
	hashStr := hex.EncodeToString(hashValue[:])
	hashStr = hashStr[:3] + "..." + hashStr[len(hashStr)-3:]
	return fmt.Sprintf("%v%v: (path:%v, payloadSize:%d hash:%v)[%s] (obj %p) %v %v ", prefix, n.Height(), n.Path(), payloadSize, hashStr, subpath, n, left, right)
}

// AllPayloads appends the payloads of the subtree with this node as root
// to the provided payloads slice. Follows same pattern as Go's native append method.
func AllPayloads(n Node, payloads []ledger.Payload) []ledger.Payload {
	if n == nil {
		return payloads
	}
	if IsLeaf(n) {
		return append(payloads, *n.Payload())
	}
	payloads = AllPayloads(n.LeftChild(), payloads)
	payloads = AllPayloads(n.RightChild(), payloads)
	return payloads
}

// IsDefaultNode returns true if the sub-trie represented by this root node contains
// only unallocated registers. This is the case, if the node is nil or the node's hash
// is equal to the default hash value at the respective height.
func IsDefaultNode(n Node) bool {
	if n == nil {
		return true
	}
	return n.Hash() == ledger.GetDefaultHashForHeight(n.Height())
}
