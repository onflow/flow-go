package emulator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/keys"
)

func TestPendingBlockBeforeExecution(t *testing.T) {
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

	tx2 := flow.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []flow.Address{b.RootAccountAddress()},
	}

	sig, err = keys.SignTransaction(tx2, b.RootKey())
	assert.NoError(t, err)

	tx2.AddSignature(b.RootAccountAddress(), sig)

	t.Run("EmptyPendingBlock", func(t *testing.T) {
		// Execute empty pending block
		_, err := b.ExecuteBlock()
		assert.NoError(t, err)

		// Commit empty pending block
		_, err = b.CommitBlock()
		assert.NoError(t, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("AddDuplicateTransaction", func(t *testing.T) {
		// Add tx1 to pending block
		err = b.AddTransaction(tx1)
		assert.NoError(t, err)

		// Add tx1 again
		err = b.AddTransaction(tx1)
		assert.IsType(t, &emulator.ErrDuplicateTransaction{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("CommitBeforeExecution", func(t *testing.T) {
		// Add tx1 to pending block
		err = b.AddTransaction(tx1)
		assert.NoError(t, err)

		// Attempt to commit block before execution begins
		_, err = b.CommitBlock()
		assert.IsType(t, &emulator.ErrPendingBlockCommitBeforeExecution{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})
}

func TestPendingBlockDuringExecution(t *testing.T) {
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

	tx2 := flow.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []flow.Address{b.RootAccountAddress()},
	}

	sig, err = keys.SignTransaction(tx2, b.RootKey())
	assert.NoError(t, err)

	tx2.AddSignature(b.RootAccountAddress(), sig)

	invalid := flow.Transaction{
		Script:             []byte("invalid script"),
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []flow.Address{b.RootAccountAddress()},
	}

	sig, err = keys.SignTransaction(invalid, b.RootKey())
	assert.NoError(t, err)

	invalid.AddSignature(b.RootAccountAddress(), sig)

	t.Run("ExecuteNextTransaction", func(t *testing.T) {
		// Add tx1 to pending block
		err = b.AddTransaction(tx1)
		assert.NoError(t, err)

		// Add invalid script tx to pending block
		err = b.AddTransaction(invalid)
		assert.NoError(t, err)

		// Execute tx1 (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Succeeded())

		// Execute invalid script tx (reverts)
		result, err = b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Reverted())

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("ExecuteBlock", func(t *testing.T) {
		// Add tx1 to pending block
		err = b.AddTransaction(tx1)
		assert.NoError(t, err)

		// Add invalid script tx to pending block
		err = b.AddTransaction(invalid)
		assert.NoError(t, err)

		// Execute all tx in pending block (tx1, invalid)
		results, err := b.ExecuteBlock()
		assert.NoError(t, err)
		// tx1 result
		assert.True(t, results[0].Succeeded())
		// invalid script tx result
		assert.True(t, results[1].Reverted())

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("ExecuteNextThenBlock", func(t *testing.T) {
		// Add tx1 to pending block
		err = b.AddTransaction(tx1)
		assert.NoError(t, err)

		// Add tx2 to pending block
		err = b.AddTransaction(tx2)
		assert.NoError(t, err)

		// Add invalid script tx to pending block
		err = b.AddTransaction(invalid)
		assert.NoError(t, err)

		// Execute tx1 first (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Succeeded())

		// Execute rest of tx in pending block (tx2, invalid)
		results, err := b.ExecuteBlock()
		assert.NoError(t, err)
		// tx2 result
		assert.True(t, results[0].Succeeded())
		// invalid script tx result
		assert.True(t, results[1].Reverted())

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("AddTransactionMidExecution", func(t *testing.T) {
		// Add tx1 to pending block
		err = b.AddTransaction(tx1)
		assert.NoError(t, err)

		// Add invalid to pending block
		err = b.AddTransaction(invalid)
		assert.NoError(t, err)

		// Execute tx1 first (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Succeeded())

		// Attempt to add tx2 to pending block after execution begins
		err = b.AddTransaction(tx2)
		assert.IsType(t, &emulator.ErrPendingBlockMidExecution{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("CommitMidExecution", func(t *testing.T) {
		// Add tx1 to pending block
		err = b.AddTransaction(tx1)
		assert.NoError(t, err)

		// Add invalid to pending block
		err = b.AddTransaction(invalid)
		assert.NoError(t, err)

		// Execute tx1 first (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Succeeded())

		// Attempt to commit block before execution finishes
		_, err = b.CommitBlock()
		assert.IsType(t, &emulator.ErrPendingBlockMidExecution{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})

	t.Run("TransactionsExhaustedDuringExecution", func(t *testing.T) {
		// Add tx1 to pending block
		err = b.AddTransaction(tx1)
		assert.NoError(t, err)

		// Execute tx1 (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Succeeded())

		// Attempt to execute nonexistent next tx (fails)
		_, err = b.ExecuteNextTransaction()
		assert.IsType(t, &emulator.ErrPendingBlockTransactionsExhausted{}, err)

		// Attempt to execute rest of block tx (fails)
		_, err = b.ExecuteBlock()
		assert.IsType(t, &emulator.ErrPendingBlockTransactionsExhausted{}, err)

		err = b.ResetPendingBlock()
		assert.NoError(t, err)
	})
}

func TestPendingBlockCommit(t *testing.T) {
	b, err := emulator.NewEmulatedBlockchain()
	require.NoError(t, err)

	addTwoScript, _ := deployAndGenerateAddTwoScript(t, b)

	t.Run("CommitBlock", func(t *testing.T) {
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

		// Enter execution mode (block hash should not change after this point)
		blockHash := b.GetPendingBlock().Hash()

		// Execute tx1 (succeeds)
		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Succeeded())

		// Commit pending block
		block, err := b.CommitBlock()
		assert.NoError(t, err)
		assert.Equal(t, blockHash, block.Hash())
	})

	t.Run("ExecuteAndCommitBlock", func(t *testing.T) {
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

		// Enter execution mode (block hash should not change after this point)
		blockHash := b.GetPendingBlock().Hash()

		// Execute and commit pending block
		block, results, err := b.ExecuteAndCommitBlock()
		assert.NoError(t, err)
		assert.Equal(t, blockHash, block.Hash())
		assert.Len(t, results, 1)
	})
}
