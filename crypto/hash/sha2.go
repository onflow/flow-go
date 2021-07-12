package hash

import (
	"crypto/sha256"
	"crypto/sha512"
	"hash"
)

// sha2_256Algo, embeds commonHasher
type sha2_256Algo struct {
	hash.Hash
}

// NewSHA2_256 returns a new instance of SHA2-256 hasher
func NewSHA2_256() Hasher {
	return &sha2_256Algo{
		Hash: sha256.New()}
}

func (s *sha2_256Algo) Algorithm() HashingAlgorithm {
	return SHA2_256
}

// ComputeHash calculates and returns the SHA2-256 digest of the input.
// The function updates the state (and therefore not thread-safe)
// but does not reset the state to allow further writing.
func (s *sha2_256Algo) ComputeHash(data []byte) Hash {
	s.Reset()
	// `Write` delegates this call to sha256.digest's `Write` which does not return an error.
	_, _ = s.Write(data)
	return s.Sum(nil)
}

// SumHash returns the SHA2-256 output.
// It does not reset the state to allow further writing.
func (s *sha2_256Algo) SumHash() Hash {
	return s.Sum(nil)
}

// sha2_384Algo, embeds commonHasher
type sha2_384Algo struct {
	hash.Hash
}

// NewSHA2_384 returns a new instance of SHA2-384 hasher
func NewSHA2_384() Hasher {
	return &sha2_384Algo{
		Hash: sha512.New384()}
}

func (s *sha2_384Algo) Algorithm() HashingAlgorithm {
	return SHA2_384
}

// ComputeHash calculates and returns the SHA2-384 digest of the input.
// It does not reset the state to allow further writing.
func (s *sha2_384Algo) ComputeHash(data []byte) Hash {
	s.Reset()
	// `Write` delegates this call to sha512.digest's `Write` which does not return an error.
	_, _ = s.Write(data)
	return s.Sum(nil)
}

// SumHash returns the SHA2-384 output.
// It does not reset the state to allow further writing.
func (s *sha2_384Algo) SumHash() Hash {
	return s.Sum(nil)
}
