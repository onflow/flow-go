package hash

import (
	"bytes"
	"fmt"
	"io"
)

// Hash is the hash algorithms output types
type Hash []byte

// Equal checks if a hash is equal to a given hash
func (h Hash) Equal(input Hash) bool {
	return bytes.Equal(h, input)
}

// Hex returns the hex string representation of the hash.
func (h Hash) Hex() string {
	return fmt.Sprintf("%#x", []byte(h))
}

// String returns the hex string representation of the hash.
func (h Hash) String() string {
	return h.Hex()
}

// Hasher interface
type Hasher interface {
	// Algorithm returns the hashing algorithm of the hasher.
	Algorithm() HashingAlgorithm
	// Size returns the hash output length in bytes.
	Size() int
	// ComputeHash returns the hash output regardless of the existing hash state.
	// It may update the state or not depending on the implementation. Thread safety
	// also depends on the implementation.
	ComputeHash([]byte) Hash
	// Write([]bytes) (using the io.Writer interface) adds more bytes to the
	// current hash state.
	io.Writer
	// SumHash returns the hash output.
	// It may update the state or not depending on the implementation.
	SumHash() Hash
	// Reset resets the hash state.
	Reset()
}
