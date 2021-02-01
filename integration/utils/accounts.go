package utils

import (
	"sync"

	flowsdk "github.com/onflow/flow-go-sdk"

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
