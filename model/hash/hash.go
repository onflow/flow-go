package hash

import (
	"github.com/onflow/crypto/hash"
)

// DefaultComputeHash is the default hasher used by Flow.
//
// `ComputeSHA3_256` can be used directly
// to minimize heap allocations
func DefaultComputeHash(data []byte) hash.Hash {
	var res [hash.HashLenSHA3_256]byte
	hash.ComputeSHA3_256(&res, data)
	return hash.Hash(res[:])
}
