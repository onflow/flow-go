package crypto

// sha2_256Algo, embeds HashAlgo
type sha2_256Algo struct {
	*commonHasher
}

// ComputeHash calculates and returns the SHA2-256 output of input byte array
func (s *sha2_256Algo) ComputeHash(data []byte) Hash {
	s.Reset()
	s.Write(data)
	digest := make(Hash, HashLengthSha2_256)
	s.Sum(digest[:0])
	return digest
}

// SumHash returns the SHA2-256 output and resets the hash state
func (s *sha2_256Algo) SumHash() Hash {
	digest := make(Hash, HashLengthSha2_256)
	s.Sum(digest[:0])
	s.Reset()
	return digest
}

// sha2_384Algo, embeds HashAlgo
type sha2_384Algo struct {
	*commonHasher
}

// ComputeHash calculates and returns the SHA2-384 output of input byte array
func (s *sha2_384Algo) ComputeHash(data []byte) Hash {
	s.Reset()
	s.Write(data)
	digest := make(Hash, HashLengthSha2_384)
	s.Sum(digest[:0])
	return digest
}

// SumHash returns the SHA2-384 output and resets the hash state
func (s *sha2_384Algo) SumHash() Hash {
	digest := make(Hash, HashLengthSha2_384)
	s.Sum(digest[:0])
	s.Reset()
	return digest
}
