package crypto

import (
	"hash"

	"github.com/ethereum/go-ethereum/common/hexutil"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/sha3"
)

// InitHashAlgo initializes and chooses a hashing algorithm
//-----------------------------------------------
func InitHashAlgo(name string) Hasher {
	if name == "SHA3_256" {
		s := &(sha3_256Algo{&HashAlgo{name, HashLengthSha3_256, sha3.New256()}})
		// Output length sanity check
		if s.outputLength != s.Size() {
			log.Errorf("%x requires an output length %d", name, s.Size())
			return nil
		} else {
			return s
		}
	} else {
		log.Errorf("the hashing algorithm %x is not supported", name)
		return nil
	}
}

// Hasher interface
//-----------------------------------------------
type Hasher interface {
	GetName() string
	// Size return the hash output length (a Hash.hash method)
	Size() int
	// Compute hash
	ComputeBytesHash([]byte) Hash
	ComputeStructHash(Encoder) Hash
	// Write adds more bytes to the current hash state (a Hash.hash method)
	AddBytes([]byte)
	AddStruct(Encoder)
	// SumHash returns the hash output and resets the hash state a
	SumHash() Hash
	// Reset resets the hash state
	Reset()
}

// HashAlgo implements Hasher
//-----------------------------------------------
type HashAlgo struct {
	name         string
	outputLength int
	hash.Hash
}

// GetName returns the name of the algorithm
func (s *HashAlgo) GetName() string {
	return s.name
}

// ComputeBytesHash is an obsolete function that gets overritten
func (s *HashAlgo) ComputeBytesHash([]byte) Hash {
	var h Hash
	return h
}

// ComputeStructHash is an obsolete function that gets overritten
func (s *HashAlgo) ComputeStructHash(struc Encoder) Hash {
	var h Hash
	return h
}

// AddBytes adds bytes to the current hash state
func (s *HashAlgo) AddBytes(data []byte) {
	s.Write(data)
}

// AddStruct adds a structure to the current hash state
func (s *HashAlgo) AddStruct(struc Encoder) {
	s.Write(struc.Encode())
}

// SumHash is an obsolete function that gets overritten
func SumHash() Hash {
	var h Hash
	return h
}

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
	ToBytes() []byte
	String() string
	IsEqual(Hash) bool
}

// Hash32 implements Hash
//----------------------

// ToBytes returns the byte representation of a hash.
func (h *Hash32) ToBytes() []byte {
	return h[:]
}

// String implements the stringer interface
func (h *Hash32) String() string {
	return hexutil.Encode(h[:])
}

// IsEqual checks if a hash is equal to an input hash
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
