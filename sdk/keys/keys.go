// Package keys provides utilities for generating, encoding, and decoding Flow account keys.
package keys

import (
	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/encoding"
	"github.com/dapperlabs/flow-go/pkg/types"
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

// GeneratePrivateKey generates a private key of the specified key type.
func GeneratePrivateKey(keyType KeyType, seed []byte) (types.AccountPrivateKey, error) {
	privateKey, err := crypto.GeneratePrivateKey(keyType.SigningAlgorithm(), seed)
	if err != nil {
		return types.AccountPrivateKey{}, err
	}

	return types.AccountPrivateKey{
		PrivateKey: privateKey,
		SignAlgo:   keyType.SigningAlgorithm(),
		HashAlgo:   keyType.HashingAlgorithm(),
	}, nil
}

// EncodePrivateKey encodes a private key as bytes.
func EncodePrivateKey(a types.AccountPrivateKey) ([]byte, error) {
	return encoding.DefaultEncoder.EncodeAccountPrivateKey(a)
}

// DecodePrivateKey decodes a private key.
func DecodePrivateKey(b []byte) (types.AccountPrivateKey, error) {
	return encoding.DefaultEncoder.DecodeAccountPrivateKey(b)
}

// EncodePublicKey encodes a public key as bytes.
func EncodePublicKey(a types.AccountPublicKey) ([]byte, error) {
	return encoding.DefaultEncoder.EncodeAccountPublicKey(a)
}

// DecodePublicKey decodes a public key.
func DecodePublicKey(b []byte) (types.AccountPublicKey, error) {
	return encoding.DefaultEncoder.DecodeAccountPublicKey(b)
}

// SignTransaction signs a transaction with a private key.
func SignTransaction(
	tx types.Transaction,
	privateKey types.AccountPrivateKey,
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
