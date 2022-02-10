package merkle

import (
	"bytes"
	"math/bits"

	"github.com/onflow/flow-go/ledger/common/bitutils"
)

// Proof captures all data needed for proving inclusion of a single key/value pair inside a merkle trie
// verifying proof requires knowledge of the trie path structure (node types) and traversing
// the trie from the leaf to the root and computing hash values.
type Proof struct {
	// Key used to insert and look up the value
	Key []byte
	// Value stored in the trie for the given key
	Value []byte
	// InterimNodeTypes is designed to be consumed bit by bit to determine if the next node
	// is a short nodes or full nodes while traversing the trie downward (0: fullnode, 1: shortnode).
	// The very first bit corresponds to the root of the trie and last bit is the last
	// interim node before reaching to the leaf.
	// Note that we always allocated the minimal number of bytes needed to capture all
	// the nodes in the path (padded with zero)
	InterimNodeTypes []byte
	// ShortPathLengths is read when we reach a short node, and the value represents number of common bits that were included
	// in the short node (shortNode.count). Elements are ordered from root to leaf.
	// WARNING, similar to the serializedPathSegmentLength method using by the trie, the uint16 encoding here requires special handling of value zero,
	// since shortNode.count can only have values in the range of [1, 65536], and a uint16 supports range of [0, 65535],
	// zero should be mapped to 65536 (all other values are the same)
	ShortPathLengths []uint16
	// SiblingHashes is read when we reach a full node. The corresponding element represents
	// the hash of the non-visited sibling node for each full node on the path. Elements are ordered from root to leaf.
	SiblingHashes [][]byte
}

const maxStepsForProofVerification = maxKeyLength * 8

// validateFormat validates the format and size of elements of the proof (syntax check)
//
// A valid proof as to satisfy the following consistency conditions:
// 1. A valid inclusion proof represents a full path through the merkle tree.
//    We separate the path into a sequence of interim vertices and a tailing leaf.
//    For interim vertex (with index i, counted from the root node) along the path,
//    the proof as to contain the following information:
//      (i) whether the vertex is a short node (InterimNodeTypes[i] == 1) or
//          a full node (InterimNodeTypes[i] == 0)
//     (ii) for each short node, we need the number of bits in the node's key segment
//          (entry in ShortPathLengths)
//    (iii) for a full node, we need the hash of the sibling that is _not_ on the path
//          (entry in SiblingHashes)
//    Hence, len(ShortPathLengths) + len(SiblingHashes) specifies how many _interim_
//    vertices are on the merkle path. Consequently, we require the same number of _bits_
//    in InterimNodeTypes. Therefore, we know that InterimNodeTypes should have a length
//    of `(numberBits+7)>>3` _bytes_.
// 2. The key length (measured in bytes) has to be in the interval [1, 8192].
//    Furthermore, each interim vertex on the merkle path represents:
//    * either a single bit in case of a full node:
//      we expect InterimNodeTypes[i] == 0
//    * a positive number of bits in case of a short node:
//      we expect InterimNodeTypes[i] == 1
//      and the number of bits is encoded in the respective element of ShortPathLengths
//    Hence, the total key length _in bits_ should be: len(SiblingHashes) + sum(ShortPathLengths)
func (p *Proof) validateFormat() error {

	// step1 - validate the key as the very first step
	if len(p.Key) == 0 {
		return NewMalformedProofErrorf("key is empty")
	}
	if len(p.Key) > maxKeyLength {
		return NewMalformedProofErrorf("key length is larger than max key length allowed (%d > %d)", len(p.Key), maxKeyLength)
	}

	// step2 - check ShortPathLengths and SiblingHashes

	// validate number of bits that is going to be checked matches the size of the given key
	keyBitCount := len(p.SiblingHashes)
	for _, sc := range p.ShortPathLengths {
		if keyBitCount > maxStepsForProofVerification {
			return NewMalformedProofErrorf("number of key bits (%d) exceed limit (%d)", keyBitCount, maxStepsForProofVerification)
		}
		keyBitCount += CountUint16EncodingToInt(sc)
	}

	if len(p.Key)*8 != keyBitCount {
		return NewMalformedProofErrorf("key length doesn't match the length of ShortPathLengths and SiblingHashes")
	}

	// step3 - check InterimNodeTypes

	// size checks
	if len(p.InterimNodeTypes) == 0 {
		return NewMalformedProofErrorf("InterimNodeTypes is empty")
	}
	if len(p.InterimNodeTypes) > maxKeyLength {
		return NewMalformedProofErrorf("InterimNodeTypes is larger than max key length allowed (%d > %d)", len(p.InterimNodeTypes), maxKeyLength)
	}
	// InterimNodeTypes should only use the smallest number of bytes needed for steps
	steps := len(p.ShortPathLengths) + len(p.SiblingHashes)
	if len(p.InterimNodeTypes) != (steps+7)>>3 {
		return NewMalformedProofErrorf("the length of InterimNodeTypes doesn't match the length of ShortPathLengths and SiblingHashes")
	}

	// semantic checks

	// Verify that number of bits that are set to 1 equals to the number of full nodes, i.e. len(SiblingHashes).
	numberOfShortNodes := 0
	for _, d := range p.InterimNodeTypes {
		numberOfShortNodes += bits.OnesCount8(d)
	}

	if numberOfShortNodes != len(p.ShortPathLengths) {
		return NewMalformedProofErrorf("len(ShortPathLengths) (%d) does not match number of set bits in InterimNodeTypes (%d)", len(p.ShortPathLengths), numberOfShortNodes)
	}

	numberOfFullNodes := steps - numberOfShortNodes
	if numberOfFullNodes != len(p.SiblingHashes) {
		return NewMalformedProofErrorf("len(SiblingHashes) (%d) does not match number of set bits in InterimNodeTypes (%d)", numberOfFullNodes, len(p.SiblingHashes))
	}

	// check that tailing auxiliary bits (to make a complete full byte) are all zero
	for i := len(p.InterimNodeTypes)*8 - 1; i >= steps; i-- {
		if bitutils.ReadBit(p.InterimNodeTypes, i) != 0 {
			return NewMalformedProofErrorf("tailing auxiliary bits in InterimNodeTypes should all be zero")
		}
	}

	return nil
}

// Verify verifies the proof by constructing the hash values bottom up and cross-check
// the constructed root hash with the given one. For valid proofs, `nil` is returned.
// During normal operations, the following error returns are expected:
//  * MalformedProofError if the proof has a syntactically invalid structure
//  * InvalidProofError if the proof is syntactically valid, but the reconstructed
//    root hash does not match the expected value.
func (p *Proof) Verify(expectedRootHash []byte) error {

	// first validate the format of the proof
	if err := p.validateFormat(); err != nil {
		return err
	}

	// an index to consume SiblingHashes from the last element to the first element
	siblingHashIndex := len(p.SiblingHashes) - 1

	// an index to consume ShortPathLengths from the last element to the first element
	shortPathLengthIndex := len(p.ShortPathLengths) - 1

	// keyIndex keeps track of the largest index of the key that is unchecked.
	// Note that we traverse bottom up, so we start with the largest key index
	// build hashes until we reach to the root.
	keyIndex := len(p.Key)*8 - 1

	// compute the hash value of the leaf
	currentHash := computeLeafHash(p.Value)

	// number of steps
	steps := len(p.ShortPathLengths) + len(p.SiblingHashes)

	// for each step (level from bottom to top) check if it's a full node or a short node and compute the
	// hash value accordingly; for full node having the sibling hash helps to compute the hash value
	// of the next level; for short nodes compute the hash using the common path constructed based on
	// the given short count
	for interimNodeTypesIndex := steps - 1; interimNodeTypesIndex >= 0; interimNodeTypesIndex-- {

		// Full node
		if bitutils.ReadBit(p.InterimNodeTypes, interimNodeTypesIndex) == 0 {

			// read and pop the sibling hash value from SiblingHashes
			sibling := p.SiblingHashes[siblingHashIndex]
			siblingHashIndex--

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

		// read and pop from ShortPathLengths
		shortPathLengths := CountUint16EncodingToInt(p.ShortPathLengths[shortPathLengthIndex])
		shortPathLengthIndex--

		// construct the common path
		commonPath := bitutils.MakeBitVector(shortPathLengths)
		for c := shortPathLengths - 1; c >= 0; c-- {
			if bitutils.ReadBit(p.Key, keyIndex) == 1 {
				bitutils.SetBit(commonPath, c)
			}
			keyIndex--
		}
		// compute the hash for the short node
		currentHash = computeShortHash(shortPathLengths, commonPath, currentHash)
	}

	// the final hash value should match whith what was expected
	if !bytes.Equal(currentHash, expectedRootHash) {
		return NewInvalidProofErrorf("root hash doesn't match, expected %X, computed %X", expectedRootHash, currentHash)
	}

	return nil
}
