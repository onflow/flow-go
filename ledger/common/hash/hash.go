package hash

// HashLen is the ledger default output hash length in bytes
const HashLen = 32

// Hash is the hash type used in all ledger
type Hash [HashLen]byte

// DummyHash is an arbitrary hash value, used in function errors.
// DummyHash represents a valid hash value.
var DummyHash Hash

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
