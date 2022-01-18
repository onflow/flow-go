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
	SkipBits []uint16
	// InterimHashes is a slice of hash values, every value is read when we reach a full node (hash value of the siblings)
	InterimHashes [][]byte
}

// Verify verifies the proof by constructing the hash values bottom up and cross check
// the constructed root hash with the given one.
// if the proof is valid it returns true and false otherwise
func (p *Proof) Verify(expectedRootHash []byte) (bool, error) {

	// number of steps
	steps := len(p.SkipBits) + len(p.InterimHashes)

	// number of steps should be smaller than number of bits i the IsAShortNode
	if steps > len(p.IsAShortNode)*8 {
		return false, fmt.Errorf("malformed proof, IsShortNode length doesnt match the size of skipbits and interimhashes")
	}

	// an index to consume interim hashes from the last element to the first element
	interimHashIndex := len(p.InterimHashes) - 1
	// an index to consume shortBits from the last element to the first element
	skipBitIndex := len(p.SkipBits) - 1

	// keyIndex keeps track of the largest index of the key that is unchecked.
	// note that traverse bottom up here, so we start with the largest key index
	// build hashes until we reach to the root.
	keyIndex := len(p.InterimHashes)
	for _, sc := range p.SkipBits {
		keyIndex += int(sc)
	}
	keyIndex-- // consider index starts from zero

	// compute the hash value of the leaf
	currentHash := computeLeafHash(p.Value)

	// for each step (level from bottom to top) check if its a full node or a short node and compute the
	// hash value accordingly; for full node having the sibling hash helps to compute the hash value
	// of the next level, for short nodes compute the hash using the common path constructed based on
	// the given short count
	for isAShortNodeIndex := steps - 1; isAShortNodeIndex >= 0; isAShortNodeIndex-- {

		// Full node
		if bitutils.ReadBit(p.IsAShortNode, isAShortNodeIndex) == 0 {

			// read and pop the sibling hash value from InterimHashes
			if interimHashIndex < 0 {
				return false, fmt.Errorf("malformed proof, no more InterimHashes available to read")
			}
			sibling := p.InterimHashes[interimHashIndex]
			interimHashIndex--

			// based on the bit at pathIndex of the key compute the hash
			if bitutils.ReadBit(p.Key, keyIndex) == 0 { // left branching
				currentHash = computeFullHash(currentHash, sibling)
			} else {
				currentHash = computeFullHash(sibling, currentHash) // right branching
			}

			// decrement the keyindex by 1
			keyIndex--

			continue
		}

		// Short node

		// read and pop from SkipBits
		if skipBitIndex < 0 {
			return false, fmt.Errorf("malformed proof, no more SkipBits available to read")
		}
		skipBits := int(p.SkipBits[skipBitIndex])
		skipBitIndex--

		// construct the common path
		startIndexOfCommonPath := keyIndex - skipBits + 1
		commonPath := bitutils.MakeBitVector(skipBits)
		for j := 0; j < skipBits; j++ {
			if bitutils.ReadBit(p.Key, startIndexOfCommonPath+j) == 1 {
				bitutils.SetBit(commonPath, j)
			}
		}

		// compute the hash for the short node
		currentHash = computeShortHash(skipBits, commonPath, currentHash)

		// decrement the keyIndex by the number of bits that were skiped in the short node
		keyIndex -= skipBits
	}

	// in the end we should have used all the path space available
	if keyIndex >= 0 {
		return false, fmt.Errorf("there are more bits in the key that has not been checked")
	}

	// the final hash value should match whith what was expected
	if !bytes.Equal(currentHash, expectedRootHash) {
		return false, fmt.Errorf("rootHash not matched")
	}

	return true, nil
}
