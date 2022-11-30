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
	Address *flowsdk.Address
	ID      int

	keys   *keystore
	signer crypto.InMemorySigner
}

func New(i int, address *flowsdk.Address, accountKeys []*flowsdk.AccountKey, signer crypto.InMemorySigner) *FlowAccount {
	keys := make([]*accountKey, 0, len(accountKeys))
	for _, key := range accountKeys {
		keys = append(keys, &accountKey{
			AccountKey: key,
			Address:    address,
			Signer:     signer,
		})
	}

	return &FlowAccount{
		Address: address,
		ID:      i,
		keys:    newKeystore(keys),
		signer:  signer,
	}
}

func LoadServiceAccount(
	ctx context.Context,
	flowClient access.Client,
	servAccAddress *flowsdk.Address,
	servAccPrivKeyHex string,
) (*FlowAccount, error) {
	acc, err := flowClient.GetAccount(ctx, *servAccAddress)
	if err != nil {
		return nil, fmt.Errorf("error while calling get account for service account: %w", err)
	}

	privateKey, err := crypto.DecodePrivateKeyHex(acc.Keys[0].SigAlgo, servAccPrivKeyHex)
	if err != nil {
		return nil, fmt.Errorf("error while decoding serice account private key hex: %w", err)
	}

	signer, err := crypto.NewInMemorySigner(privateKey, acc.Keys[0].HashAlgo)
	if err != nil {
		return nil, fmt.Errorf("error while creating signer: %w", err)
	}

	return New(0, servAccAddress, acc.Keys, signer), nil
}

func (acc *FlowAccount) NumKeys() int {
	return acc.keys.Size()
}

func (acc *FlowAccount) GetKey() (*accountKey, error) {
	return acc.keys.getKey()
}

// randomPrivateKey returns a randomly generated ECDSA P-256 private key.
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
