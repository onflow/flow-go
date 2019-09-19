package crypto

import (
	"hash"

	"golang.org/x/crypto/sha3"
)

// NewHashAlgo initializes and chooses a hashing algorithm
func NewHashAlgo(name AlgoName) (Hasher, error) {
	if name == SHA3_256 {
		a := &(sha3_256Algo{&HashAlgo{name, HashLengthSha3_256, sha3.New256()}})
		// Output length sanity check, size() is provided by Hash.hash
		if a.outputSize != a.Size() {
			return nil, cryptoError{string(SHA3_256) + " requires an output length " + string(a.Size())}
		}
		return a, nil
	}
	if name == SHA3_384 {
		a := &(sha3_384Algo{&HashAlgo{name, HashLengthSha3_384, sha3.New384()}})
		// Output length sanity check, size() is provided by Hash.hash
		if a.outputSize != a.Size() {
			return nil, cryptoError{string(SHA3_384) + " requires an output length " + string(a.Size())}
		}
		return a, nil
	}
	return nil, cryptoError{"the hashing algorithm " + string(name) + " is not supported."}
}

// Hash is the hash algorithms output types
type Hash []byte

// Hash32 implements Hash
func (h Hash) Bytes() []byte {
	return h
}

func (h Hash) IsEqual(input Hash) bool {
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

// Hasher interface

type Hasher interface {
	Name() AlgoName
	// Size return the hash output length
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
	name       AlgoName
	outputSize int
	hash.Hash
}

// Name returns the name of the algorithm
func (a *HashAlgo) Name() AlgoName {
	return a.name
}

// Name returns the size of the output
func (a *HashAlgo) Size() int {
	return a.outputSize
}

// AddBytes adds bytes to the current hash state
func (a *HashAlgo) AddBytes(data []byte) {
	a.Write(data)
}

// AddStruct adds a structure to the current hash state
func (a *HashAlgo) AddStruct(struc Encoder) {
	a.Write(struc.Encode())
}

func BytesToHash(b []byte) Hash {
	h := make([]byte, len(b))
	copy(h, b)
	return h
}

// HashesToBytes converts a slice of hashes to a slice of byte slices.
func HashesToBytes(hashes []Hash) [][]byte {
	b := make([][]byte, len(hashes))

	for i, h := range hashes {
		b[i] = h.Bytes()
	}

	return b
}
