package crypto

import "github.com/ethereum/go-ethereum/common/hexutil"

// sha3_256Algo, embeds HashAlgo
//-----------------------------------------------
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

// Encoder should be implemented by structures to be hashed
//----------------------
// Encoder is an interface of a generic structure
type Encoder interface {
	Encode() []byte
}

// Hash type tools
//----------------------
// Hash is the hash algorithms output types
type Hash interface {
	// ToBytes returns the bytes representation of a hash
	ToBytes() []byte
	// String returns a Hex string representation of the hash bytes
	String() string
	// IsEqual tests an equality with a given hash
	IsEqual(Hash) bool
}

// Hash32 implements Hash
//----------------------
func (h *Hash32) ToBytes() []byte {
	return h[:]
}

func (h *Hash32) String() string {
	return hexutil.Encode(h[:])
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
