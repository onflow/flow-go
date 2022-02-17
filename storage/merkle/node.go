// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package merkle

import (
	"golang.org/x/crypto/blake2b"
)

// The node represents a generic vertex in the sparse Merkle trie.
// CONVENTION:
//  * This implementation guarantees that within the trie there are
//    _never_ nil-references to descendants. We completely avoid this by
//    utilizing `short nodes` that represent any path-segment without branching.
type node interface {
	// Hash computes recursively the hash of this respective sub trie.
	// To simplify enforcing cryptographic security, we introduce the convention
	// that hashing a nil node is an illegal operation, which panics.
	Hash() []byte
}

// NodeTags encodes the type of node when hashing it. Required for cryptographic
// safety to prevent collision attacks (replacing a full node with a short node
// that has identical hash would otherwise be possible).
// Values are intended to be global constants and must be unique.
var (
	leafNodeTag  = []byte{0}
	fullNodeTag  = []byte{1}
	shortNodeTag = []byte{2}
)

// Short Node
// Per convention a Short node has always _one child_, which is either
// a full node or a leaf.

type short struct {
	count int    // holds the count of bits in the path
	path  []byte // holds the common path to the next node
	child node   // holds the child after the common path; never nil
}

var _ node = &short{}

func computeShortHash(count int, path []byte, childHash []byte) []byte {
	c := serializedPathSegmentLength(count)
	h, _ := blake2b.New256(shortNodeTag) // blake2b.New256(..) never errors for given MAC (verified in tests)
	_, _ = h.Write(c[:])                 // blake2b.Write(..) never errors for _any_ input
	_, _ = h.Write(path)                 // blake2b.Write(..) never errors for _any_ input
	_, _ = h.Write(childHash)            // blake2b.Write(..) never errors for _any_ input
	return h.Sum(nil)
}

func (n *short) Hash() []byte {
	return computeShortHash(n.count, n.path, n.child.Hash())
}

// serializedPathSegmentLength serializes the bitCount into two bytes.
// We are able to represent key length of up to 65528 bits
func serializedPathSegmentLength(bitCount int) [2]byte {
	var byteCount [2]byte
	byteCount[0] = byte(bitCount >> 8)
	byteCount[1] = byte(bitCount)
	return byteCount
}

//Full Node
// Per convention a Full Node has always _two children_. Nil values not allowed.

type full struct {
	left  node // holds the left path node (bit 0); never nil
	right node // holds the right path node (bit 1); never nil
}

var _ node = &full{}

func computeFullHash(leftChildHash, rightChildHash []byte) []byte {
	h, _ := blake2b.New256(fullNodeTag) // blake2b.New256(..) never errors for given MAC (verified in tests)
	_, _ = h.Write(leftChildHash)       // blake2b.Write(..) never errors for _any_ input
	_, _ = h.Write(rightChildHash)      // blake2b.Write(..) never errors for _any_ input
	return h.Sum(nil)
}

func (n *full) Hash() []byte {
	return computeFullHash(n.left.Hash(), n.right.Hash())
}

// Leaf Node
// Leaf represents a key-value pair. We only store the value, because the
// key is implicitly stored as the merkle path through the tree.

type leaf struct {
	val []byte
}

var _ node = &leaf{}

func computeLeafHash(value []byte) []byte {
	h, _ := blake2b.New256(leafNodeTag) // blake2b.New256(..) never errors for given MAC (verified in tests)
	_, _ = h.Write(value)               // blake2b.Write(..) never errors for _any_ input
	return h.Sum(nil)
}

func (n *leaf) Hash() []byte {
	return computeLeafHash(n.val)
}

// Dummy Node
// Dummy node type as substitute for `nil`. Not used in the trie, but as zero
// value for auxiliary variables during trie update operations. This reduces
// complexity of the business logic, as we can then also apply the convention of
// "nodes are never nil" to the auxiliary variables.

type dummy struct{}

var _ node = &dummy{}

func (n *dummy) Hash() []byte {
	// Per convention, Hash should never be called by the business logic but
	// is required to implement the node interface
	panic("dummy node has no hash")
}
