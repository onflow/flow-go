package merkle

import (
	"bytes"

	"github.com/onflow/flow-go/ledger/common/bitutils"
)

// Proof caputres all data needed for inclusion proof of a single value inserted under key `Key`
type Proof struct {
	// key that is used to insert and look up the value
	Key []byte
	// value stored in the trie for the given key
	Value []byte
	// size of this is equal to the steps needed for verification,
	// constructed by traversing top to down, if set to zero means we had a full node and a value from a hash value from InterimHashes should be read,
	// any other value than 0 means we hit a short node and we need the shortcounts for computing the hash
	ShortCounts []uint8
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
		pathIndex += int(sc)
	}

	// for each step check if is full node or short node and compute the
	// hash value accordingly
	for i := len(p.ShortCounts) - 1; i >= 0; i-- {
		shortCounts := p.ShortCounts[i]
		if shortCounts == 0 { // is full node
			neighbour := p.InterimHashes[hashIndex]
			hashIndex--
			pathIndex--
			// based on the bit on pathIndex, compute the hash
			if bitutils.ReadBit(p.Key, pathIndex) == 1 {
				currentHash = computeFullHash(neighbour, currentHash)
			} else {
				currentHash = computeFullHash(currentHash, neighbour)
			}
			continue
		}
		// else its a short node
		// construct the common path
		commonPath := bitutils.MakeBitVector(int(shortCounts))
		pathIndex = pathIndex - int(shortCounts)
		for j := 0; j < int(shortCounts); j++ {
			if bitutils.ReadBit(p.Key, pathIndex+j) == 1 {
				bitutils.SetBit(commonPath, j)
			}
		}

		currentHash = computeShortHash(int(shortCounts), commonPath, currentHash)
	}

	if pathIndex != 0 || !bytes.Equal(currentHash, expectedRootHash) {
		return false
	}
	return true
}
