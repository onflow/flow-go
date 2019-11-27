package emulator_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/keys"
)

func TestExecuteScript(t *testing.T) {
	b, err := emulator.NewEmulatedBlockchain()
	require.NoError(t, err)

	accountAddress := b.RootAccountAddress()

	tx := flow.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       accountAddress,
		ScriptAccounts:     []flow.Address{accountAddress},
	}

	sig, err := keys.SignTransaction(tx, b.RootKey())
	assert.NoError(t, err)

	tx.AddSignature(accountAddress, sig)

	callScript := fmt.Sprintf(sampleCall, accountAddress)

	// Sample call (value is 0)
	value, _, err := b.ExecuteScript([]byte(callScript))
	assert.NoError(t, err)
	assert.Equal(t, values.NewInt(0), value)

	// Submit tx1 (script adds 2)
	err = b.SubmitTransaction(tx)
	assert.NoError(t, err)

	// Sample call (value is 2)
	value, _, err = b.ExecuteScript([]byte(callScript))
	assert.NoError(t, err)
	assert.Equal(t, values.NewInt(2), value)
}

func TestExecuteScriptAtBlockNumber(t *testing.T) {
	// TODO
	// Test that scripts can be executed at different block heights
}
