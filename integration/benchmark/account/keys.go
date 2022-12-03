package account

import (
	"fmt"
	"sync"

	flowsdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go-sdk/crypto"
)

var ErrNoKeysAvailable = fmt.Errorf("no keys available")

type accountKey struct {
	flowsdk.AccountKey

	mu      sync.Mutex
	ks      *keystore
	Address *flowsdk.Address
	Signer  crypto.InMemorySigner
	inuse   bool
}

type keystore struct {
	availableKeys chan *accountKey
	size          int
}

func newKeystore(keys []*accountKey) *keystore {
	ks := &keystore{}

	availableKeys := make(chan *accountKey, len(keys))
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

func (k *keystore) getKey() (*accountKey, error) {
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

func (k *accountKey) markUnused() {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.inuse = false
}

// Done unlocks a key after use and puts it back into the pool.
func (k *accountKey) Done() {
	k.markUnused()
	k.ks.availableKeys <- k
}

// IncrementSequenceNumber is called when a key was successfully used to sign a transaction as the proposer.
// It increments the sequence number.
func (k *accountKey) IncrementSequenceNumber() {
	k.mu.Lock()
	defer k.mu.Unlock()

	if !k.inuse {
		panic("assert: key not locked")
	}
	k.SequenceNumber++
}

func (k *accountKey) SignPayload(tx *flowsdk.Transaction) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	return tx.SignPayload(*k.Address, k.Index, k.Signer)
}

func (k *accountKey) SignTx(tx *flowsdk.Transaction) error {
	k.mu.Lock()
	defer k.mu.Unlock()

	if len(tx.Authorizers) == 0 {
		tx = tx.AddAuthorizer(*k.Address)
	}

	return tx.
		SetProposalKey(*k.Address, k.Index, k.SequenceNumber).
		SetPayer(*k.Address).
		SignEnvelope(*k.Address, k.Index, k.Signer)
}
