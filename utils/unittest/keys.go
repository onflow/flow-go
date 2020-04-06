package unittest

import (
	"crypto/rand"

	"github.com/dapperlabs/flow-go/crypto"
)

func NetworkingKey() (crypto.PrivateKey, error) {
	seed := make([]byte, crypto.KeyGenSeedMinLenECDSA_P256)
	n, err := rand.Read(seed)
	if err != nil || n != crypto.KeyGenSeedMinLenECDSA_P256 {
		return nil, err
	}

	sk, err := crypto.GeneratePrivateKey(crypto.ECDSA_P256, seed)
	return sk, err
}

func NetworkingKeys(n int) ([]crypto.PrivateKey, error) {
	keys := make([]crypto.PrivateKey, 0, n)

	for i := 0; i < n; i++ {
		key, err := NetworkingKey()
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}

	return keys, nil
}

func StakingKey() (crypto.PrivateKey, error) {
	seed := make([]byte, crypto.KeyGenSeedMinLenBLS_BLS12381)
	n, err := rand.Read(seed)
	if err != nil || n != crypto.KeyGenSeedMinLenBLS_BLS12381 {
		return nil, err
	}

	sk, err := crypto.GeneratePrivateKey(crypto.BLS_BLS12381, seed)
	return sk, err
}

func StakingKeys(n int) ([]crypto.PrivateKey, error) {
	keys := make([]crypto.PrivateKey, 0, n)

	for i := 0; i < n; i++ {
		key, err := StakingKey()
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}

	return keys, nil
}
