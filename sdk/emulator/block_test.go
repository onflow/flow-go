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

	// Submit tx1
	err = b.SubmitTransaction(tx1)
	assert.NoError(t, err)

	tx, err := b.GetTransaction(tx1.Hash())
	assert.NoError(t, err)

	// Commit tx1 into new block
	_, err = b.CommitBlock()
	assert.NoError(t, err)

	assert.Equal(t, flow.TransactionFinalized, tx.Status)

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

	// Submit invalid tx2
	err = b.SubmitTransaction(tx2)
	assert.NotNil(t, err)

	tx, err = b.GetTransaction(tx2.Hash())
	assert.NoError(t, err)

	assert.Equal(t, flow.TransactionReverted, tx.Status)

	// Commit tx2 into new block
	_, err = b.CommitBlock()
	assert.NoError(t, err)

	// tx1 status becomes TransactionSealed
	tx, err = b.GetTransaction(tx1.Hash())
	require.Nil(t, err)
	assert.Equal(t, flow.TransactionSealed, tx.Status)

	// tx2 status stays TransactionReverted
	tx, err = b.GetTransaction(tx2.Hash())
	require.Nil(t, err)
	assert.Equal(t, flow.TransactionReverted, tx.Status)
}
