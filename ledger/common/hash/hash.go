package hash

import (
	"os"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/ledger/common/utils"
)

// HashLen is the ledger default output hash length in bytes
const HashLen = 32

// Hash is the hash type used in all ledger
type Hash [HashLen]byte

// default value and default hash value for a default node
var defaultLeafHash Hash

// EmptyHash is a hash with all zeroes, used for padding
var EmptyHash Hash

// tree maximum height
const TreeMaxHeight = 256

// we are currently supporting paths of a size equal to 32 bytes.
// I.e. path length from the rootNode of a fully expanded tree to the leaf node is 256.
// A path of length k is comprised of k+1 vertices. Hence, we need 257 default hashes.
const defaultHashesNum = TreeMaxHeight + 1

// array to store all default hashes
var defaultHashes [defaultHashesNum]Hash

// TODO: remove once the hashing with 32-bytes input is tested
var log zerolog.Logger

func init() {

	log = zerolog.New(os.Stderr)

	hasher := hash.NewSHA3_256()
	copy(defaultLeafHash[:], hasher.ComputeHash([]byte("default:")))

	// Creates the Default hashes from base to level height
	defaultHashes[0] = defaultLeafHash
	for i := 1; i < defaultHashesNum; i++ {
		defaultHashes[i] = HashInterNode(defaultHashes[i-1], defaultHashes[i-1])
	}
}

// GetDefaultHashes returns the default hashes of the SMT.
//
// For each tree level N, there is a default hash equal to the chained
// hashing of the default value N times.
func GetDefaultHashes() [defaultHashesNum]Hash {
	return defaultHashes
}

// GetDefaultHashForHeight returns the default hashes of the SMT at a specified height.
//
// For each tree level N, there is a default hash equal to the chained
// hashing of the default value N times.
func GetDefaultHashForHeight(height int) Hash {
	return defaultHashes[height]
}

// HashLeaf generates hash value for leaf nodes (SHA3-256).
//
// path must be a 32 byte slice.
// note that we don't include the keys here as they are already included in the path
// TODO: delete this function after refactoring ptrie
func HashLeaf(path Hash, value []byte) Hash {
	hasher := new256()
	return hasher.hash256Plus(path, value) // path is always 256 bits
}

// HashInterNode generates hash value for intermediate nodes (SHA3-256).
//
// hash1 and hash2 must each be a 32 byte slice.
// TODO: delete this function after refactoring ptrie
func HashInterNode(hash1 Hash, hash2 Hash) Hash {
	hasher := new256()
	return hasher.hash256plus256(hash1, hash2) // hash1 and hash2 are 256 bits
}

// HashLeafIn generates hash value for leaf nodes (SHA3-256)
// and stores the result in the result input
//
// path must be a 32 byte slice.
// note that we don't include the keys here as they are already included in the path
func HashLeafIn(path Hash, value []byte) Hash {
	hasher := new256()
	return hasher.hash256Plus(path, value) // path is always 256 bits
}

// HashInterNodeIn generates hash value for intermediate nodes (SHA3-256)
// and stores the result in the input array.
//
// result slice can be equal to hash1 or hash2.
// hash1 and hash2 must each be a 32 byte slice.
func HashInterNodeIn(hash1 Hash, hash2 Hash) Hash {
	hasher := new256()
	return hasher.hash256plus256(hash1, hash2) // hash1 and hash2 are 256 bits
}

// ComputeCompactValue computes the value for the node considering the sub tree
// to only include this value and default values. It writes the hash result to the result input.
// UNCHECKED: payload!= nil
func ComputeCompactValue(path Hash, value []byte, nodeHeight int) Hash {
	// if register is unallocated: return default hash
	if len(value) == 0 {
		return GetDefaultHashForHeight(nodeHeight)
	}

	var out Hash
	out = HashLeafIn(path, value)      // we first compute the hash of the fully-expanded leaf
	for h := 1; h <= nodeHeight; h++ { // then, we hash our way upwards towards the root until we hit the specified nodeHeight
		// h is the height of the node, whose hash we are computing in this iteration.
		// The hash is computed from the node's children at height h-1.
		bit := utils.Bit(path[:], TreeMaxHeight-h)
		if bit == 1 { // right branching
			out = HashInterNodeIn(GetDefaultHashForHeight(h-1), out)
		} else { // left branching
			out = HashInterNodeIn(out, GetDefaultHashForHeight(h-1))
		}
	}
	return out
}
