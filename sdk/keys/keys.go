// Package keys provides utilities for generating, encoding, and decoding Flow account keys.
package keys

import (
	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator/constants"
)

// KeyType is a key format supported by Flow.
type KeyType int

const (
	UnknownKeyType KeyType = iota
	ECDSA_P256_SHA2_256
	ECDSA_P256_SHA3_256
	ECDSA_SECp256k1_SHA2_256
	ECDSA_SECp256k1_SHA3_256
)

// SigningAlgorithm returns the signing algorithm for this key type.
func (k KeyType) SigningAlgorithm() crypto.SigningAlgorithm {
	switch k {
	case ECDSA_P256_SHA2_256, ECDSA_P256_SHA3_256:
		return crypto.ECDSA_P256
	case ECDSA_SECp256k1_SHA2_256, ECDSA_SECp256k1_SHA3_256:
		return crypto.ECDSA_SECp256k1
	default:
		return crypto.UnknownSigningAlgorithm
	}
}

// HashingAlgorithm returns the hashing algorithm for this key type.
func (k KeyType) HashingAlgorithm() crypto.HashingAlgorithm {
	switch k {
	case ECDSA_P256_SHA2_256, ECDSA_SECp256k1_SHA2_256:
		return crypto.SHA2_256
	case ECDSA_P256_SHA3_256, ECDSA_SECp256k1_SHA3_256:
		return crypto.SHA3_256
	default:
		return crypto.UnknownHashingAlgorithm
	}
}

const KeyWeightThreshold = constants.AccountKeyWeightThreshold

// GeneratePrivateKey generates a private key of the specified key type.
func GeneratePrivateKey(keyType KeyType, seed []byte) (flow.AccountPrivateKey, error) {
	privateKey, err := crypto.GeneratePrivateKey(keyType.SigningAlgorithm(), seed)
	if err != nil {
		return flow.AccountPrivateKey{}, err
	}

	return flow.AccountPrivateKey{
		PrivateKey: privateKey,
		SignAlgo:   keyType.SigningAlgorithm(),
		HashAlgo:   keyType.HashingAlgorithm(),
	}, nil
}

// EncodePrivateKey encodes a private key as bytes.
func EncodePrivateKey(a flow.AccountPrivateKey) ([]byte, error) {
	return encoding.DefaultEncoder.EncodeAccountPrivateKey(a)
}

// DecodePrivateKey decodes a private key.
func DecodePrivateKey(b []byte) (flow.AccountPrivateKey, error) {
	return encoding.DefaultEncoder.DecodeAccountPrivateKey(b)
}

// EncodePublicKey encodes a public key as bytes.
func EncodePublicKey(a flow.AccountPublicKey) ([]byte, error) {
	return encoding.DefaultEncoder.EncodeAccountPublicKey(a)
}

// DecodePublicKey decodes a public key.
func DecodePublicKey(b []byte) (flow.AccountPublicKey, error) {
	return encoding.DefaultEncoder.DecodeAccountPublicKey(b)
}

// SignTransaction signs a transaction with a private key.
func SignTransaction(
	tx flow.Transaction,
	privateKey flow.AccountPrivateKey,
) (crypto.Signature, error) {
	hasher, err := crypto.NewHasher(privateKey.HashAlgo)
	if err != nil {
		return nil, err
	}

	b, err := encoding.DefaultEncoder.EncodeTransaction(tx)
	if err != nil {
		return nil, err
	}

	return privateKey.PrivateKey.Sign(b, hasher)
}

// ValidateEncodedPublicKey returns an error if the bytes do not represent a valid public key.
func ValidateEncodedPublicKey(b []byte) error {
	publicKey, err := flow.DecodeAccountPublicKey(b)
	if err != nil {
		return errors.Wrap(err, "invalid public key encoding")
	}

	return ValidatePublicKey(publicKey)
}

// ValidatePublicKey returns an error if the public key is invalid.
func ValidatePublicKey(publicKey flow.AccountPublicKey) error {
	if !CompatibleAlgorithms(publicKey.SignAlgo, publicKey.HashAlgo) {
		return errors.Errorf(
			"signing algorithm (%s) is incompatible with hashing algorithm (%s)",
			publicKey.SignAlgo,
			publicKey.HashAlgo,
		)
	}

	return nil
}

// CompatibleAlgorithms returns true if the given signing and hashing algorithms are compatible.
func CompatibleAlgorithms(signAlgo crypto.SigningAlgorithm, hashAlgo crypto.HashingAlgorithm) bool {
	t := map[crypto.SigningAlgorithm]map[crypto.HashingAlgorithm]bool{
		crypto.ECDSA_P256: {
			crypto.SHA2_256: true,
			crypto.SHA3_256: true,
		},
		crypto.ECDSA_SECp256k1: {
			crypto.SHA2_256: true,
			crypto.SHA3_256: true,
		},
	}

	return t[signAlgo][hashAlgo]
}
