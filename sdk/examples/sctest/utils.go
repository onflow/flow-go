package sctest

import (
	"io/ioutil"
	"testing"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/keys"
	"github.com/stretchr/testify/assert"
)

func readFile(path string) []byte {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return contents
}

// Returns a nonce value that is guaranteed to be unique.
var getNonce = func() func() uint64 {
	var nonce uint64
	return func() uint64 {
		nonce++
		return nonce
	}
}()

// Returns a emulator object for testing
func newEmulator() *emulator.EmulatedBlockchain {
	return emulator.NewEmulatedBlockchain(emulator.EmulatedBlockchainOptions{
		OnLogMessage: func(msg string) {},
	})
}

// Signs a transaction with the Root Key and adds the signature to the transaction
// Then submits the transaction to the emulator
// shouldRevert parameter indicates whether the transaction should fail or not
// This function asserts the correct result and commits the block if it passed
func signAndSubmit(tx flow.Transaction, b *emulator.EmulatedBlockchain, t *testing.T, shouldRevert bool) {

	sig, err := keys.SignTransaction(tx, b.RootKey())
	assert.Nil(t, err)

	tx.AddSignature(b.RootAccountAddress(), sig)

	err = b.SubmitTransaction(tx)

	if shouldRevert {
		if assert.Error(t, err) {
			assert.IsType(t, &emulator.ErrTransactionReverted{}, err)
		}
	} else {
		if !assert.Nil(t, err) {
			t.Log(err.Error())
		}
		b.CommitBlock()
	}
}
