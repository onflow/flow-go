// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package merkle

import (
	"github.com/onflow/flow-go/crypto/hash"
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

// nodeTag encodes the type of node when hashing it. Required for cryptographic
// safety to prevent malleability attacks (replacing a full node with a short node
// that has identical hash would otherwise be possible).
// Values are intended to be global constants and must be unique.
var (
	leafNodeTag  = [1]byte{uint8(0)}
	fullNodeTag  = [1]byte{uint8(1)}
	shortNodeTag = [1]byte{uint8(2)}
)

/* ******************************* Short Node ******************************* */
// Per convention a Full node has _always_ one child, which is either
// a full node or a leaf.

type short struct {
	path  []byte // holds the common path to the next node
	count uint   // holds the count of bits in the path
	child node   // holds the child after the common path; never nil
}

func (n short) Hash() []byte {
	h, _ := blake2b.New256(shortNodeTag[:])
	_, _ = h.Write([]byte{byte(n.count)})
	_, _ = h.Write(n.path)
	_, _ = h.Write(n.child.Hash())
	return h.Sum(nil)
}

/* ******************************** Full Node ******************************* */
// Per convention a Full node has _always_ two children. Nil values not allowed.

type full struct {
	left  node // holds the left path node (bit 0); never nil
	right node // holds the right path node (bit 1); never nil
}

func (n full) Hash() []byte {
	h, _ := blake2b.New256(fullNodeTag[:])
	_, _ = h.Write(n.left.Hash())
	_, _ = h.Write(n.right.Hash())
	return h.Sum(nil)
}

/* ******************************** Leaf Node ******************************* */
// Holds a key-value pair. We only hash the value, because the key is committed
// to by the merkle path through the trie.

type leaf struct {
	key hash.Hash // copy of key
	val []byte    // the concrete data we actually stored
}

func (n leaf) Hash() []byte {
	h, _ := blake2b.New256(leafNodeTag[:])
	_, _ = h.Write(n.val)
	return h.Sum(nil)
}

/* ******************************** Dummy Node ******************************* */
// 4th dummy node type as substitute for `nil`. Not used in the trie but as zero
// value for auxiliary variables during trie update operations. This reduces
// complexity of the business logic, as we can then also apply the convention of
// "nodes are never nil" to the auxiliary variables.

type dummy struct{}

func (n dummy) Hash() []byte {
	// Per convention, Hash should never be called by the business logic but
	// is required to implement the node iterface
	panic("dummy node has no hash")
}
