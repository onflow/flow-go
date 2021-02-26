package common

import (
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
)

// default value and default hash value for a default node
var emptySlice []byte
var defaultLeafHash = HashLeaf([]byte("default:"), emptySlice)

const defaultHashLen = 257

// we are currently supporting paths of a size up to 32 bytes. I.e. path length from the rootNode of a fully expanded tree to the leaf node is 256. A path of length k is comprised of k+1 vertices. Hence, we need 257 default hashes.
var defaultHashes [defaultHashLen][]byte

func init() {
	// Creates the Default hashes from base to level height
	defaultHashes[0] = defaultLeafHash
	for i := 1; i < defaultHashLen; i++ {
		defaultHashes[i] = HashInterNode(defaultHashes[i-1], defaultHashes[i-1])
	}
}

// GetDefaultHashes returns the default hashes of the SMT.
//
// For each tree level N, there is a default hash equal to the chained
// hashing of the default value N times.
func GetDefaultHashes() [defaultHashLen][]byte {
	return defaultHashes
}

// GetDefaultHashForHeight returns the default hashes of the SMT at a specified height.
//
// For each tree level N, there is a default hash equal to the chained
// hashing of the default value N times.
func GetDefaultHashForHeight(height int) []byte {
	return defaultHashes[height]
}

// HashLeaf generates hash value for leaf nodes (SHA3-256).
//
// path must be a 32 byte slice.
// note that we don't include the keys here as they are already included in the path
func HashLeaf(path []byte, value []byte) []byte {
	hasher := new256()
	hasher.write256(path) // path is always 256 bits
	hasher.write(value)
	hasher.finalize()
	return hasher.hashResult()
}

// HashInterNode generates hash value for intermediate nodes (SHA3-256).
//
// hash1 and hash2 must each be a 32 byte slice.
func HashInterNode(hash1 []byte, hash2 []byte) []byte {
	hasher := new256()
	hasher.write256(hash1) // hash1 and hash2 are 256 bits
	hasher.write256(hash2)
	hasher.finalize512()
	return hasher.hashResult()
}

// ComputeCompactValue computes the value for the node considering the sub tree to only include this value and default values.
func ComputeCompactValue(path []byte, payload *ledger.Payload, nodeHeight int) []byte {
	// if register is unallocated: return default hash
	if len(payload.Value) == 0 {
		return GetDefaultHashForHeight(nodeHeight)
	}

	// register is allocated
	treeHeight := 8 * len(path)
	// TODO Change this later to include the key as well
	// for now is just the value to make it compatible with previous code
	computedHash := HashLeaf(path, payload.Value) // we first compute the hash of the fully-expanded leaf
	for h := 1; h <= nodeHeight; h++ {            // then, we hash our way upwards towards the root until we hit the specified nodeHeight
		// h is the height of the node, whose hash we are computing in this iteration.
		// The hash is computed from the node's children at height h-1.
		bit := utils.Bit(path, treeHeight-h)
		if bit == 1 { // right branching
			computedHash = HashInterNode(GetDefaultHashForHeight(h-1), computedHash)
		} else { // left branching
			computedHash = HashInterNode(computedHash, GetDefaultHashForHeight(h-1))
		}
	}
	return computedHash
}
