package crypto

import (
	"github.com/ethereum/go-ethereum/common/hexutil"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/sha3"
)

// ComputeBytesHash calculates and returns the SHA3-256 output of input byte array
func ComputeBytesHash(data []byte) Hash {
	return sha3.Sum256(data)
}

// ComputeStructHash calculates and returns the SHA3-256 output of any input structure
func ComputeStructHash(struc Encoder) Hash {
	return sha3.Sum256(struc.Encode())
}

// Encoder is an interface of a generic structure
type Encoder interface {
	Encode() []byte
}

// BytesToHash sets b to hash.
// If b is larger than HashLength, b will be cropped from the right.
func BytesToHash(b []byte) Hash {
	if HashLength < len(b) {
		log.Warn("the array is cropped from the right")
	}
	var h Hash
	// number of copied bytes is min(len(b), HashLength)
	copy(h[0:], b)
	// clear the remaining bytes of h
	for i := len(b); i < HashLength; i++ {
		h[i] = 0
	}
	return h
}

// ToBytes sets a hash to b
// ToBytes gets the byte representation of the underlying hash.
func (h Hash) ToBytes() []byte {
	return h[:]
}

// Hex converts a hash to a hex string.
func (h Hash) Hex() string { return hexutil.Encode(h[:]) }

// String implements the stringer interface and is used also by the logger when
// doing full logging into a file.
func (h Hash) String() string {
	return h.Hex()
}

// IsEqual checks if a hash is equal to an input hash
func (h Hash) IsEqual(input *Hash) bool {
	return h.IsEqual(input)
}
