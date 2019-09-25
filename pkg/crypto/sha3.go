package crypto

// sha3_256Algo, embeds HashAlgo
type sha3_256Algo struct {
	*HashAlgo
}

// ComputeBytesHash calculates and returns the SHA3-256 output of input byte array
func (s *sha3_256Algo) ComputeBytesHash(data []byte) Hash {
	s.Reset()
	s.Write(data)
	digest := make(Hash, HashLengthSha3_256)
	s.Sum(digest[:0])
	return digest
}

// ComputeStructHash calculates and returns the SHA3-256 output of any input structure
func (s *sha3_256Algo) ComputeStructHash(struc Encoder) Hash {
	s.Reset()
	s.Write(struc.Encode())
	digest := make(Hash, HashLengthSha3_256)
	s.Sum(digest[:0])
	return digest
}

// SumHash returns the SHA3-256 output and resets the hash state
func (s *sha3_256Algo) SumHash() Hash {
	digest := make(Hash, HashLengthSha3_256)
	s.Sum(digest[:0])
	s.Reset()
	return digest
}

// sha3_256Algo, embeds HashAlgo
type sha3_384Algo struct {
	*HashAlgo
}

// ComputeBytesHash calculates and returns the SHA3-256 output of input byte array
func (s *sha3_384Algo) ComputeBytesHash(data []byte) Hash {
	s.Reset()
	s.Write(data)
	digest := make(Hash, HashLengthSha3_384)
	s.Sum(digest[:0])
	return digest
}

// ComputeStructHash calculates and returns the SHA3-256 output of any input structure
func (s *sha3_384Algo) ComputeStructHash(struc Encoder) Hash {
	s.Reset()
	s.Write(struc.Encode())
	digest := make(Hash, HashLengthSha3_384)
	s.Sum(digest[:0])
	return digest
}

// SumHash returns the SHA3-256 output and resets the hash state
func (s *sha3_384Algo) SumHash() Hash {
	digest := make(Hash, HashLengthSha3_384)
	s.Sum(digest[:0])
	s.Reset()
	return digest
}
