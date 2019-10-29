package emulator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator"
)

func TestCommitBlock(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	tx1 := &flow.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []flow.Address{b.RootAccountAddress()},
	}

	tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

	// Submit tx1
	err := b.SubmitTransaction(tx1)
	tx, _ := b.GetTransaction(tx1.Hash())
	assert.Nil(t, err)
	assert.Equal(t, flow.TransactionFinalized, tx.Status)

	tx2 := &flow.Transaction{
		Script:             []byte("invalid script"),
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []flow.Address{b.RootAccountAddress()},
	}

	tx2.AddSignature(b.RootAccountAddress(), b.RootKey())

	// Submit invalid tx2
	err = b.SubmitTransaction(tx2)
	assert.NotNil(t, err)

	tx, err = b.GetTransaction(tx2.Hash())
	assert.Nil(t, err)

	assert.Equal(t, flow.TransactionReverted, tx.Status)

	// Commit tx1 and tx2 into new block
	b.CommitBlock()

	// tx1 status becomes TransactionSealed
	tx, _ = b.GetTransaction(tx1.Hash())
	assert.Equal(t, flow.TransactionSealed, tx.Status)

	// tx2 status stays TransactionReverted
	tx, _ = b.GetTransaction(tx2.Hash())
	assert.Equal(t, flow.TransactionReverted, tx.Status)
}
