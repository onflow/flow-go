package merkle

import (
	"bytes"

	"github.com/onflow/flow-go/ledger/common/bitutils"
)

// Proof caputres all data needed for inclusion proof of a single value inserted under key `Key` into the merkle trie
type Proof struct {
	// key that is used to insert and look up the value
	Key []byte
	// value stored in the trie for the given key
	Value []byte
	// size of this is equal to the steps needed for verification,
	// constructed by traversing top to down, if set to zero means we had a full node and a value from a hash value from InterimHashes should be read,
	// any other value than 0 means we hit a short node and we need the shortcounts for computing the hash
	ShortCounts []int
	// a slice of hash values (hash value of siblings for full nodes, the size of this is equal to the zeros we have in ShortCounts)
	InterimHashes [][]byte
}

// Verify verifies the proof by constructing the hash values bottom up and cross check
// the constructed root hash with the given one.
// if the proof is valid it returns true and false otherwise
func (p *Proof) Verify(expectedRootHash []byte) bool {
	// iterate backward and verify the proof
	currentHash := computeLeafHash(p.Value)
	hashIndex := len(p.InterimHashes) - 1

	// compute last path index
	pathIndex := len(p.InterimHashes)
	for _, sc := range p.ShortCounts {
		pathIndex += sc
	}

	// for each step (level from bottom to top) check if its a full node or a short node and compute the
	// hash value accordingly; for full node having the sibling hash helps to compute the hash value
	// of the next level, for short nodes compute the hash using the common path constructed based on
	// the given short count
	for i := len(p.ShortCounts) - 1; i >= 0; i-- {
		shortCounts := p.ShortCounts[i]

		//// its a full node
		if shortCounts == 0 {
			// read and pop the sibling hash value
			sibling := p.InterimHashes[hashIndex]
			hashIndex--

			// decrement the path index by 1
			pathIndex--

			// based on the bit at pathIndex of the key compute the hash
			if bitutils.ReadBit(p.Key, pathIndex) == 0 { // left branching
				currentHash = computeFullHash(currentHash, sibling)
				continue
			}
			currentHash = computeFullHash(sibling, currentHash) // right branching
			continue
		}

		//// its a short node
		// construct the common path
		commonPath := bitutils.MakeBitVector(shortCounts)
		pathIndex = pathIndex - shortCounts
		for j := 0; j < shortCounts; j++ {
			if bitutils.ReadBit(p.Key, pathIndex+j) == 1 {
				bitutils.SetBit(commonPath, j)
			}
		}
		// compute the hash for the short node
		currentHash = computeShortHash(shortCounts, commonPath, currentHash)
	}

	// in the end we should have used all the path space available and
	// the final hash value should match whith what was expected
	if pathIndex != 0 || !bytes.Equal(currentHash, expectedRootHash) {
		return false
	}

	return true
}
