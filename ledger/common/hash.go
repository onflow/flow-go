package common

import (
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
)

// hash method 1 mean using sha3-256
// hash method 2 means using blake2s
const defaultHashMethod = 1

type LedgerHasher struct {
	hashMethod      int
	defaultLeafHash []byte
	// we are currently supporting paths of a size up to 32 bytes. I.e. path length from the rootNode of a fully expanded tree to the leaf node is 256. A path of length k is comprised of k+1 vertices. Hence, we need 257 default hashes.
	defaultHashes [257][]byte
}

func newLedgerHasher(hashMethod int) *LedgerHasher {
	lh := &LedgerHasher{
		hashMethod: hashMethod,
	}

	// default value and default hash value for a default node
	var emptySlice []byte
	lh.defaultLeafHash = lh.HashLeaf([]byte("default:"), emptySlice)

	var defaultHashes [257][]byte
	// Creates the Default hashes from base to level height
	defaultHashes[0] = lh.defaultLeafHash
	for i := 1; i < len(defaultHashes); i++ {
		defaultHashes[i] = lh.HashInterNode(defaultHashes[i-1], defaultHashes[i-1])
	}
	lh.defaultHashes = defaultHashes
	return lh
}

// GetDefaultHashes returns the default hashes of the SMT.
//
// For each tree level N, there is a default hash equal to the chained
// hashing of the default value N times.
func (lh *LedgerHasher) GetDefaultHashes() [257][]byte {
	return lh.defaultHashes
}

// GetDefaultHashForHeight returns the default hashes of the SMT at a specified height.
//
// For each tree level N, there is a default hash equal to the chained
// hashing of the default value N times.
func (lh *LedgerHasher) GetDefaultHashForHeight(height int) []byte {
	return lh.defaultHashes[height]
}

// HashLeaf generates hash value for leaf nodes (SHA3-256).
// note that we don't include the keys here as they are already included in the path
func (lh *LedgerHasher) HashLeaf(path []byte, value []byte) []byte {
	hasher := hash.NewSHA3_256()
	_, err := hasher.Write(path)
	if err != nil {
		panic(err)
	}
	_, err = hasher.Write(value)
	if err != nil {
		panic(err)
	}

	return hasher.SumHash()
}

// HashInterNode generates hash value for intermediate nodes (SHA3-256).
func (lh *LedgerHasher) HashInterNode(hash1 []byte, hash2 []byte) []byte {
	hasher := hash.NewSHA3_256()
	_, err := hasher.Write(hash1)
	if err != nil {
		panic(err)
	}
	_, err = hasher.Write(hash2)
	if err != nil {
		panic(err)
	}
	return hasher.SumHash()
}

// ComputeCompactValue computes the value for the node considering the sub tree to only include this value and default values.
func (lh *LedgerHasher) ComputeCompactValue(path []byte, payload *ledger.Payload, nodeHeight int) []byte {
	// if register is unallocated: return default hash
	if len(payload.Value) == 0 {
		return lh.GetDefaultHashForHeight(nodeHeight)
	}

	// register is allocated
	treeHeight := 8 * len(path)
	// TODO Change this later to include the key as well
	// for now is just the value to make it compatible with previous code
	computedHash := lh.HashLeaf(path, payload.Value) // we first compute the hash of the fully-expanded leaf
	for h := 1; h <= nodeHeight; h++ {               // then, we hash our way upwards towards the root until we hit the specified nodeHeight
		// h is the height of the node, whose hash we are computing in this iteration.
		// The hash is computed from the node's children at height h-1.
		bitIsSet, err := utils.IsBitSet(path, treeHeight-h)
		if err != nil { // this won't happen ever
			panic(err)
		}
		if bitIsSet { // right branching
			computedHash = lh.HashInterNode(lh.GetDefaultHashForHeight(h-1), computedHash)
		} else { // left branching
			computedHash = lh.HashInterNode(computedHash, lh.GetDefaultHashForHeight(h-1))
		}
	}
	return computedHash
}
