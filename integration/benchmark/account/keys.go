package account

import (
	"fmt"
	"sync/atomic"

	flowsdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go-sdk/crypto"
)

var ErrNoKeysAvailable = fmt.Errorf("no keys available")

type accountKey struct {
	*flowsdk.AccountKey

	ks      *keystore
	Address *flowsdk.Address
	Signer  crypto.InMemorySigner

	locked atomic.Bool
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
		ks.size++
	}
	ks.availableKeys = availableKeys

	return ks
}

func (k *keystore) Size() int {
	return k.size
}

func (k *keystore) getKey() (*accountKey, error) {
	select {
	case key := <-k.availableKeys:
		if !key.locked.CompareAndSwap(false, true) {
			panic("key is already locked")
		}

		return key, nil
	default:
		return nil, ErrNoKeysAvailable
	}
}

// Done unlocks a key after use
func (k *accountKey) Done() {
	k.locked.Store(false)
	k.ks.availableKeys <- k
}

// Success is called when a key was successfully used to sign a transaction as the proposer
// It increments the sequence number and releases key's lock
func (k *accountKey) Success() {
	if !k.locked.Load() {
		panic("key not locked")
	}

	k.SequenceNumber++
}

func (k *accountKey) SignPayload(tx *flowsdk.Transaction) error {
	return tx.SignPayload(*k.Address, k.Index, k.Signer)
}

func (k *accountKey) SignTx(tx *flowsdk.Transaction) error {
	if len(tx.Authorizers) == 0 {
		tx = tx.AddAuthorizer(*k.Address)
	}

	return tx.
		SetProposalKey(*k.Address, k.Index, k.SequenceNumber).
		SetPayer(*k.Address).
		SignEnvelope(*k.Address, k.Index, k.Signer)
}
