// Package crypto ...
package crypto

import (
	"crypto/elliptic"
	"fmt"

	"github.com/btcsuite/btcd/btcec"

	"github.com/onflow/flow-go/crypto/hash"
)

// revive:disable:var-naming

// revive:enable

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

// newNonRelicSigner returns a signer that does not depend on Relic library.
func newNonRelicSigner(algo SigningAlgorithm) (signer, error) {
	switch algo {
	case ECDSAP256:
		return p256Instance, nil
	case ECDSASecp256k1:
		return secp256k1Instance, nil
	default:
		return nil, newInvalidInputsError("the signature scheme %s is not supported", algo)
	}
}

// Initialize the context of all algos not requiring Relic
func initNonRelic() {
	// P-256
	p256Instance = &(ecdsaAlgo{
		curve: elliptic.P256(),
		algo:  ECDSAP256,
	})

	// secp256k1
	secp256k1Instance = &(ecdsaAlgo{
		curve: btcec.S256(),
		algo:  ECDSASecp256k1,
	})
}

// Signature format Check for non-relic algos (ECDSA)
func signatureFormatCheckNonRelic(algo SigningAlgorithm, s Signature) (bool, error) {
	switch algo {
	case ECDSAP256:
		return p256Instance.signatureFormatCheck(s), nil
	case ECDSASecp256k1:
		return secp256k1Instance.signatureFormatCheck(s), nil
	default:
		return false, newInvalidInputsError(
			"the signature scheme %s is not supported",
			algo)
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
	// For now, signatureFormatCheckNonRelic is only defined for non-Relic algos.
	return signatureFormatCheckNonRelic(algo, s)
}

// GeneratePrivateKey generates a private key of the algorithm using the entropy of the given seed.
//
// It is recommended to use a secure crypto RNG to generate the seed.
// The seed must have enough entropy and should be sampled uniformly at random.
func GeneratePrivateKey(algo SigningAlgorithm, seed []byte) (PrivateKey, error) {
	signer, err := newSigner(algo)
	if err != nil {
		return nil, newInvalidInputsError("key generation failed: %s", err)
	}
	return signer.generatePrivateKey(seed)
}

// DecodePrivateKey decodes an array of bytes into a private key of the given algorithm
func DecodePrivateKey(algo SigningAlgorithm, data []byte) (PrivateKey, error) {
	signer, err := newSigner(algo)
	if err != nil {
		return nil, newInvalidInputsError("decode private key failed: %s", err)
	}
	return signer.decodePrivateKey(data)
}

// DecodePublicKey decodes an array of bytes into a public key of the given algorithm
func DecodePublicKey(algo SigningAlgorithm, data []byte) (PublicKey, error) {
	signer, err := newSigner(algo)
	if err != nil {
		return nil, newInvalidInputsError("decode public key failed: %s", err)
	}
	return signer.decodePublicKey(data)
}

// DecodePublicKeyCompressed decodes an array of bytes given in a compressed representation into a public key of the given algorithm
func DecodePublicKeyCompressed(algo SigningAlgorithm, data []byte) (PublicKey, error) {
	signer, err := newSigner(algo)
	if err != nil {
		return nil, newInvalidInputsError("decode public key failed: %s", err)
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
	// Encode returns a compressed byte representation of the public key.
	// The compressed serialization concept is generic to elliptic curves,
	// but we refer to individual curve parameters for details of the compressed format
	EncodeCompressed() []byte
	// Equals returns true if the given PublicKeys are equal. Keys are considered unequal if their algorithms are
	// unequal or if their encoded representations are unequal. If the encoding of either key fails, they are considered
	// unequal as well.
	Equals(PublicKey) bool
}
