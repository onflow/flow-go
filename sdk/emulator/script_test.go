package emulator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/keys"
)

func TestExecuteScript(t *testing.T) {
	b, err := emulator.NewBlockchain()
	require.NoError(t, err)

	addTwoScript, counterAddress := deployAndGenerateAddTwoScript(t, b)

	accountAddress := b.RootAccountAddress()

	tx := flow.Transaction{TransactionBody: flow.TransactionBody{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       accountAddress,
		ScriptAccounts:     []flow.Address{accountAddress},
	}}

	sig, err := keys.SignTransaction(tx, b.RootKey())
	assert.NoError(t, err)

	tx.AddSignature(accountAddress, sig)

	callScript := generateGetCounterCountScript(counterAddress, accountAddress)

	// Sample call (value is 0)
	scriptResult, err := b.ExecuteScript([]byte(callScript))
	require.NoError(t, err)
	assert.Equal(t, values.NewInt(0), scriptResult.Value)

	// Submit tx (script adds 2)
	err = b.AddTransaction(tx)
	assert.NoError(t, err)

	txResult, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assert.True(t, txResult.Succeeded())

	t.Run("BeforeCommit", func(t *testing.T) {
		t.Skip("TODO: fix stored ledger")

		// Sample call (value is still 0)
		result, err := b.ExecuteScript([]byte(callScript))
		require.NoError(t, err)
		assert.Equal(t, values.NewInt(0), result.Value)
	})

	_, err = b.CommitBlock()
	assert.NoError(t, err)

	t.Run("AfterCommit", func(t *testing.T) {
		// Sample call (value is 2)
		result, err := b.ExecuteScript([]byte(callScript))
		require.NoError(t, err)
		assert.Equal(t, values.NewInt(2), result.Value)
	})
}

func TestExecuteScriptAtBlockNumber(t *testing.T) {
	// TODO
	// Test that scripts can be executed at different block heights
}
