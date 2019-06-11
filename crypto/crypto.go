package crypto

import (
	"golang.org/x/crypto/sha3"
)

// ComputeHash computes the SHA3-256 hash of some arbitrary set of data.
func ComputeHash(data []byte) []byte {
	hash := sha3.New256()
	hash.Write(data)
	return hash.Sum(nil)
}
