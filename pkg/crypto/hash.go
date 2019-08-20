package crypto

import (
	"hash"

	"golang.org/x/crypto/sha3"
)

// NewHashAlgo initializes and chooses a hashing algorithm
func NewHashAlgo(name AlgoName) (Hasher, error) {
	if name == SHA3_256 {
		a := &(sha3_256Algo{&HashAlgo{name, HashLengthSha3_256, sha3.New256()}})
		// Output length sanity check
		if a.outputLength != a.Size() {
			return nil, cryptoError{string(SHA3_256) + " requires an output length " + string(a.Size())}
		}
		return a, nil
	}
	return nil, cryptoError{"the hashing algorithm " + string(name) + " is not supported."}
}

// Hash is the hash algorithms output types

type Hash interface {
	// Bytes returns the bytes representation of a hash
	Bytes() []byte
	// String returns a Hex string representation of the hash bytes in big endian
	String() string
	// IsEqual tests an equality with a given hash
	IsEqual(Hash) bool
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

// HashAlgo
type HashAlgo struct {
	name         AlgoName
	outputLength int
	hash.Hash
}

// Name returns the name of the algorithm
func (a *HashAlgo) Name() AlgoName {
	return a.name
}

// AddBytes adds bytes to the current hash state
func (a *HashAlgo) AddBytes(data []byte) {
	a.Write(data)
}

// AddStruct adds a structure to the current hash state
func (a *HashAlgo) AddStruct(struc Encoder) {
	a.Write(struc.Encode())
}
