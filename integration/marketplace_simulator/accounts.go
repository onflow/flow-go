package marketplace

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
)

// TODO move toward accountKeys instead of only one

type flowAccount struct {
	Address              *flowsdk.Address
	AccountPrivateKeyHex string
	accountKey           *flowsdk.AccountKey
	signer               crypto.InMemorySigner
	signerLock           sync.Mutex
}

func (acc *flowAccount) signTx(tx *flowsdk.Transaction, keyID int) error {
	acc.signerLock.Lock()
	defer acc.signerLock.Unlock()
	err := tx.SignEnvelope(*acc.Address, keyID, acc.signer)
	if err != nil {
		return err
	}
	acc.accountKey.SequenceNumber++
	return nil
}

func (acc *flowAccount) PrepareAndSignTx(tx *flowsdk.Transaction, keyID int) error {
	acc.signerLock.Lock()
	defer acc.signerLock.Unlock()

	tx.SetProposalKey(*acc.Address, 0, acc.accountKey.SequenceNumber).
		SetPayer(*acc.Address).
		AddAuthorizer(*acc.Address)

	err := tx.SignEnvelope(*acc.Address, keyID, acc.signer)
	if err != nil {
		return err
	}
	acc.accountKey.SequenceNumber++
	return nil
}

func (acc *flowAccount) SyncAccountKey(flowClient *client.Client) error {
	acc.signerLock.Lock()
	defer acc.signerLock.Unlock()

	account, err := flowClient.GetAccount(context.Background(), *acc.Address)
	if err != nil {
		return fmt.Errorf("error while calling get account: %w", err)
	}
	acc.accountKey = account.Keys[0]
	return nil
}

func (acc *flowAccount) ToJSON() ([]byte, error) {
	b, err := json.Marshal(acc)
	return b, err
}

func newFlowAccount(i int, address *flowsdk.Address, accountPrivateKeyHex string, accountKey *flowsdk.AccountKey, signer crypto.InMemorySigner) *flowAccount {
	return &flowAccount{
		Address:              address,
		AccountPrivateKeyHex: accountPrivateKeyHex,
		accountKey:           accountKey,
		signer:               signer,
		signerLock:           sync.Mutex{},
	}
}

func newFlowAccountFromJSON(jsonStr []byte, flowClient *client.Client) (*flowAccount, error) {

	var account flowAccount

	err := json.Unmarshal(jsonStr, &account)
	if err != nil {
		return nil, fmt.Errorf("error decoding account from json: %w", err)
	}

	acc, err := flowClient.GetAccount(context.Background(), *account.Address)
	if err != nil {
		return nil, fmt.Errorf("error while calling get account: %w", err)
	}
	account.accountKey = acc.Keys[0]

	privateKey, err := crypto.DecodePrivateKeyHex(account.accountKey.SigAlgo, account.AccountPrivateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("error while decoding account private key hex: %w", err)
	}

	account.signer = crypto.NewInMemorySigner(privateKey, account.accountKey.HashAlgo)
	account.signerLock = sync.Mutex{}

	return &account, nil
}
