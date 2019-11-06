package sctest

import (
	"io/ioutil"
	"testing"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/keys"
	"github.com/stretchr/testify/assert"
)

func ReadFile(path string) []byte {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return contents
}

// Returns a nonce value that is guaranteed to be unique.
var GetNonce = func() func() uint64 {
	var nonce uint64
	return func() uint64 {
		nonce++
		return nonce
	}
}()

// func getAddressFromEvent(event flow.Event) flow.Address {
// 	addressI := event.Values["address"].([]interface{})
// 	addressBytes := make([]byte, len(addressI))
// 	for i, addressByte := range addressI {
// 		addressBytes[i] = byte(addressByte.(float64))
// 	}
// 	return flow.BytesToAddress(addressBytes)
// }

// func waitForSeal(ctx context.Context, c *client.Client, hash crypto.Hash) *flow.Transaction {
// 	txResp, err := c.GetTransaction(ctx, hash)
// 	if err != nil {
// 		panic(err)
// 	}
// 	for txResp.Status != flow.TransactionSealed {
// 		time.Sleep(time.Second)
// 		fmt.Print(".")
// 		txResp, err = c.GetTransaction(ctx, hash)
// 		if err != nil {
// 			panic(err)
// 		}
// 	}
// 	fmt.Println()
// 	return txResp
// }

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
func SignAndSubmit(tx flow.Transaction, b *emulator.EmulatedBlockchain, t *testing.T, shouldRevert bool) {

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
