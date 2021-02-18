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

type flowAccount struct {
	Address              *flowsdk.Address
	AccountPrivateKeyHex string // TODO change me to an slice
	accountKeys          []*flowsdk.AccountKey
	signer               crypto.InMemorySigner
	flowClient           *client.Client
	txTracker            *TxTracker
	logger               zerolog.Logger
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

func (acc *flowAccount) RevertSeq(keyID int) {
	acc.signerLock.Lock()
	defer acc.signerLock.Unlock()
	acc.accountKeys[keyID].SequenceNumber--
}

func (acc *flowAccount) SyncAccountKey() error {
	acc.signerLock.Lock()
	defer acc.signerLock.Unlock()

	account, err := acc.flowClient.GetAccount(context.Background(), *acc.Address)
	if err != nil {
		return fmt.Errorf("error while calling get account: %w", err)
	}
	acc.accountKeys = account.Keys
	return nil
}

func (acc *flowAccount) AddKeys(numberOfKeysToAdd int) error {

	acc.logger.Info().Msgf("adding %d keys to account %s", numberOfKeysToAdd, acc.Address.String())

	// add keys in batches
	batchSize := 50
	steps := numberOfKeysToAdd / batchSize
	for i := 0; i < steps; i++ {
		blockRef, err := acc.flowClient.GetLatestBlockHeader(context.Background(), false)
		if err != nil {
			return err
		}

		cadenceKeys := make([]cadence.Value, batchSize)
		for i := 0; i < batchSize; i++ {
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

		_, err = acc.SendTxAndWait(tx, 0)
		// TODO handler result
		if err != nil {
			return fmt.Errorf("error adding account keys: %w", err)
		}
	}
	// resync
	acc.SyncAccountKey()
	return nil
}

// TODO add batch send and share the same waiting group

func (acc *flowAccount) SendTxAndWait(tx *flowsdk.Transaction, keyIndex int) (*flowsdk.TransactionResult, error) {

	var result *flowsdk.TransactionResult
	var err error

	acc.logger.Info().Msgf("tx sent from account %s and key %d , next seq number %d", acc.Address, keyIndex, acc.accountKeys[keyIndex].SequenceNumber)

	err = acc.PrepareAndSignTx(tx, keyIndex)
	if err != nil {
		return nil, fmt.Errorf("error preparing and signing the transaction: %w", err)
	}

	err = acc.flowClient.SendTransaction(context.Background(), *tx)
	if err != nil {
		return nil, fmt.Errorf("error sending the transaction: %w", err)

	}

	stopped := false
	wg := sync.WaitGroup{}
	wg.Add(1)
	acc.txTracker.AddTx(tx.ID(),
		nil,
		func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
			acc.logger.Trace().Str("tx_id", tx.ID().String()).Msg("finalized tx")
			if !stopped {
				stopped = true
				result = res
				wg.Done()
			}
		}, // on finalized
		func(_ flowsdk.Identifier, res *flowsdk.TransactionResult) {
			acc.logger.Trace().Str("tx_id", tx.ID().String()).Msg("sealed tx")
			if !stopped {
				stopped = true
				result = res
				wg.Done()
			}
		}, // on sealed
		func(_ flowsdk.Identifier) {
			acc.logger.Warn().Str("tx_id", tx.ID().String()).Msg("tx expired")
			if !stopped {
				stopped = true
				wg.Done()
			}
		}, // on expired
		func(_ flowsdk.Identifier) {
			acc.logger.Warn().Str("tx_id", tx.ID().String()).Msg("tx timed out")
			if !stopped {
				stopped = true
				wg.Done()
			}
		}, // on timout
		func(_ flowsdk.Identifier, e error) {
			acc.logger.Error().Err(err).Str("tx_id", tx.ID().String()).Msgf("tx error")
			if !stopped {
				stopped = true
				// acc.RevertSeq(keyIndex)
				err = e
				wg.Done()
			}
		}, // on error
		600)
	wg.Wait()

	return result, err
}

func (acc *flowAccount) ToJSON() ([]byte, error) {
	b, err := json.Marshal(acc)
	return b, err
}

func newFlowAccount(address *flowsdk.Address,
	accountPrivateKeyHex string,
	accountKeys []*flowsdk.AccountKey,
	signer crypto.InMemorySigner,
	flowClient *client.Client,
	txTracker *TxTracker,
	logger zerolog.Logger,
) (*flowAccount, error) {

	if len(accountKeys) == 0 {
		return nil, fmt.Errorf("at least one account key is needed: given: %d", len(accountKeys))
	}

	privateKey, err := crypto.DecodePrivateKeyHex(accountKeys[0].SigAlgo, accountPrivateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("error while decoding account private key hex: %w", err)
	}
	signer = crypto.NewInMemorySigner(privateKey, accountKeys[0].HashAlgo)

	// TODO no need to pass signer here
	return &flowAccount{
		Address:              address,
		AccountPrivateKeyHex: accountPrivateKeyHex,
		accountKeys:          accountKeys,
		signer:               signer,
		signerLock:           sync.Mutex{},
		flowClient:           flowClient,
		txTracker:            txTracker,
		logger:               logger,
	}, nil
}

// TODO fix me
func newFlowAccountFromJSON(jsonStr []byte,
	flowClient *client.Client,
	txTracker *TxTracker,
	logger zerolog.Logger,
) (*flowAccount, error) {

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
