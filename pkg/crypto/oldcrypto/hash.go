package oldcrypto

import (
	"encoding/hex"
)

const (
	// HashLength is the size of a SHA3-256 hash.
	HashLength = 32
)

// Hash represents the 32 byte SHA3-256 hash of arbitrary data.
type Hash [HashLength]byte

// BytesToHash returns a Hash with value b.
// If b is larger than len(h), b will be cropped from the left.
func BytesToHash(b []byte) Hash {
	var h Hash
	h.SetBytes(b)
	return h
}

// HashesToBytes converts a slice of hashes to a slice of bytes.
func HashesToBytes(hashes []Hash) [][]byte {
	b := make([][]byte, len(hashes))

	for i, h := range hashes {
		b[i] = h.Bytes()
	}

	return b
}

// BytesToHashes converts a slice of bytes to a slice of hashes.
func BytesToHashes(b [][]byte) []Hash {
	hashes := make([]Hash, len(b))

	for i, v := range b {
		hashes[i] = BytesToHash(v)
	}

	return hashes
}

// SetBytes sets the hash to the value of b.
// If b is larger than len(h), b will be cropped from the left.
func (h *Hash) SetBytes(b []byte) {
	if len(b) > len(h) {
		b = b[len(b)-HashLength:]
	}

	copy(h[HashLength-len(b):], b)
}

// Bytes gets the byte representation of the underlying hash.
func (h Hash) Bytes() []byte { return h[:] }

// NewHash computes the SHA3-256 hash of some arbitrary set of data.
func NewHash(data []byte) Hash {
	return BytesToHash(ComputeHash(data))
}

// String encodes Hash as a readable string for logging purposes.
func (h Hash) String() string {
	return hex.EncodeToString(h.Bytes())
}

// ZeroBlockHash represents the parent hash of the genesis block.
func ZeroBlockHash() Hash {
	return Hash{}
}
