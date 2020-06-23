package hash

import (
	"hash"

	"golang.org/x/crypto/sha3"
)

// sha3_256Algo, embeds commonHasher
type sha3_256Algo struct {
	*commonHasher
	hash.Hash
}

// NewSHA3_256 returns a new instance of SHA3-256 hasher
func NewSHA3_256() Hasher {
	return &sha3_256Algo{
		commonHasher: &commonHasher{
			algo:       SHA3_256,
			outputSize: HashLenSha3_256},
		Hash: sha3.New256()}
}

// ComputeHash calculates and returns the SHA3-256 output of input byte array
func (s *sha3_256Algo) ComputeHash(data []byte) Hash {
	s.Reset()
	_, _ = s.Write(data)
	return s.Sum(nil)
}

// SumHash returns the SHA3-256 output and resets the hash state
func (s *sha3_256Algo) SumHash() Hash {
	digest := s.Sum(nil)
	s.Reset()
	return digest
}

// sha3_384Algo, embeds commonHasher
type sha3_384Algo struct {
	*commonHasher
	hash.Hash
}

// NewSHA3_384 returns a new instance of SHA3-384 hasher
func NewSHA3_384() Hasher {
	return &sha3_384Algo{
		commonHasher: &commonHasher{
			algo:       SHA3_384,
			outputSize: HashLenSha3_384},
		Hash: sha3.New384()}
}

// ComputeHash calculates and returns the SHA3-256 output of input byte array
func (s *sha3_384Algo) ComputeHash(data []byte) Hash {
	s.Reset()
	_, _ = s.Write(data)
	return s.Sum(nil)
}

// SumHash returns the SHA3-256 output and resets the hash state
func (s *sha3_384Algo) SumHash() Hash {
	digest := s.Sum(nil)
	s.Reset()
	return digest
}
