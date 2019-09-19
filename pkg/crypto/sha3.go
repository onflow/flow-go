package crypto

import (
	"strconv"
	"strings"
)

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
func (h *Hash32) Bytes() []byte {
	return h[:]
}

func (h *Hash32) String() string {
	var sb strings.Builder
	sb.WriteString("0x")
	for _, i := range h {
		sb.WriteString(strconv.FormatUint(uint64(i), 16))
	}
	return sb.String()
}

func (h *Hash32) IsEqual(input Hash) bool {
	inputBytes := input.Bytes()
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

// sha3_256Algo, embeds HashAlgo
type sha3_384Algo struct {
	*HashAlgo
}

// ComputeBytesHash calculates and returns the SHA3-256 output of input byte array
func (s *sha3_384Algo) ComputeBytesHash(data []byte) Hash {
	s.Reset()
	s.Write(data)
	digest := make([]byte, HashLengthSha3_384)
	s.Sum(digest[:0])
	return digest
}

// ComputeStructHash calculates and returns the SHA3-256 output of any input structure
func (s *sha3_384Algo) ComputeStructHash(struc Encoder) Hash {
	s.Reset()
	s.Write(struc.Encode())
	digest := make([]byte, HashLengthSha3_384)
	s.Sum(digest[:0])
	return digest
}

// SumHash returns the SHA3-256 output and resets the hash state
func (s *sha3_384Algo) SumHash() Hash {
	digest := make([]byte, HashLengthSha3_384)
	s.Sum(digest[:0])
	s.Reset()
	return digest
}
