package mtrie

import (
	"golang.org/x/crypto/blake2b"
)

var emptySlice []byte
var defaultLeafHash = HashLeaf([]byte("default:"), emptySlice)

// we are currently supporting keys of a size up to 32 bytes. I.e. path length from the rootNode of a fully expanded tree to the leaf node is 256. A path of length k is comprised of k+1 vertices. Hence, we need 257 default hashes.
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
func HashLeaf(key []byte, value []byte) []byte {
	hasher, _ := blake2b.New256(nil)
	_, err := hasher.Write(key)
	if err != nil {
		panic(err)
	}
	_, err = hasher.Write(value)
	if err != nil {
		panic(err)
	}
	return hasher.Sum(nil)
}

// HashInterNode generates hash value for intermediate nodes (SHA3-256).
func HashInterNode(hash1 []byte, hash2 []byte) []byte {
	hasher, _ := blake2b.New256(nil)
	_, err := hasher.Write(hash1)
	if err != nil {
		panic(err)
	}
	_, err = hasher.Write(hash2)
	if err != nil {
		panic(err)
	}
	return hasher.Sum(nil)
}

// ComputeCompactValue computes the value for the node considering the sub tree to only include this value and default values.
func ComputeCompactValue(key []byte, value []byte, nodeHeight int, maxHeight int) []byte {
	// if value is nil return default hash
	if len(value) == 0 {
		return GetDefaultHashForHeight(nodeHeight)
	}
	computedHash := HashLeaf(key, value)

	for j := maxHeight - 2; j > maxHeight-nodeHeight-2; j-- {
		bitIsSet, err := IsBitSet(key, j)
		// this won't happen ever
		if err != nil {
			panic(err)
		}
		if bitIsSet { // right branching
			computedHash = HashInterNode(GetDefaultHashForHeight(maxHeight-j-2), computedHash)
		} else { // left branching
			computedHash = HashInterNode(computedHash, GetDefaultHashForHeight(maxHeight-j-2))
		}
	}
	return computedHash
}
