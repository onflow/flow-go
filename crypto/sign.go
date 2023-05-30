// Package crypto ...
package crypto

import (
	"crypto/elliptic"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"

	"github.com/onflow/flow-go/crypto/hash"
)

// revive:disable:var-naming

// revive:enable

// SigningAlgorithm is an identifier for a signing algorithm
// (and parameters if applicable)
type SigningAlgorithm int

const (
	// Supported signing algorithms
	UnknownSigningAlgorithm SigningAlgorithm = iota
	// BLSBLS12381 is BLS on BLS 12-381 curve
	BLSBLS12381
	// ECDSAP256 is ECDSA on NIST P-256 curve
	ECDSAP256
	// ECDSASecp256k1 is ECDSA on secp256k1 curve
	ECDSASecp256k1
)

// String returns the string representation of this signing algorithm.
func (f SigningAlgorithm) String() string {
	return [...]string{"UNKNOWN", "BLS_BLS12381", "ECDSA_P256", "ECDSA_secp256k1"}[f]
}

// Signature is a generic type, regardless of the signature scheme
type Signature []byte

// Signer interface
type signer interface {
	// generatePrivateKey generates a private key
	generatePrivateKey([]byte) (PrivateKey, error)
	// decodePrivateKey loads a private key from a byte array
	decodePrivateKey([]byte) (PrivateKey, error)
	// decodePublicKey loads a public key from a byte array
	decodePublicKey([]byte) (PublicKey, error)
	// decodePublicKeyCompressed loads a public key from a byte array representing a point in compressed form
	decodePublicKeyCompressed([]byte) (PublicKey, error)
}

// newSigner returns a signer instance
func newSigner(algo SigningAlgorithm) (signer, error) {
	switch algo {
	case ECDSAP256:
		return p256Instance, nil
	case ECDSASecp256k1:
		return secp256k1Instance, nil
	case BLSBLS12381:
		return blsInstance, nil
	default:
		return nil, invalidInputsErrorf("the signature scheme %s is not supported", algo)
	}
}

// Initialize the context of all algos
func init() {
	// ECDSA
	p256Instance = &(ecdsaAlgo{
		curve: elliptic.P256(),
		algo:  ECDSAP256,
	})
	secp256k1Instance = &(ecdsaAlgo{
		curve: btcec.S256(),
		algo:  ECDSASecp256k1,
	})

	// BLS
	initBLS12381()
	blsInstance = &blsBLS12381Algo{
		algo: BLSBLS12381,
	}
}

// SignatureFormatCheck verifies the format of a serialized signature,
// regardless of messages or public keys.
//
// This function is only defined for ECDSA algos for now.
//
// If SignatureFormatCheck returns false then the input is not a valid
// signature and will fail a verification against any message and public key.
func SignatureFormatCheck(algo SigningAlgorithm, s Signature) (bool, error) {
	switch algo {
	case ECDSAP256:
		return p256Instance.signatureFormatCheck(s), nil
	case ECDSASecp256k1:
		return secp256k1Instance.signatureFormatCheck(s), nil
	default:
		return false, invalidInputsErrorf(
			"the signature scheme %s is not supported",
			algo)
	}
}

// GeneratePrivateKey generates a private key of the algorithm using the entropy of the given seed.
//
// The seed minimum length is 32 bytes and it should have enough entropy.
// It is recommended to use a secure crypto RNG to generate the seed.
//
// The function returns:
//   - (false, invalidInputsErrors) if the signing algorithm is not supported or
//     if the seed length is not valid (less than 32 bytes or larger than 256 bytes)
//   - (false, error) if an unexpected error occurs
//   - (sk, nil) if key generation was successful
func GeneratePrivateKey(algo SigningAlgorithm, seed []byte) (PrivateKey, error) {
	signer, err := newSigner(algo)
	if err != nil {
		return nil, fmt.Errorf("key generation failed: %w", err)
	}
	return signer.generatePrivateKey(seed)
}

// DecodePrivateKey decodes an array of bytes into a private key of the given algorithm
//
// The function returns:
//   - (nil, invalidInputsErrors) if the signing algorithm is not supported
//   - (nil, invalidInputsErrors) if the input does not serialize a valid private key:
//   - ECDSA: bytes(x) where bytes() is the big-endian encoding padded to the curve order.
//   - BLS: bytes(x) where bytes() is the big-endian encoding padded to the order of BLS12-381.
//     for all algorithms supported, input is big-endian encoding
//     of a the private scalar less than the curve order and left-padded to 32 bytes
//   - (nil, error) if an unexpected error occurs
//   - (sk, nil) otherwise
func DecodePrivateKey(algo SigningAlgorithm, data []byte) (PrivateKey, error) {
	signer, err := newSigner(algo)
	if err != nil {
		return nil, fmt.Errorf("decode private key failed: %w", err)
	}
	return signer.decodePrivateKey(data)
}

// DecodePublicKey decodes an array of bytes into a public key of the given algorithm
//
// The function returns:
//   - (nil, invalidInputsErrors) if the signing algorithm is not supported
//   - (nil, invalidInputsErrors) if the input does not serialize a valid public key:
//   - ECDSA: bytes(x)||bytes(y) where bytes() is the big-endian encoding padded to the field size.
//   - BLS: compressed serialization of a G2 point following https://www.ietf.org/archive/id/draft-irtf-cfrg-pairing-friendly-curves-08.html#name-zcash-serialization-format-
//   - (nil, error) if an unexpected error occurs
//   - (sk, nil) otherwise
func DecodePublicKey(algo SigningAlgorithm, data []byte) (PublicKey, error) {
	signer, err := newSigner(algo)
	if err != nil {
		return nil, fmt.Errorf("decode public key failed: %w", err)
	}
	return signer.decodePublicKey(data)
}

// DecodePublicKeyCompressed decodes an array of bytes given in a compressed representation into a public key of the given algorithm
// Only ECDSA is supported (BLS uses the compressed serialzation by default).
//
// The function returns:
//   - (nil, invalidInputsErrors) if the signing algorithm is not supported (is not ECDSA)
//   - (nil, invalidInputsErrors) if the input does not serialize a valid public key:
//   - ECDSA: sign_byte||bytes(x) according to X9.62 section 4.3.6.
//   - (nil, error) if an unexpected error occurs
//   - (sk, nil) otherwise
func DecodePublicKeyCompressed(algo SigningAlgorithm, data []byte) (PublicKey, error) {
	signer, err := newSigner(algo)
	if err != nil {
		return nil, fmt.Errorf("decode public key failed: %w", err)
	}
	return signer.decodePublicKeyCompressed(data)
}

// Signature type tools

// Bytes returns a byte array of the signature data
func (s Signature) Bytes() []byte {
	return s[:]
}

// String returns a String representation of the signature data
func (s Signature) String() string {
	return fmt.Sprintf("%#x", s.Bytes())
}

// Key Pair

// PrivateKey is an unspecified signature scheme private key
type PrivateKey interface {
	// Algorithm returns the signing algorithm related to the private key.
	Algorithm() SigningAlgorithm
	// Size return the key size in bytes.
	Size() int
	// String return a hex representation of the key
	String() string
	// Sign generates a signature using the provided hasher.
	Sign([]byte, hash.Hasher) (Signature, error)
	// PublicKey returns the public key.
	PublicKey() PublicKey
	// Encode returns a bytes representation of the private key
	Encode() []byte
	// Equals returns true if the given PrivateKeys are equal. Keys are considered unequal if their algorithms are
	// unequal or if their encoded representations are unequal. If the encoding of either key fails, they are considered
	// unequal as well.
	Equals(PrivateKey) bool
}

// PublicKey is an unspecified signature scheme public key.
type PublicKey interface {
	// Algorithm returns the signing algorithm related to the public key.
	Algorithm() SigningAlgorithm
	// Size() return the key size in bytes.
	Size() int
	// String return a hex representation of the key
	String() string
	// Verify verifies a signature of an input message using the provided hasher.
	Verify(Signature, []byte, hash.Hasher) (bool, error)
	// Encode returns a bytes representation of the public key.
	Encode() []byte
	// EncodeCompressed returns a compressed byte representation of the public key.
	// The compressed serialization concept is generic to elliptic curves,
	// but we refer to individual curve parameters for details of the compressed format
	EncodeCompressed() []byte
	// Equals returns true if the given PublicKeys are equal. Keys are considered unequal if their algorithms are
	// unequal or if their encoded representations are unequal. If the encoding of either key fails, they are considered
	// unequal as well.
	Equals(PublicKey) bool
}
