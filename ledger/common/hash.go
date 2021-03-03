package common

import (
	"os"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto/hash"
	c "github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/utils"
)

// default value and default hash value for a default node
var defaultLeafHash c.Hash

const defaultHashLen = 257

// HashLen is the default output hash length in bytes
const HashLen = 32

// hash with all zeroes used for padding
var EmptyHash [HashLen]byte

// we are currently supporting paths of a size up to 32 bytes. I.e. path length from the rootNode of a fully expanded tree to the leaf node is 256. A path of length k is comprised of k+1 vertices. Hence, we need 257 default hashes.
var defaultHashes [defaultHashLen][]byte

// TODO: remove once the hashing with 32-bytes input is tested
var log zerolog.Logger

func init() {

	log = zerolog.New(os.Stderr)

	hasher := hash.NewSHA3_256()
	defaultLeafHash = hasher.ComputeHash([]byte("default:"))

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
// TODO: delete this function after refactoring ptrie
func HashLeaf(path []byte, value []byte) []byte {
	// TODO: this is a sanity check and should be removed soon
	if len(path) != HashLen {
		log.Warn().Msgf("HashLeaf path input should be 32 bytes, got %d", len(path))
		hasher := hash.NewSHA3_256()
		_, _ = hasher.Write(path)
		_, _ = hasher.Write(value)
		return hasher.SumHash()
	}
	out := make([]byte, HashLen)
	hasher := new256()
	hasher.hash256Plus(&out, path, value) // path is always 256 bits
	return out
}

// HashInterNode generates hash value for intermediate nodes (SHA3-256).
//
// hash1 and hash2 must each be a 32 byte slice.
// TODO: delete this function after refactoring ptrie
func HashInterNode(hash1 []byte, hash2 []byte) []byte {
	// TODO: this is a sanity check and should be removed soon
	if len(hash1) != HashLen || len(hash2) != HashLen {
		log.Warn().Msgf("HashInterNode inputs should be 32 bytes, got %d and %d",
			len(hash1), len(hash2))
		hasher := hash.NewSHA3_256()
		_, _ = hasher.Write(hash1)
		_, _ = hasher.Write(hash2)
		return hasher.SumHash()
	}
	out := make([]byte, HashLen)
	hasher := new256()
	hasher.hash256plus256(&out, hash1, hash2) // hash1 and hash2 are 256 bits
	return out
}

// HashLeafIn generates hash value for leaf nodes (SHA3-256)
// and stores the result in the result input
//
// path must be a 32 byte slice.
// note that we don't include the keys here as they are already included in the path
func HashLeafIn(result *[]byte, path []byte, value []byte) {
	// TODO: this is a sanity check and should be removed soon
	if len(path) != HashLen {
		log.Warn().Msgf("HashLeaf path input should be 32 bytes, got %d", len(path))
		hasher := hash.NewSHA3_256()
		_, _ = hasher.Write(path)
		_, _ = hasher.Write(value)
		copy(*result, hasher.SumHash())
		return
	}
	hasher := new256()
	hasher.hash256Plus(result, path, value) // path is always 256 bits
}

// HashInterNodeIn generates hash value for intermediate nodes (SHA3-256)
// and stores the result in the input array.
//
// result slice can be equal to hash1 or hash2.
// hash1 and hash2 must each be a 32 byte slice.
func HashInterNodeIn(result *[]byte, hash1 []byte, hash2 []byte) {
	// TODO: this is a sanity check and should be removed soon
	if len(hash1) != HashLen || len(hash2) != HashLen {
		log.Warn().Msgf("HashInterNode inputs should be 32 bytes, got %d and %d",
			len(hash1), len(hash2))
		hasher := hash.NewSHA3_256()
		_, _ = hasher.Write(hash1)
		_, _ = hasher.Write(hash2)
		copy(*result, hasher.SumHash())
		return
	}
	hasher := new256()
	hasher.hash256plus256(result, hash1, hash2) // hash1 and hash2 are 256 bits
}

// ComputeCompactValue computes the value for the node considering the sub tree
// to only include this value and default values. It writes the hash result to the result input.
// UNCHECKED: payload!= nil
func ComputeCompactValue(result *[]byte, path []byte, payload *ledger.Payload, nodeHeight int) {
	// if register is unallocated: return default hash
	if len(payload.Value) == 0 {
		copy(*result, GetDefaultHashForHeight(nodeHeight))
		return
	}

	// register is allocated
	treeHeight := 8 * len(path)

	HashLeafIn(result, path, payload.Value) // we first compute the hash of the fully-expanded leaf
	for h := 1; h <= nodeHeight; h++ {      // then, we hash our way upwards towards the root until we hit the specified nodeHeight
		// h is the height of the node, whose hash we are computing in this iteration.
		// The hash is computed from the node's children at height h-1.
		bit := utils.Bit(path, treeHeight-h)
		if bit == 1 { // right branching
			HashInterNodeIn(result, GetDefaultHashForHeight(h-1), *result)
		} else { // left branching
			HashInterNodeIn(result, *result, GetDefaultHashForHeight(h-1))
		}
	}
}
