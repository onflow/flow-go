package utils

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"

	flowsdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go-sdk/access"
	"github.com/onflow/flow-go-sdk/crypto"
)

type flowAccount struct {
	i          int
	address    *flowsdk.Address
	accountKey *flowsdk.AccountKey
	seqNumber  uint64
	signer     crypto.InMemorySigner
	signerLock sync.Mutex
}

func (acc *flowAccount) signCreateAccountTx(createAccountTx *flowsdk.Transaction) error {
	acc.signerLock.Lock()
	defer acc.signerLock.Unlock()

	err := createAccountTx.SignEnvelope(
		*acc.address,
		acc.accountKey.Index,
		acc.signer,
	)
	if err != nil {
		return err
	}

	acc.accountKey.SequenceNumber++
	return nil
}

func (acc *flowAccount) signTx(tx *flowsdk.Transaction, keyID int) error {
	acc.signerLock.Lock()
	defer acc.signerLock.Unlock()
	err := tx.SignEnvelope(*acc.address, keyID, acc.signer)
	if err != nil {
		return err
	}
	acc.seqNumber++
	return nil
}

func newFlowAccount(i int, address *flowsdk.Address, accountKey *flowsdk.AccountKey, signer crypto.InMemorySigner) *flowAccount {
	return &flowAccount{
		i:          i,
		address:    address,
		accountKey: accountKey,
		signer:     signer,
		seqNumber:  uint64(0),
		signerLock: sync.Mutex{},
	}
}

func loadServiceAccount(flowClient access.Client,
	servAccAddress *flowsdk.Address,
	servAccPrivKeyHex string) (*flowAccount, error) {

	acc, err := flowClient.GetAccount(context.Background(), *servAccAddress)
	if err != nil {
		return nil, fmt.Errorf("error while calling get account for service account: %w", err)
	}
	accountKey := acc.Keys[0]

	privateKey, err := crypto.DecodePrivateKeyHex(accountKey.SigAlgo, servAccPrivKeyHex)
	if err != nil {
		return nil, fmt.Errorf("error while decoding serice account private key hex: %w", err)
	}

	signer, err := crypto.NewInMemorySigner(privateKey, accountKey.HashAlgo)
	if err != nil {
		return nil, fmt.Errorf("error while creating signer: %w", err)
	}

	return &flowAccount{
		address:    servAccAddress,
		accountKey: accountKey,
		seqNumber:  accountKey.SequenceNumber,
		signer:     signer,
		signerLock: sync.Mutex{},
	}, nil
}

// randomPrivateKey returns a randomly generated ECDSA P-256 private key.
func randomPrivateKey() crypto.PrivateKey {
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
