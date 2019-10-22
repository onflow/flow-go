package emulator_test

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/sdk/emulator"
)

func TestWorldStates(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	// Create 3 signed transactions (tx1, tx2, tx3)
	tx1 := &types.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

	tx2 := &types.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		Nonce:              1,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx2.AddSignature(b.RootAccountAddress(), b.RootKey())

	tx3 := &types.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		Nonce:              3,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx3.AddSignature(b.RootAccountAddress(), b.RootKey())

	ws1 := b.pendingWorldState.Hash()
	t.Logf("initial world state: %x\n", ws1)

	// Tx pool contains nothing
	assert.Len(t, b.txPool, 0)

	// Submit tx1
	err := b.SubmitTransaction(tx1)
	assert.Nil(t, err)

	ws2 := b.pendingWorldState.Hash()
	t.Logf("world state after tx1: %x\n", ws2)

	// tx1 included in tx pool
	assert.Len(t, b.txPool, 1)
	// World state updates
	assert.NotEqual(t, ws1, ws2)

	// Submit tx1 again
	err = b.SubmitTransaction(tx1)
	assert.NotNil(t, err)

	ws3 := b.pendingWorldState.Hash()
	t.Logf("world state after dup tx1: %x\n", ws3)

	// tx1 not included in tx pool
	assert.Len(t, b.txPool, 1)
	// World state does not update
	assert.Equal(t, ws2, ws3)

	// Submit tx2
	err = b.SubmitTransaction(tx2)
	assert.Nil(t, err)

	ws4 := b.pendingWorldState.Hash()
	t.Logf("world state after tx2: %x\n", ws4)

	// tx2 included in tx pool
	assert.Len(t, b.txPool, 2)
	// World state updates
	assert.NotEqual(t, ws3, ws4)

	// Commit new block
	b.CommitBlock()
	ws5 := b.pendingWorldState.Hash()
	t.Logf("world state after commit: %x\n", ws5)

	// Tx pool cleared
	assert.Len(t, b.txPool, 0)
	// World state updates
	assert.NotEqual(t, ws4, ws5)
	// World state is indexed
	assert.Contains(t, b.worldStates, string(ws5))

	// Submit tx3
	err = b.SubmitTransaction(tx3)
	assert.Nil(t, err)

	ws6 := b.pendingWorldState.Hash()
	t.Logf("world state after tx3: %x\n", ws6)

	// tx3 included in tx pool
	assert.Len(t, b.txPool, 1)
	// World state updates
	assert.NotEqual(t, ws5, ws6)

	// Seek to committed block/world state
	b.SeekToState(ws5)
	ws7 := b.pendingWorldState.Hash()
	t.Logf("world state after seek: %x\n", ws7)

	// Tx pool cleared
	assert.Len(t, b.txPool, 0)
	// World state rollback to ws5 (before tx3)
	assert.Equal(t, ws5, ws7)
	// World state does not include tx3
	assert.False(t, b.pendingWorldState.ContainsTransaction(tx3.Hash()))

	// Seek to non-committed world state
	b.SeekToState(ws4)
	ws8 := b.pendingWorldState.Hash()
	t.Logf("world state after failed seek: %x\n", ws8)

	// World state does not rollback to ws4 (before commit block)
	assert.NotEqual(t, ws4, ws8)
}

func TestQueryByVersion(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	tx1 := &types.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx1.AddSignature(b.RootAccountAddress(), b.RootKey())

	tx2 := &types.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		Nonce:              1,
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []types.Address{b.RootAccountAddress()},
	}

	tx2.AddSignature(b.RootAccountAddress(), b.RootKey())

	var invalidWorldState crypto.Hash

	// Submit tx1 and tx2 (logging state versions before and after)
	ws1 := b.pendingWorldState.Hash()

	err := b.SubmitTransaction(tx1)
	assert.Nil(t, err)

	ws2 := b.pendingWorldState.Hash()

	err = b.SubmitTransaction(tx2)
	assert.Nil(t, err)

	ws3 := b.pendingWorldState.Hash()

	// Get transaction at invalid world state version (errors)
	tx, err := b.GetTransactionAtVersion(tx1.Hash(), invalidWorldState)
	assert.IsType(t, err, &emulator.ErrInvalidStateVersion{})
	assert.Nil(t, tx)

	// tx1 does not exist at ws1
	tx, err = b.GetTransactionAtVersion(tx1.Hash(), ws1)
	assert.IsType(t, err, &emulator.ErrTransactionNotFound{})
	assert.Nil(t, tx)

	// tx1 does exist at ws2
	tx, err = b.GetTransactionAtVersion(tx1.Hash(), ws2)
	assert.Nil(t, err)
	assert.NotNil(t, tx)

	// tx2 does not exist at ws2
	tx, err = b.GetTransactionAtVersion(tx2.Hash(), ws2)
	assert.IsType(t, err, &emulator.ErrTransactionNotFound{})
	assert.Nil(t, tx)

	// tx2 does exist at ws3
	tx, err = b.GetTransactionAtVersion(tx2.Hash(), ws3)
	assert.Nil(t, err)
	assert.NotNil(t, tx)

	// Call script at invalid world state version (errors)
	value, err := b.CallScriptAtVersion([]byte(sampleCall), invalidWorldState)
	assert.IsType(t, err, &emulator.ErrInvalidStateVersion{})
	assert.Nil(t, value)

	// Value at ws1 is 0
	value, err = b.CallScriptAtVersion([]byte(sampleCall), ws1)
	assert.Nil(t, err)
	assert.Equal(t, big.NewInt(0), value)

	// Value at ws2 is 2 (after script executed)
	value, err = b.CallScriptAtVersion([]byte(sampleCall), ws2)
	assert.Nil(t, err)
	assert.Equal(t, big.NewInt(2), value)

	// Value at ws3 is 4 (after script executed)
	value, err = b.CallScriptAtVersion([]byte(sampleCall), ws3)
	assert.Nil(t, err)
	assert.Equal(t, big.NewInt(4), value)

	// Pending state does not change after call scripts/get transactions
	assert.Equal(t, ws3, b.pendingWorldState.Hash())
}
