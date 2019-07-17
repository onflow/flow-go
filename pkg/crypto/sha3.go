package crypto

import "strconv"

// sha3_256Algo, embeds HashAlgo
type sha3_256Algo struct {
	*HashAlgo
}

// ComputeBytesHash calculates and returns the SHA3-256 output of input byte array
func (s *sha3_256Algo) ComputeBytesHash(data []byte) Hash {
	s.Reset()
	s.Write(data)
	var digest Hash32
	s.Sum(digest[:0])
	return &digest
}

// ComputeStructHash calculates and returns the SHA3-256 output of any input structure
func (s *sha3_256Algo) ComputeStructHash(struc Encoder) Hash {
	s.Reset()
	s.Write(struc.Encode())
	var digest Hash32
	s.Sum(digest[:0])
	return &digest
}

// SumHash returns the SHA3-256 output and resets the hash state
func (s *sha3_256Algo) SumHash() Hash {
	var digest Hash32
	s.Sum(digest[:0])
	s.Reset()
	return &digest
}

// Hash32 implements Hash
func (h *Hash32) ToBytes() []byte {
	return h[:]
}

func (h *Hash32) String() string {
	var s string
	for i := 0; i < len(h); i++ {
		s = strconv.FormatUint(uint64(h[i]), 16) + s
	}
	return "0x" + s
}

func (h *Hash32) IsEqual(input Hash) bool {
	inputBytes := input.ToBytes()
	if len(h) != len(inputBytes) {
		return false
	}
	for i := 0; i < len(h); i++ {
		if h[i] != inputBytes[i] {
			return false
		}
	}
	return true
}
