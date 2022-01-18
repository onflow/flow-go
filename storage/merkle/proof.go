package merkle

import (
	"bytes"
	"fmt"

	"github.com/onflow/flow-go/ledger/common/bitutils"
)

// Proof caputres all data needed for inclusion proof of a single value inserted under key `Key` into the merkle trie
type Proof struct {
	// Key used to insert and look up the value
	Key []byte
	// Value stored in the trie for the given key
	Value []byte
	// IsAShortNode captures a way to determine short nodes versus full nodes while tranversing the
	// trie downward. if the bit is set to 1, it means that we have reached to a short node, and
	// if is set to 0 means we have reached a full node.
	IsAShortNode []byte
	// SkipBits is read when we reach a short node, and the value represents number of bits that were skipped
	// by the short node (shortNode.count)
	SkipBits []uint8
	// InterimHashes is a slice of hash values, every value is read when we reach a full node (hash value of the siblings)
	InterimHashes [][]byte
}

// Verify verifies the proof by constructing the hash values bottom up and cross check
// the constructed root hash with the given one.
// if the proof is valid it returns true and false otherwise
func (p *Proof) Verify(expectedRootHash []byte) (bool, error) {
	// iterate backward and verify the proof
	currentHash := computeLeafHash(p.Value)

	// an index to consume interim hashes from the last element to the first element
	interimHashIndex := len(p.InterimHashes) - 1
	skipBitIndex := len(p.SkipBits) - 1

	// compute steps
	steps := len(p.SkipBits) + len(p.InterimHashes)

	if steps > len(p.IsAShortNode)*8 {
		return false, fmt.Errorf("malformed proof, IsShortNode length doesnt match the size of skipbits and interimhashes")
	}

	// key index keeps track of last location of the key that was checked.
	keyIndex := len(p.InterimHashes)
	for _, sc := range p.SkipBits {
		keyIndex += int(sc)
	}

	// for each step (level from bottom to top) check if its a full node or a short node and compute the
	// hash value accordingly; for full node having the sibling hash helps to compute the hash value
	// of the next level, for short nodes compute the hash using the common path constructed based on
	// the given short count
	for i := steps - 1; i >= 0; i-- {

		// its a full node
		if bitutils.ReadBit(p.IsAShortNode, i) == 0 {
			// read and pop the sibling hash value from InterimHashes
			if interimHashIndex == 0 {
				return false, fmt.Errorf("malformed proof, no more InterimHashes available to read")
			}
			sibling := p.InterimHashes[interimHashIndex]
			interimHashIndex--

			// decrement the keyindex by 1
			keyIndex--

			// based on the bit at pathIndex of the key compute the hash
			if bitutils.ReadBit(p.Key, keyIndex) == 0 { // left branching
				currentHash = computeFullHash(currentHash, sibling)
				continue
			}
			currentHash = computeFullHash(sibling, currentHash) // right branching
			continue
		}

		// its a short node

		// read and pop from SkipBits
		if skipBitIndex == 0 {
			return false, fmt.Errorf("malformed proof, no more SkipBits available to read")
		}
		skipBits := int(p.SkipBits[skipBitIndex])
		skipBitIndex--

		// decrement the keyIndex by the number of bits that were skiped in the short node
		keyIndex -= skipBits

		// construct the common path
		commonPath := bitutils.MakeBitVector(skipBits)
		for j := 0; j < skipBits; j++ {
			if bitutils.ReadBit(p.Key, keyIndex+j) == 1 {
				bitutils.SetBit(commonPath, j)
			}
		}

		// compute the hash for the short node
		currentHash = computeShortHash(skipBits, commonPath, currentHash)
	}

	// in the end we should have used all the path space available
	if keyIndex != 0 {
		return false, fmt.Errorf("there are more bits in the key that has not been checked")
	}

	// the final hash value should match whith what was expected
	if !bytes.Equal(currentHash, expectedRootHash) {
		return false, fmt.Errorf("rootHash not matched")
	}

	return true, nil
}
