package keys

import (
	"github.com/dapperlabs/flow-go/pkg/crypto"
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

// SigningAlgorithm returns the hashing algorithm for this key type.
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

func DecodePrivateKey(keyType KeyType, b []byte) (types.AccountPrivateKey, error) {
	privateKey, err := crypto.DecodePrivateKey(keyType.SigningAlgorithm(), b)
	if err != nil {
		return types.AccountPrivateKey{}, err
	}

	return types.AccountPrivateKey{
		PrivateKey: privateKey,
		SignAlgo:   keyType.SigningAlgorithm(),
		HashAlgo:   keyType.HashingAlgorithm(),
	}, nil
}

func DecodePublicKey(keyType KeyType, weight int, b []byte) (types.AccountPublicKey, error) {
	publicKey, err := crypto.DecodePublicKey(keyType.SigningAlgorithm(), b)
	if err != nil {
		return types.AccountPublicKey{}, err
	}

	return types.AccountPublicKey{
		PublicKey: publicKey,
		SignAlgo:  keyType.SigningAlgorithm(),
		HashAlgo:  keyType.HashingAlgorithm(),
		Weight:    weight,
	}, nil
}
