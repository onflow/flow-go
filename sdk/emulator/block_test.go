package emulator_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/keys"
)

func TestCommitBlock(t *testing.T) {
	b, err := emulator.NewEmulatedBlockchain()
	require.NoError(t, err)

	addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

	tx1 := flow.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []flow.Address{b.RootAccountAddress()},
	}

	sig, err := keys.SignTransaction(tx1, b.RootKey())
	assert.NoError(t, err)

	tx1.AddSignature(b.RootAccountAddress(), sig)

	// Add tx1 to pending block
	err = b.AddTransaction(tx1)
	assert.NoError(t, err)

	tx, err := b.GetTransaction(tx1.Hash())
	assert.NoError(t, err)

	assert.Equal(t, flow.TransactionPending, tx.Status)

	tx2 := flow.Transaction{
		Script:             []byte("invalid script"),
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []flow.Address{b.RootAccountAddress()},
	}

	sig, err = keys.SignTransaction(tx2, b.RootKey())
	assert.NoError(t, err)

	tx2.AddSignature(b.RootAccountAddress(), sig)

	// Add tx2 to pending block
	err = b.AddTransaction(tx2)
	assert.NoError(t, err)

	tx, err = b.GetTransaction(tx2.Hash())
	assert.NoError(t, err)

	assert.Equal(t, flow.TransactionPending, tx.Status)

	// Execute tx1
	result, err := b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assert.True(t, result.Succeeded())

	// Execute tx2
	result, err = b.ExecuteNextTransaction()
	assert.NoError(t, err)
	assert.True(t, result.Reverted())

	// Commit tx1 and tx2 into new block
	_, err = b.CommitBlock()
	assert.NoError(t, err)

	// tx1 status becomes TransactionSealed
	tx, err = b.GetTransaction(tx1.Hash())
	require.NoError(t, err)
	assert.Equal(t, flow.TransactionSealed, tx.Status)

	// tx2 status stays TransactionReverted
	tx, err = b.GetTransaction(tx2.Hash())
	require.NoError(t, err)
	assert.Equal(t, flow.TransactionReverted, tx.Status)
}
