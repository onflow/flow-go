package emulator_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/emulator/execution"
	"github.com/dapperlabs/flow-go/sdk/keys"
)

func TestEventEmitted(t *testing.T) {
	t.Run("EmittedFromTransaction", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain()

		script := []byte(`
			event MyEvent(x: Int, y: Int)
			
			fun main() {
			  emit MyEvent(x: 1, y: 2)
			}
		`)

		tx := flow.Transaction{
			Script:             script,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
		}

		sig, err := keys.SignTransaction(tx, b.RootKey())
		assert.NoError(t, err)

		tx.AddSignature(b.RootAccountAddress(), sig)

		err = b.SubmitTransaction(tx)
		assert.NoError(t, err)

		block, err := b.CommitBlock()
		require.NoError(t, err)

		events, err := b.GetEvents("", block.Number, block.Number)
		require.NoError(t, err)
		require.Len(t, events, 1)

		expectedType := fmt.Sprintf("tx.%s.MyEvent", tx.Hash().Hex())
		expectedID := flow.Event{TxHash: tx.Hash(), Index: 0}.ID()

		assert.Equal(t, expectedType, events[0].Type)
		assert.Equal(t, expectedID, events[0].ID())
		assert.Equal(t, big.NewInt(1), events[0].Values["x"])
		assert.Equal(t, big.NewInt(2), events[0].Values["y"])
	})

	t.Run("EmittedFromScript", func(t *testing.T) {
		events := make([]flow.Event, 0)

		b := emulator.NewEmulatedBlockchain()

		script := []byte(`
			event MyEvent(x: Int, y: Int)
			
			fun main() {
			  emit MyEvent(x: 1, y: 2)
			}
		`)

		_, events, err := b.ExecuteScript(script)
		assert.NoError(t, err)
		require.Len(t, events, 1)

		expectedType := fmt.Sprintf("script.%s.MyEvent", execution.ScriptHash(script).Hex())
		// NOTE: ID is undefined for events emitted from scripts

		assert.Equal(t, expectedType, events[0].Type)
		assert.Equal(t, big.NewInt(1), events[0].Values["x"])
		assert.Equal(t, big.NewInt(2), events[0].Values["y"])
	})

	t.Run("EmittedFromAccount", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain()

		accountScript := []byte(`
			event MyEvent(x: Int, y: Int)

			fun emitMyEvent(x: Int, y: Int) {
				emit MyEvent(x: x, y: y)
			}
		`)

		publicKey := b.RootKey().PublicKey(keys.PublicKeyWeightThreshold)

		address, err := b.CreateAccount([]flow.AccountPublicKey{publicKey}, accountScript, getNonce())
		assert.NoError(t, err)

		script := []byte(fmt.Sprintf(`
			import 0x%s
			
			fun main() {
				emitMyEvent(x: 1, y: 2)
			}
		`, address.Hex()))

		tx := flow.Transaction{
			Script:             script,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
		}

		sig, err := keys.SignTransaction(tx, b.RootKey())
		assert.NoError(t, err)

		tx.AddSignature(b.RootAccountAddress(), sig)

		err = b.SubmitTransaction(tx)
		assert.NoError(t, err)

		block, err := b.CommitBlock()
		require.NoError(t, err)

		expectedType := fmt.Sprintf("account.%s.MyEvent", address.Hex())
		events, err := b.GetEvents(expectedType, block.Number, block.Number)
		require.NoError(t, err)
		require.Len(t, events, 1)

		actualEvent := events[0]
		expectedID := flow.Event{TxHash: tx.Hash(), Index: 0}.ID()

		assert.Equal(t, expectedType, actualEvent.Type)
		assert.Equal(t, expectedID, actualEvent.ID())
		assert.Equal(t, big.NewInt(1), actualEvent.Values["x"])
		assert.Equal(t, big.NewInt(2), actualEvent.Values["y"])
	})
}
