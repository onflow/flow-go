package crypto

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"

	"crypto/sha256"
	"crypto/sha512"

	"golang.org/x/crypto/sha3"
)

// NewHasher initializes and chooses a hashing algorithm
func NewHasher(algo HashingAlgorithm) (Hasher, error) {
	switch algo {
	case SHA3_256:
		hasher := &sha3_256Algo{
			commonHasher: &commonHasher{
				algo:       algo,
				outputSize: HashLenSha3_256},
			Hash: sha3.New256()}

		// Output length sanity check, size() is provided by Hash.hash
		if hasher.outputSize != hasher.Hash.Size() {
			return nil, cryptoError{
				fmt.Sprintf("%s requires an output length %d", SHA3_256, hasher.Hash.Size()),
			}
		}
		return hasher, nil

	case SHA3_384:
		hasher := &sha3_384Algo{
			commonHasher: &commonHasher{
				algo:       algo,
				outputSize: HashLenSha3_384},
			Hash: sha3.New384()}
		// Output length sanity check, size() is provided by Hash.hash
		if hasher.outputSize != hasher.Hash.Size() {
			return nil, cryptoError{
				fmt.Sprintf("%s requires an output length %d", SHA3_384, hasher.Hash.Size()),
			}
		}
		return hasher, nil

	case SHA2_256:
		hasher := &sha2_256Algo{
			commonHasher: &commonHasher{
				algo:       algo,
				outputSize: HashLenSha2_256},
			Hash: sha256.New()}

		// Output length sanity check, size() is provided by Hash.hash
		if hasher.outputSize != hasher.Hash.Size() {
			return nil, cryptoError{
				fmt.Sprintf("%s requires an output length %d", SHA2_256, hasher.Hash.Size()),
			}
		}
		return hasher, nil

	case SHA2_384:
		hasher := &sha2_384Algo{
			commonHasher: &commonHasher{
				algo:       algo,
				outputSize: HashLenSha2_384},
			Hash: sha512.New384()}

		// Output length sanity check, size() is provided by Hash.hash
		if hasher.outputSize != hasher.Hash.Size() {
			return nil, cryptoError{
				fmt.Sprintf("%s requires an output length %d", SHA2_384, hasher.Hash.Size()),
			}
		}
		return hasher, nil

	/*case CSHAKE_128:
	var definer, customizer []byte
	var size int
	if params != nil {
		if params.definer != nil {
			copy(definer, params.definer)
		}
		if params.customizer != nil {
			copy(customizer, params.customizer)
		}
		size = params.outputSize
	}
	hasher := &cShake128Algo{
		commonHasher: &commonHasher{
			algo:       algo,
			outputSize: size,
		},
		ShakeHash: sha3.NewCShake128(definer, customizer)}
	return hasher, nil*/

	default:
		return nil, cryptoError{
			fmt.Sprintf("the hashing algorithm %s is not supported.", algo),
		}
	}
}

// Hash is the hash algorithms output types
type Hash []byte

// Equal checks if a hash is equal to a given hash
func (h Hash) Equal(input Hash) bool {
	return bytes.Equal(h, input)
}

// Hex returns the hex string representation of the hash.
func (h Hash) Hex() string {
	return hex.EncodeToString(h)
}

// Hasher interface

type Hasher interface {
	// Algorithm returns the hashing algorithm for this hasher.
	Algorithm() HashingAlgorithm
	// Size returns the hash output length
	Size() int
	// ComputeHash returns the hash output regardless of the hash state
	ComputeHash([]byte) Hash
	// Write([]bytes) (using the io.Writer interface) adds more bytes to the
	// current hash state
	io.Writer
	// SumHash returns the hash output and resets the hash state
	SumHash() Hash
	// Reset resets the hash state
	Reset()
}

// Hasher interface

/*type MACer interface {
	// Algorithm returns the MAC algorithm
	Algorithm() MacAlgorithm
	// Size returns the MAC output length
	Size() int
	// Write([]bytes) (using the io.Writer interface) adds more
	// bytes to the current MAC state
	io.Writer
	// ComputeHash returns the MAC output of the input data
	// the MAC state is reinitialized with same key and same customization string
	ComputeHash(data []byte) Hash
	// Reset resets the MAC state and reinitializes it with
	// the same key and same customization string
	Reset()
}*/

// some hashers can be parameterized
// hasherParameters contains the parameters required when
// a Hasher is initialized
/*type hasherParameters struct {
	outputSize int
	definer    []byte
	customizer []byte
}*/

// commonHasher holds the common data for all hashers
type commonHasher struct {
	algo       HashingAlgorithm
	outputSize int
}

func (a *commonHasher) Algorithm() HashingAlgorithm {
	return a.algo
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
		b[i] = h
	}
	return b
}
