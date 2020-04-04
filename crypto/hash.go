package crypto

import (
	"bytes"
	"encoding/hex"
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
	return hex.EncodeToString(h)
}

// Hasher interface
type Hasher interface {
	// Algorithm returns the hashing algorithm for this hasher.
	Algorithm() HashingAlgorithm
	// Size returns the hash output length
	Size() int
	// ComputeHash returns the hash output regardless of the hash state
	ComputeHash([]byte) Hash
	// Write([]bytes) (using the io.Writer interface) adds more bytes to the
	// current hash state
	io.Writer
	// SumHash returns the hash output and resets the hash state
	SumHash() Hash
	// Reset resets the hash state
	Reset()
}

// commonHasher holds the common data for all hashers
type commonHasher struct {
	algo       HashingAlgorithm
	outputSize int
}

func (a *commonHasher) Algorithm() HashingAlgorithm {
	return a.algo
}

func BytesToHash(b []byte) Hash {
	h := make([]byte, len(b))
	copy(h, b)
	return h
}

// HashesToBytes converts a slice of hashes to a slice of byte slices.
func HashesToBytes(hashes []Hash) [][]byte {
	b := make([][]byte, len(hashes))
	for i, h := range hashes {
		b[i] = h
	}
	return b
}

// NewHasher chooses and initializes a hashing algorithm
// Deprecated and will removed later: use dedicated hash generation functions instead.
func NewHasher(algo HashingAlgorithm) (Hasher, error) {
	switch algo {
	case SHA3_256:
		return NewSHA3_256(), nil
	case SHA3_384:
		return NewSHA3_384(), nil
	case SHA2_256:
		return NewSHA2_256(), nil
	case SHA2_384:
		return NewSHA2_384(), nil
	default:
		return nil, fmt.Errorf("the hashing algorithm %s is not supported.", algo)
	}
}
