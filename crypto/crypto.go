package crypto

import (
	"golang.org/x/crypto/sha3"
)

// ComputeHash computes the Keccak256 hash of some arbitrary set of data, returns a []byte.
func ComputeHash(data []byte) []byte {
	hash := sha3.New256()
	hash.Write(data)
	return hash.Sum(nil)
}
