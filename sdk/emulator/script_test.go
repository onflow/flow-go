package emulator_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator"
)

func TestCallScript(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	accountAddress := b.RootAccountAddress()

	tx := &flow.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       accountAddress,
		ScriptAccounts:     []flow.Address{accountAddress},
	}

	tx.AddSignature(accountAddress, b.RootKey())

	callScript := fmt.Sprintf(sampleCall, accountAddress)

	// Sample call (value is 0)
	value, err := b.CallScript([]byte(callScript))
	assert.Nil(t, err)
	assert.Equal(t, big.NewInt(0), value)

	// Submit tx1 (script adds 2)
	err = b.SubmitTransaction(tx)
	assert.Nil(t, err)

	// Sample call (value is 2)
	value, err = b.CallScript([]byte(callScript))
	assert.Nil(t, err)
	assert.Equal(t, big.NewInt(2), value)
}
