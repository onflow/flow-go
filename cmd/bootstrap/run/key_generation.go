package run

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
)

func GenerateNetworkingKey(seed []byte) (crypto.PrivateKey, error) {
	keys, err := GenerateKeys(crypto.EcdsaP256, 1, [][]byte{seed})
	if err != nil {
		return nil, err
	}
	return keys[0], nil
}

func GenerateNetworkingKeys(n int, seeds [][]byte) ([]crypto.PrivateKey, error) {
	return GenerateKeys(crypto.EcdsaP256, n, seeds)
}

func GenerateStakingKey(seed []byte) (crypto.PrivateKey, error) {
	keys, err := GenerateKeys(crypto.BlsBls12381, 1, [][]byte{seed})
	if err != nil {
		return nil, err
	}
	return keys[0], nil
}

func GenerateStakingKeys(n int, seeds [][]byte) ([]crypto.PrivateKey, error) {
	return GenerateKeys(crypto.BlsBls12381, n, seeds)
}

func GenerateKeys(algo crypto.SigningAlgorithm, n int, seeds [][]byte) ([]crypto.PrivateKey, error) {
	if n != len(seeds) {
		return nil, fmt.Errorf("n needs to match the number of seeds (%v != %v)", n, len(seeds))
	}

	keys := make([]crypto.PrivateKey, n)

	var err error
	for i, seed := range seeds {
		if keys[i], err = crypto.GeneratePrivateKey(algo, seed); err != nil {
			return nil, err
		}
	}

	return keys, nil
}
