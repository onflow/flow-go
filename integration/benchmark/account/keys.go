package account

import (
	"errors"
	"fmt"
	"github.com/onflow/cadence"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/integration/benchmark/common"
	"github.com/onflow/flow-go/integration/benchmark/scripts"
	"github.com/rs/zerolog"
	"sync"

	flowsdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go-sdk/crypto"
)

var ErrNoKeysAvailable = fmt.Errorf("no keys available")

type AccountKey struct {
	flowsdk.AccountKey

	mu      sync.Mutex
	ks      *keystore
	Address flowsdk.Address
	Signer  crypto.Signer
	inuse   bool
}

type keystore struct {
	availableKeys chan *AccountKey
	size          int
}

func newKeystore(keys []*AccountKey) *keystore {
	ks := &keystore{}

	availableKeys := make(chan *AccountKey, len(keys))
	for _, key := range keys {
		key.ks = ks
		availableKeys <- key
	}
	ks.size = len(keys)
	ks.availableKeys = availableKeys

	return ks
}

func (k *keystore) Size() int {
	return k.size
}

func (k *keystore) getKey() (*AccountKey, error) {
	select {
	case key := <-k.availableKeys:
		key.mu.Lock()
		defer key.mu.Unlock()

		if key.inuse {
			return nil, fmt.Errorf("key already in use")
		}
		key.inuse = true

		return key, nil
	default:
		return nil, ErrNoKeysAvailable
	}
}

func (k *AccountKey) markUnused() {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.inuse = false
}

// Done unlocks a key after use and puts it back into the pool.
func (k *AccountKey) Done() {
	k.markUnused()
	k.ks.availableKeys <- k
}

// IncrementSequenceNumber is called when a key was successfully used to sign a transaction as the proposer.
// It increments the sequence number.
func (k *AccountKey) IncrementSequenceNumber() {
	k.mu.Lock()
	defer k.mu.Unlock()

	if !k.inuse {
		panic("assert: key not locked")
	}
	k.SequenceNumber++
}

func (k *AccountKey) SignPayload(tx *flowsdk.Transaction) error {
	return tx.SignPayload(k.Address, k.Index, k.Signer)
}

func (k *AccountKey) SetProposerPayerAndSign(tx *flowsdk.Transaction) error {
	if len(tx.Authorizers) == 0 {
		tx = tx.AddAuthorizer(k.Address)
	}

	return tx.
		SetProposalKey(k.Address, k.Index, k.SequenceNumber).
		SetPayer(k.Address).
		SignEnvelope(k.Address, k.Index, k.Signer)
}

func EnsureAccountHasKeys(
	log zerolog.Logger,
	account *FlowAccount,
	num int,
	referenceBlockProvider common.ReferenceBlockProvider,
	sender common.TransactionSender,
) error {
	if account.NumKeys() >= num {
		return nil
	}

	numberOfKeysToAdd := num - account.NumKeys()

	return AddKeysToAccount(log, account, numberOfKeysToAdd, referenceBlockProvider, sender)
}

func AddKeysToAccount(log zerolog.Logger,
	account *FlowAccount,
	numberOfKeysToAdd int,
	referenceBlockProvider common.ReferenceBlockProvider,
	sender common.TransactionSender,
) error {
	key, err := account.GetKey()
	if err != nil {
		return err
	}
	defer key.Done()

	wrapErr := func(err error) error {
		return fmt.Errorf("error adding keys to account %s: %w", account.Address, err)
	}
	accountKeys := make([]flowsdk.AccountKey, numberOfKeysToAdd)
	for i := 0; i < numberOfKeysToAdd; i++ {
		accountKey := key.AccountKey
		accountKey.Index = i + account.NumKeys()
		accountKey.SequenceNumber = 0
		accountKeys[i] = accountKey
	}

	cadenceKeys := make([]cadence.Value, numberOfKeysToAdd)
	for i := 0; i < numberOfKeysToAdd; i++ {
		cadenceKeys[i] = blueprints.BytesToCadenceArray(accountKeys[i].PublicKey.Encode())
	}
	cadenceKeysArray := cadence.NewArray(cadenceKeys)

	addKeysTx := flowsdk.NewTransaction().
		SetScript(scripts.AddKeysToAccountTransaction).
		AddAuthorizer(account.Address).
		SetReferenceBlockID(referenceBlockProvider.ReferenceBlockID())

	err = addKeysTx.AddArgument(cadenceKeysArray)
	if err != nil {
		return err
	}

	err = key.SetProposerPayerAndSign(addKeysTx)
	if err != nil {
		return wrapErr(err)
	}

	_, err = sender.Send(addKeysTx)
	if err == nil || errors.Is(err, common.TransactionError{}) {
		key.IncrementSequenceNumber()
	}
	if err != nil {
		return wrapErr(err)
	}

	return nil
}
