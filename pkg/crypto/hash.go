package crypto

import (
	"hash"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/sha3"
)

// NewHashAlgo initializes and chooses a hashing algorithm
func NewHashAlgo(name AlgoName) Hasher {
	if name == SHA3_256 {
		s := &(sha3_256Algo{&HashAlgo{name, HashLengthSha3_256, sha3.New256()}})
		// Output length sanity check
		if s.outputLength != s.Size() {
			log.Errorf("%s requires an output length %d", SHA3_256, s.Size())
			return nil
		}
		return s
	}
	log.Errorf("the hashing algorithm %s is not supported.", name)
	return nil
}

// Hasher interface

type Hasher interface {
	Name() AlgoName
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
type HashAlgo struct {
	name         AlgoName
	outputLength int
	hash.Hash
}

// Name returns the name of the algorithm
func (s *HashAlgo) Name() AlgoName {
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
