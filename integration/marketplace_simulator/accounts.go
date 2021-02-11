package marketplace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/cadence"
	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/rs/zerolog"
)

// TODO move toward accountKeys instead of only one

type flowAccount struct {
	Address              *flowsdk.Address
	AccountPrivateKeyHex string // TODO change me to an slice
	accountKeys          []*flowsdk.AccountKey
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
	acc.accountKeys[keyID].SequenceNumber++
	return nil
}

func (acc *flowAccount) PrepareAndSignTx(tx *flowsdk.Transaction, keyID int) error {
	acc.signerLock.Lock()
	defer acc.signerLock.Unlock()

	tx.SetProposalKey(*acc.Address, keyID, acc.accountKeys[keyID].SequenceNumber).
		SetPayer(*acc.Address).
		AddAuthorizer(*acc.Address)

	err := tx.SignEnvelope(*acc.Address, keyID, acc.signer)
	if err != nil {
		return err
	}
	acc.accountKeys[keyID].SequenceNumber++
	return nil
}

func (acc *flowAccount) SyncAccountKey(flowClient *client.Client) error {
	acc.signerLock.Lock()
	defer acc.signerLock.Unlock()

	account, err := flowClient.GetAccount(context.Background(), *acc.Address)
	if err != nil {
		return fmt.Errorf("error while calling get account: %w", err)
	}
	acc.accountKeys = account.Keys
	return nil
}

func (acc *flowAccount) AddKeys(log zerolog.Logger, txTracker *TxTracker, flowClient *client.Client, numberOfKeysToAdd int) error {

	blockRef, err := flowClient.GetLatestBlockHeader(context.Background(), false)
	if err != nil {
		return err
	}

	cadenceKeys := make([]cadence.Value, numberOfKeysToAdd)
	for i := 0; i < numberOfKeysToAdd; i++ {
		cadenceKeys[i] = bytesToCadenceArray(acc.accountKeys[0].Encode())
	}
	cadenceKeysArray := cadence.NewArray(cadenceKeys)

	addKeysScript, err := AddKeyToAccountScript()
	if err != nil {
		return errors.New("error getting add key to account script")
	}

	tx := flowsdk.NewTransaction().
		SetScript(addKeysScript).
		SetReferenceBlockID(blockRef.ID)

	err = tx.AddArgument(cadenceKeysArray)
	if err != nil {
		return errors.New("error constructing add keys to account transaction")
	}

	acc.PrepareAndSignTx(tx, 0)

	if err != nil {
		return fmt.Errorf("error preparing and signing the transaction: %w", err)
	}

	err = flowClient.SendTransaction(context.Background(), *tx)
	if err != nil {
		return fmt.Errorf("error sending the transaction: %w", err)

	}

	var result *flowsdk.TransactionResult

	stopped := false
	wg := sync.WaitGroup{}
	wg.Add(1)
	txTracker.AddTx(tx.ID(),
		nil,
		nil, // on finalized
		func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
			log.Debug().Str("tx_id", tx.ID().String()).Msgf("finalized tx")
			result = res
			if !stopped {
				stopped = true
				wg.Done()
			}
		}, // on sealed
		func(_ flowsdk.Identifier) {
			log.Error().Str("tx_id", tx.ID().String()).Msgf("tx expired")
			if !stopped {
				stopped = true
				wg.Done()
			}
		}, // on expired
		func(_ flowsdk.Identifier) {
			log.Error().Str("tx_id", tx.ID().String()).Msgf("tx timed out")
			if !stopped {
				stopped = true
				wg.Done()
			}
		}, // on timout
		func(_ flowsdk.Identifier, e error) {
			log.Error().Err(err).Str("tx_id", tx.ID().String()).Msgf("tx error")
			if !stopped {
				stopped = true
				err = e
				wg.Done()
			}
		}, // on error
		360)
	wg.Wait()

	// resync
	account, err := flowClient.GetAccount(context.Background(), *acc.Address)
	if err != nil {
		return fmt.Errorf("error while calling get account: %w", err)
	}
	acc.accountKeys = account.Keys

	return nil
}

func (acc *flowAccount) ToJSON() ([]byte, error) {
	b, err := json.Marshal(acc)
	return b, err
}

func newFlowAccount(i int, address *flowsdk.Address, accountPrivateKeyHex string, accountKeys []*flowsdk.AccountKey, signer crypto.InMemorySigner) *flowAccount {
	return &flowAccount{
		Address:              address,
		AccountPrivateKeyHex: accountPrivateKeyHex,
		accountKeys:          accountKeys,
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
	account.accountKeys = acc.Keys

	privateKey, err := crypto.DecodePrivateKeyHex(account.accountKeys[0].SigAlgo, account.AccountPrivateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("error while decoding account private key hex: %w", err)
	}

	account.signer = crypto.NewInMemorySigner(privateKey, account.accountKeys[0].HashAlgo)
	account.signerLock = sync.Mutex{}

	return &account, nil
}
