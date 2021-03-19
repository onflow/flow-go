package hash

import (
	"crypto/sha256"
	"crypto/sha512"
	"hash"
)

// sha2_256Algo, embeds commonHasher
type sha2_256Algo struct {
	*commonHasher
	hash.Hash
}

// NewSHA2_256 returns a new instance of SHA2-256 hasher
func NewSHA2_256() Hasher {
	return &sha2_256Algo{
		commonHasher: &commonHasher{
			algo:       SHA2_256,
			outputSize: HashLenSha2_256},
		Hash: sha256.New()}
}

// ComputeHash calculates and returns the SHA2-256 output of input byte array.
// It does not reset the state to allow further writing.
func (s *sha2_256Algo) ComputeHash(data []byte) Hash {
	s.Reset()
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
	*commonHasher
	hash.Hash
}

// NewSHA2_384 returns a new instance of SHA2-384 hasher
func NewSHA2_384() Hasher {
	return &sha2_384Algo{
		commonHasher: &commonHasher{
			algo:       SHA2_384,
			outputSize: HashLenSha2_384},
		Hash: sha512.New384()}
}

// ComputeHash calculates and returns the SHA2-384 output of the input byte array.
// It does not reset the state to allow further writing.
func (s *sha2_384Algo) ComputeHash(data []byte) Hash {
	s.Reset()
	_, _ = s.Write(data)
	return s.Sum(nil)
}

// SumHash returns the SHA2-384 output.
// It does not reset the state to allow further writing.
func (s *sha2_384Algo) SumHash() Hash {
	return s.Sum(nil)
}
