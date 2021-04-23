package hash

import "fmt"

// HashLen is the ledger default output hash length in bytes
const HashLen = 32

// Hash is the hash type used in all ledger
type Hash [HashLen]byte

// DummyHash is an arbitrary hash value, used in function errors.
// DummyHash represents a valid hash value.
var DummyHash Hash

// HashLeaf returns the hash value for leaf nodes.
//
// path must be a 32 byte slice.
// note that we don't include the keys here as they are already included in the path.
func HashLeaf(path Hash, value []byte) Hash {
	hasher := new256()
	return hasher.hash256Plus(path, value) // path is 256 bits
}

// HashInterNode returns the hash value for intermediate nodes.
//
// hash1 and hash2 must be a 32 byte slice each.
func HashInterNode(hash1 Hash, hash2 Hash) Hash {
	hasher := new256()
	return hasher.hash256plus256(hash1, hash2) // hash1 and hash2 are 256 bits
}

// ToState converts a byte slice into a State.
// It returns an error if the slice has an invalid length.
func ToHash(bytes []byte) (Hash, error) {
	var h Hash
	if len(bytes) != len(h) {
		return DummyHash, fmt.Errorf("expecting %d bytes but got %d bytes", len(h), len(bytes))
	}
	copy(h[:], bytes)
	return h, nil
}
