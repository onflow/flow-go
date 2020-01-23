package crypto

import "hash"

// sha2_256Algo, embeds commonHasher
type sha2_256Algo struct {
	*commonHasher
	hash.Hash
}

// ComputeHash calculates and returns the SHA2-256 output of input byte array
func (s *sha2_256Algo) ComputeHash(data []byte) Hash {
	s.Reset()
	s.Write(data)
	digest := make(Hash, 0, HashLenSha2_256)
	return s.Sum(digest)
}

// SumHash returns the SHA2-256 output and resets the hash state
func (s *sha2_256Algo) SumHash() Hash {
	digest := make(Hash, HashLenSha2_256)
	s.Sum(digest[:0])
	s.Reset()
	return digest
}

// Add adds data to the state data to be hashed
/*func (s *sha2_256Algo) Add(data []byte) {
	s.Write(data)
}*/

// sha2_384Algo, embeds commonHasher
type sha2_384Algo struct {
	*commonHasher
	hash.Hash
}

// ComputeHash calculates and returns the SHA2-384 output of input byte array
func (s *sha2_384Algo) ComputeHash(data []byte) Hash {
	s.Reset()
	s.Write(data)
	digest := make(Hash, 0, HashLenSha2_384)
	return s.Sum(digest)
}

// SumHash returns the SHA2-384 output and resets the hash state
func (s *sha2_384Algo) SumHash() Hash {
	digest := make(Hash, HashLenSha2_384)
	s.Sum(digest[:0])
	s.Reset()
	return digest
}

// Add adds data to the state data to be hashed
/*func (s *sha2_384Algo) Add(data []byte) {
	s.Write(data)
}*/
