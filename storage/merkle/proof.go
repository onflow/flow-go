package merkle

import (
	"bytes"

	"github.com/onflow/flow-go/ledger/common/bitutils"
)

// Proof captures all data needed for proving inclusion of a single value inserted under key `Key` into the merkle trie
type Proof struct {
	// Key used to insert and look up the value
	Key []byte
	// Value stored in the trie for the given key
	Value []byte
	// InterimNodeTypes holds bits of data to determine short nodes versus full nodes while traversing the
	// trie downward. if the bit is set to 1, it means that we have reached to a short node, and
	// if is set to 0 means we have reached a full node.
	InterimNodeTypes []byte
	// SkipBits is read when we reach a short node, and the value represents number of bits that were skipped
	// by the short node (shortNode.count)
	SkipBits []uint16
	// SiblingHashes is a slice of hash values, every value is read when we reach a full node (hash value of the siblings)
	SiblingHashes [][]byte
}

// Verify verifies the proof by constructing the hash values bottom up and cross check
// the constructed root hash with the given one.
// if the proof is valid it returns true and false otherwise
func (p *Proof) Verify(expectedRootHash []byte) (bool, error) {

	// first check the key size to be smaller than max allowed
	if len(p.Key) > maxKeyLength {
		return false, NewMalformedProofErrorf("key length is larger than max key lenght allowed (%d > %d)", len(p.Key), maxKeyLength)
	}

	// number of steps
	steps := len(p.SkipBits) + len(p.SiblingHashes)
	// number of steps should be smaller than max key length
	if steps > maxKeyLength {
		return false, NewMalformedProofErrorf("length of SkipBits pluse length of SiblingHashes is larger than max key lenght allowed (%d > %d)", steps, maxKeyLength)
	}
	// number of steps should be smaller than number of bits in the InterimNodeTypes
	if len(p.InterimNodeTypes) != (steps+7)>>3 {
		return false, NewMalformedProofErrorf("the length of InterimNodeTypes doesn't match the length of SkipBits and SiblingHashes")
	}

	// an index to consume interim hashes from the last element to the first element
	interimHashIndex := len(p.SiblingHashes) - 1
	// an index to consume shortBits from the last element to the first element
	skipBitIndex := len(p.SkipBits) - 1

	// keyIndex keeps track of the largest index of the key that is unchecked.
	// note that traverse bottom up here, so we start with the largest key index
	// build hashes until we reach to the root.
	keyIndex := len(p.SiblingHashes)
	for _, sc := range p.SkipBits {
		keyIndex += int(sc)
	}
	keyIndex-- // consider index starts from zero

	if len(p.Key)*8 != keyIndex+1 {
		return false, NewMalformedProofErrorf("key length doesn't match the length of SkipBits and SiblingHashes")
	}

	// compute the hash value of the leaf
	currentHash := computeLeafHash(p.Value)

	// for each step (level from bottom to top) check if its a full node or a short node and compute the
	// hash value accordingly; for full node having the sibling hash helps to compute the hash value
	// of the next level, for short nodes compute the hash using the common path constructed based on
	// the given short count
	for interimNodeTypesIndex := steps - 1; interimNodeTypesIndex >= 0; interimNodeTypesIndex-- {

		// Full node
		if bitutils.ReadBit(p.InterimNodeTypes, interimNodeTypesIndex) == 0 {

			// read and pop the sibling hash value from SiblingHashes
			if interimHashIndex < 0 {
				return false, NewMalformedProofErrorf("no more SiblingHashes available to read")
			}
			sibling := p.SiblingHashes[interimHashIndex]
			interimHashIndex--

			// based on the bit at keyIndex, compute the hash
			if bitutils.ReadBit(p.Key, keyIndex) == 0 { // left branching
				currentHash = computeFullHash(currentHash, sibling)
			} else {
				currentHash = computeFullHash(sibling, currentHash) // right branching
			}

			// move to the parent vertex along the path
			keyIndex--

			continue
		}

		// Short node

		// read and pop from SkipBits
		if skipBitIndex < 0 {
			return false, NewMalformedProofErrorf("no more SkipBits available to read")
		}
		skipBits := int(p.SkipBits[skipBitIndex])
		skipBitIndex--

		// construct the common path
		commonPath := bitutils.MakeBitVector(skipBits)
		for c := skipBits - 1; c >= 0; c-- {
			if bitutils.ReadBit(p.Key, keyIndex) == 1 {
				bitutils.SetBit(commonPath, c)
			}
			keyIndex--
		}
		// compute the hash for the short node
		currentHash = computeShortHash(skipBits, commonPath, currentHash)
	}

	// in the end we should have used all the path space available
	if keyIndex >= 0 {
		return false, NewMalformedProofErrorf("a subset of the key is not checked (keyIndex: %d)", keyIndex)
	}

	// the final hash value should match whith what was expected
	if !bytes.Equal(currentHash, expectedRootHash) {
		return false, NewMalformedProofErrorf("root hash doesn't match, expected %X, computed %X", expectedRootHash, currentHash)
	}

	return true, nil
}
