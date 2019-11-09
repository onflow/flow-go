package sctest

import (
	"crypto/rand"
	"io/ioutil"
	"testing"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/keys"
	"github.com/stretchr/testify/assert"
)

// ReadFile reads a file from the file system
func ReadFile(path string) []byte {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return contents
}

// GetNonce returns a nonce value that is guaranteed to be unique.
var GetNonce = func() func() uint64 {
	var nonce uint64
	return func() uint64 {
		nonce++
		return nonce
	}
}()

// randomKey returns a randomly generated private key
func randomKey() flow.AccountPrivateKey {
	seed := make([]byte, 40)
	rand.Read(seed)

	privateKey, err := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256, seed)
	if err != nil {
		panic(err)
	}

	return privateKey
}

// newEmulator returns a emulator object for testing
func newEmulator() *emulator.EmulatedBlockchain {
	return emulator.NewEmulatedBlockchain(emulator.Options{
		OnLogMessage: func(msg string) {},
	})
}

// SignAndSubmit signs a transaction with an array of signers and adds their signatures to the transaction
// Then submits the transaction to the emulator.  If the private keys don't match up with the addresses,
// the transaction will not succeed.
// shouldRevert parameter indicates whether the transaction should fail or not
// This function asserts the correct result and commits the block if it passed
func SignAndSubmit(tx flow.Transaction, b *emulator.EmulatedBlockchain, t *testing.T, signing_keys []flow.AccountPrivateKey, signing_addresses []flow.Address, shouldRevert bool) {

	// add array of signers to transaction
	for i := 0; i < len(signing_addresses); i++ {
		sig, err := keys.SignTransaction(tx, signing_keys[i])
		assert.Nil(t, err)

		tx.AddSignature(signing_addresses[i], sig)
	}

	// submit the signed transaction
	err := b.SubmitTransaction(tx)

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
