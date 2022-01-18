package merkle

import (
	"bytes"

	"github.com/onflow/flow-go/ledger/common/bitutils"
)

// Proof captures all data needed for inclusion proof of a single value inserted under key `Key` into the merkle trie
type Proof struct {
	// key that is used to insert and look up the value
	Key []byte
	// value stored in the trie for the given key
	Value []byte

	// InterimNodeTypes encodes the type of each visited node on the path. We use the following convention:
	//  * Let `t := InterimNodeTypes[i]`. Index `i` represents node along the path with the distance
	//    `i` (number of edges) from the trie root.
	//  * The value `t` represents the node's type:
	//    • `t == 0` indicates that the vertex is a full node
	//    • `t == k`, for any `k > 0`, indicates that vertex is a short node `s`, with `s.count == k`
	//
	// TODO: as this would include many zeros and would be sparse if the keys are computed as output of hashes, we might optimize
	// this by using an sparse representation in the future (index and counts for non-zeros only)
	InterimNodeTypes []uint32

	// SiblingHashes hold the hash of the non-visited sibling node for each branch on the path.
	// Elements are ordered from root to leaf. As branches are represented as full nodes, the
	// size of this slice must be equal to the zeros we have in InterimNodeTypes.
	SiblingHashes [][]byte
}

// Verify verifies the proof by constructing the hash values bottom up and cross check
// the constructed root hash with the given one.
// if the proof is valid it returns true and false otherwise
func (p *Proof) Verify(expectedRootHash []byte) bool {
	// before we verify any hashes, check correct formatting of the proof
	if len(p.Key) < 1 || maxKeyLength < len(p.Key) {
		return false
	}
	expectedKeyLength := uint32(0)
	expectedNumberSiblingHashes := 0
	for _, t := range p.InterimNodeTypes {
		if t == 0 { // full node
			expectedKeyLength += 1
			expectedNumberSiblingHashes += 1
		} else { // short node
			expectedKeyLength += t
		}
		if maxKeyLength < expectedKeyLength { // preventing overflow attack
			return false
		}
	}
	if expectedKeyLength != uint32(len(p.Key)) || expectedNumberSiblingHashes != len(p.SiblingHashes) {
		return false
	}

	// start with the leaf and reconstruct the root hash bottom-up
	currentHash := computeLeafHash(p.Value)
	nextSiblingHashIdx := len(p.SiblingHashes) - 1
	nextPathIdx := len(p.Key) - 1

	// Iterating over vertices along path from bottom up: determine node type and compute
	// hash value accordingly; for full node having the sibling hash helps to compute the hash value
	// of the next level; for short nodes compute the hash using the common path constructed based on
	// the given short count
	for i := len(p.InterimNodeTypes) - 1; i >= 0; i-- {
		t := p.InterimNodeTypes[i]

		// FULL NODE
		if t == 0 {
			// read and pop the sibling hash value
			sibling := p.SiblingHashes[nextSiblingHashIdx]
			nextSiblingHashIdx--

			// based on the path's next bit, compute the hash
			if bitutils.ReadBit(p.Key, nextPathIdx) == 0 { // left branching
				currentHash = computeFullHash(currentHash, sibling)
			} else {
				currentHash = computeFullHash(sibling, currentHash) // right branching
			}
			nextPathIdx--
			continue
		}

		// SHORT NODE
		// construct the common path
		count := int(t)
		commonPath := bitutils.MakeBitVector(count)
		for c := count - 1; c >= 0; c-- {
			if bitutils.ReadBit(p.Key, nextPathIdx) == 1 {
				bitutils.SetBit(commonPath, c)
			}
			nextPathIdx--
		}
		// compute the hash for the short node
		currentHash = computeShortHash(count, commonPath, currentHash)
	}

	// Now we have reconstructed the hash of the root node, which we expect to be equal to the `expectedRootHash`
	return bytes.Equal(currentHash, expectedRootHash)
}
