package account

import (
	"context"
	"crypto/rand"
	"fmt"

	flowsdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go-sdk/access"
	"github.com/onflow/flow-go-sdk/crypto"
)

type FlowAccount struct {
	Address    flowsdk.Address
	keys       *keystore
	PrivateKey crypto.PrivateKey
	HashAlgo   crypto.HashAlgorithm
}

func New(
	address flowsdk.Address,
	privateKey crypto.PrivateKey,
	hashAlgo crypto.HashAlgorithm,
	accountKeys []flowsdk.AccountKey,
) (*FlowAccount, error) {
	keys := make([]*AccountKey, 0, len(accountKeys))
	for _, key := range accountKeys {
		// signer are not thread safe, so we need to create a new signer for each key
		signer, err := crypto.NewInMemorySigner(privateKey, hashAlgo)
		if err != nil {
			return nil, fmt.Errorf("error while creating in-memory signer: %w", err)
		}

		keys = append(keys, &AccountKey{
			AccountKey: key,
			Address:    address,
			Signer:     signer,
		})
	}

	return &FlowAccount{
		Address:    address,
		keys:       newKeystore(keys),
		PrivateKey: privateKey,
		HashAlgo:   hashAlgo,
	}, nil
}

func LoadAccount(
	ctx context.Context,
	flowClient access.Client,
	address flowsdk.Address,
	privateKey crypto.PrivateKey,
	hashAlgo crypto.HashAlgorithm,
) (*FlowAccount, error) {
	acc, err := flowClient.GetAccount(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("error while calling get account for account %s: %w", address, err)
	}

	keys := make([]flowsdk.AccountKey, 0, len(acc.Keys))
	for i, key := range acc.Keys {
		keys[i] = *key
	}

	return New(address, privateKey, hashAlgo, keys)
}

func (acc *FlowAccount) NumKeys() int {
	return acc.keys.Size()
}

func (acc *FlowAccount) GetKey() (*AccountKey, error) {
	return acc.keys.getKey()
}

// RandomPrivateKey returns a randomly generated ECDSA P-256 private key.
func RandomPrivateKey() crypto.PrivateKey {
	seed := make([]byte, crypto.MinSeedLength)

	_, err := rand.Read(seed)
	if err != nil {
		panic(err)
	}

	privateKey, err := crypto.GeneratePrivateKey(crypto.ECDSA_P256, seed)
	if err != nil {
		panic(err)
	}

	return privateKey
}
