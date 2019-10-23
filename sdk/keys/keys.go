package keys

import (
	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/types"
)

type KeyType int

const (
	KeyTypeUnknown KeyType = iota
	KeyTypeECDSA_P256_SHA2_256
	KeyTypeECDSA_P256_SHA3_256
	KeyTypeECDSA_SECp256k1_SHA2_256
	KeyTypeECDSA_SECp256k1_SHA3_256
)

func (k KeyType) SigningAlgorithm() crypto.SigningAlgorithm {
	switch k {
	case KeyTypeECDSA_P256_SHA2_256, KeyTypeECDSA_P256_SHA3_256:
		return crypto.ECDSA_P256
	case KeyTypeECDSA_SECp256k1_SHA2_256, KeyTypeECDSA_SECp256k1_SHA3_256:
		return crypto.ECDSA_SECp256k1
	default:
		return crypto.UnknownSigningAlgorithm
	}
}

func (k KeyType) HashingAlgorithm() crypto.HashingAlgorithm {
	switch k {
	case KeyTypeECDSA_P256_SHA2_256, KeyTypeECDSA_SECp256k1_SHA2_256:
		return crypto.SHA2_256
	case KeyTypeECDSA_P256_SHA3_256, KeyTypeECDSA_SECp256k1_SHA3_256:
		return crypto.SHA3_256
	default:
		return crypto.UnknownHashingAlgorithm
	}
}

func GeneratePrivateKey(keyType KeyType, seed []byte) (*types.AccountPrivateKey, error) {
	privateKey, err := crypto.GeneratePrivateKey(keyType.SigningAlgorithm(), seed)
	if err != nil {
		return nil, err
	}

	return &types.AccountPrivateKey{
		PrivateKey: privateKey,
		SignAlgo:   keyType.SigningAlgorithm(),
		HashAlgo:   keyType.HashingAlgorithm(),
	}, nil
}

func DecodePrivateKey(keyType KeyType, b []byte) (*types.AccountPrivateKey, error) {
	privateKey, err := crypto.DecodePrivateKey(keyType.SigningAlgorithm(), b)
	if err != nil {
		return nil, err
	}

	return &types.AccountPrivateKey{
		PrivateKey: privateKey,
		SignAlgo:   keyType.SigningAlgorithm(),
		HashAlgo:   keyType.HashingAlgorithm(),
	}, nil
}

func DecodePublicKey(keyType KeyType, weight int, b []byte) (*types.AccountPublicKey, error) {
	publicKey, err := crypto.DecodePublicKey(keyType.SigningAlgorithm(), b)
	if err != nil {
		return nil, err
	}

	return &types.AccountPublicKey{
		PublicKey: publicKey,
		SignAlgo:  keyType.SigningAlgorithm(),
		HashAlgo:  keyType.HashingAlgorithm(),
		Weight:    weight,
	}, nil
}
