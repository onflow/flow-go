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
		s := sha3_256Algo{&HashAlgo{name, HashLengthSha3_256, sha3.New256()}}
		// Output length sanity check
		if s.outputLength != s.Size() {
			log.Error("the hashing algorithm requested is not supported")
		}
		return &s
	}
	log.Error("the hashing algorithm requested is not supported")
	return nil
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
	//h            hash.Hash
	hash.Hash
}

// GetName returns the name of the algorithm
func (s *HashAlgo) GetName() string {
	return s.name
}

// GetOutputLength returns the hash output length of the algorithm
/*func (s *HashAlgo) GetOutputLength() int {
	return s.outputLength
}*/

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
	/*s.h.Reset()
	s.h.Write(data)
	var digest Hash // 32
	s.h.Sum(digest[:0])
	return digest*/
	s.Reset()
	s.Write(data)
	var digest Hash // 32
	s.Sum(digest[:0])
	return digest
}

// ComputeStructHash calculates and returns the SHA3-256 output of any input structure
func (s *sha3_256Algo) ComputeStructHash(struc Encoder) Hash {
	/*s.h.Reset()
	s.h.Write(struc.Encode())
	var digest Hash // 32
	s.h.Sum(digest[:0])
	return digest*/
	s.Reset()
	s.Write(struc.Encode())
	var digest Hash // 32
	s.Sum(digest[:0])
	return digest
}

// SumHash returns the SHA3-256 output and resets the hash state
func (s *sha3_256Algo) SumHash() Hash {
	var digest Hash // 32
	s.Sum(digest[:0])
	s.Reset()
	return digest
}

// Encoder should be implemented by structures to be hashed
//----------------------
// Encoder is an interface of a generic structure
type Encoder interface {
	Encode() []byte
}

// Hash type tools
//----------------------

// BytesToHash sets b to hash.
// If b is larger than HashLength, b will be cropped from the right.
func BytesToHash(b []byte) Hash {
	if HashLength < len(b) {
		log.Warn("the array is cropped from the right")
	}
	var h Hash
	// number of copied bytes is min(len(b), HashLength)
	copy(h[0:], b)
	// clear the remaining bytes of h
	for i := len(b); i < HashLength; i++ {
		h[i] = 0
	}
	return h
}

// ToBytes sets a hash to b
// ToBytes gets the byte representation of the underlying hash.
func (h Hash) ToBytes() []byte {
	return h[:]
}

// Hex converts a hash to a hex string.
func (h Hash) Hex() string { return hexutil.Encode(h[:]) }

// String implements the stringer interface and is used also by the logger when
// doing full logging into a file.
func (h Hash) String() string {
	return h.Hex()
}

// IsEqual checks if a hash is equal to an input hash
func (h Hash) IsEqual(input *Hash) bool {
	return h.IsEqual(input)
}
