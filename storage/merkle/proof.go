package merkle

import (
	"bytes"

	"github.com/onflow/flow-go/ledger/common/bitutils"
	"golang.org/x/crypto/blake2b"
)

type Proof struct {
	Key           []byte   // key
	HashValue     []byte   // hash of the Value
	ShortCounts   []uint8  // if set to one means full node, else means short node
	InterimHashes [][]byte // hash values
}

// Verify returns if the proof is valid and false otherwise
func (p *Proof) Verify(expectedRootHash []byte) bool {
	// iterate backward and verify the proof
	currentHash := p.HashValue
	hashIndex := len(p.InterimHashes) - 1

	// compute last path index
	pathIndex := len(p.InterimHashes)
	for _, sc := range p.ShortCounts {
		pathIndex += int(sc)
	}

	for i := len(p.ShortCounts) - 1; i >= 0; i-- {
		shortCounts := p.ShortCounts[i]
		if shortCounts == 0 { // is full node
			neighbour := p.InterimHashes[hashIndex]
			hashIndex--
			pathIndex--
			h, _ := blake2b.New256(fullNodeTag) // blake2b.New256(..) error for given MAC (verified in tests)
			// based on the bit on pathIndex, compute the hash
			if bitutils.ReadBit(p.Key, pathIndex) == 1 {
				_, _ = h.Write(neighbour)
				_, _ = h.Write(currentHash)
				currentHash = h.Sum(nil)
			} else {
				_, _ = h.Write(currentHash)
				_, _ = h.Write(neighbour)
				currentHash = h.Sum(nil)
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

		h, _ := blake2b.New256(shortNodeTag) // blake2b.New256(..) error for given MAC (verified in tests)
		c := serializedPathSegmentLength(int(shortCounts))
		_, _ = h.Write(c[:])        // blake2b.Write(..) never errors for _any_ input
		_, _ = h.Write(commonPath)  // blake2b.Write(..) never errors for _any_ input
		_, _ = h.Write(currentHash) // blake2b.Write(..) never errors for _any_ input
		currentHash = h.Sum(nil)
	}

	if pathIndex != 0 || !bytes.Equal(currentHash, expectedRootHash) {
		return false
	}
	return true
}
