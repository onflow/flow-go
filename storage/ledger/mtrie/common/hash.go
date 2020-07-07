package common

import (
	"crypto/sha256"

	"github.com/dapperlabs/flow-go/crypto/hash"
)

var emptySlice []byte
var defaultLeafHash = HashLeaf([]byte("default:"), emptySlice, emptySlice)

// we are currently supporting paths of a size up to 32 bytes. I.e. path length from the rootNode of a fully expanded tree to the leaf node is 256. A path of length k is comprised of k+1 vertices. Hence, we need 257 default hashes.
var defaultHashes [257][]byte

func init() {
	// Creates the Default hashes from base to level height
	defaultHashes[0] = defaultLeafHash
	for i := 1; i < len(defaultHashes); i++ {
		defaultHashes[i] = HashInterNode(defaultHashes[i-1], defaultHashes[i-1])
	}
}

// GetDefaultHashes returns the default hashes of the SMT.
//
// For each tree level N, there is a default hash equal to the chained
// hashing of the default value N times.
func GetDefaultHashes() [257][]byte {
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
// Currently we ignore the key since is already included in path
func HashLeaf(path []byte, key []byte, value []byte) []byte {
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
func HashInterNode(hash1 []byte, hash2 []byte) []byte {
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
func ComputeCompactValue(path []byte, key []byte, value []byte, nodeHeight int) []byte {
	// if register is unallocated: return default hash
	if len(value) == 0 {
		return GetDefaultHashForHeight(nodeHeight)
	}

	// register is allocated
	treeHeight := 8 * len(path)
	computedHash := HashLeaf(path, key, value) // we first compute the hash of the fully-expanded leaf
	for h := 1; h <= nodeHeight; h++ {         // then, we hash our way upwards towards the root until we hit the specified nodeHeight
		// h is the height of the node, whose hash we are computing in this iteration.
		// The hash is computed from the node's children at height h-1.
		bitIsSet, err := IsBitSet(path, treeHeight-h)
		if err != nil { // this won't happen ever
			panic(err)
		}
		if bitIsSet { // right branching
			computedHash = HashInterNode(GetDefaultHashForHeight(h-1), computedHash)
		} else { // left branching
			computedHash = HashInterNode(computedHash, GetDefaultHashForHeight(h-1))
		}
	}
	return computedHash
}

// KeyToPath computes the storage path given a key
func KeyToPath(key []byte) []byte {
	// TODO change this to Sha3
	h := sha256.New()
	_, _ = h.Write(key)
	return h.Sum(nil)
}
