package emulator_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	encodingValues "github.com/dapperlabs/flow-go/sdk/abi/encoding/values"
	"github.com/dapperlabs/flow-go/sdk/abi/types"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/keys"
)

func TestEventEmitted(t *testing.T) {
	// event type definition that is reused in tests
	myEventType := types.Event{
		Fields: []*types.Parameter{
			{
				Field: types.Field{
					Identifier: "x",
					Type:       types.Int{},
				},
			},
			{
				Field: types.Field{
					Identifier: "y",
					Type:       types.Int{},
				},
			},
		},
	}

	t.Run("EmittedFromTransaction", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		script := []byte(`
			pub event MyEvent(x: Int, y: Int)
			
			transaction {
		  	  execute {
			  	emit MyEvent(x: 1, y: 2)
			  }
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

		err = b.AddTransaction(tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Succeeded())

		block, err := b.CommitBlock()
		require.NoError(t, err)

		events, err := b.GetEvents("", block.Number, block.Number)
		require.NoError(t, err)
		require.Len(t, events, 1)

		actualEvent := events[0]

		eventValue, err := encodingValues.Decode(myEventType, actualEvent.Payload)
		assert.NoError(t, err)

		decodedEvent := eventValue.(values.Event)

		expectedType := fmt.Sprintf("tx.%s.MyEvent", tx.Hash().Hex())
		expectedID := flow.Event{TxHash: tx.Hash(), Index: 0}.ID()

		assert.Equal(t, expectedType, actualEvent.Type)
		assert.Equal(t, expectedID, actualEvent.ID())
		assert.Equal(t, values.NewInt(1), decodedEvent.Fields[0])
		assert.Equal(t, values.NewInt(2), decodedEvent.Fields[1])
	})

	t.Run("EmittedFromScript", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		script := []byte(`
			pub event MyEvent(x: Int, y: Int)
			
			pub fun main() {
			  emit MyEvent(x: 1, y: 2)
			}
		`)

		result, err := b.ExecuteScript(script)
		assert.NoError(t, err)
		require.Len(t, result.Events, 1)

		actualEvent := result.Events[0]

		eventValue, err := encodingValues.Decode(myEventType, actualEvent.Payload)
		assert.NoError(t, err)

		decodedEvent := eventValue.(values.Event)

		expectedType := fmt.Sprintf("script.%s.MyEvent", result.ScriptHash.Hex())
		// NOTE: ID is undefined for events emitted from scripts

		assert.Equal(t, expectedType, actualEvent.Type)
		assert.Equal(t, values.NewInt(1), decodedEvent.Fields[0])
		assert.Equal(t, values.NewInt(2), decodedEvent.Fields[1])
	})

	t.Run("EmittedFromAccount", func(t *testing.T) {
		b, err := emulator.NewBlockchain()
		require.NoError(t, err)

		accountScript := []byte(`
			pub event MyEvent(x: Int, y: Int)

            pub contract Test {
				pub fun emitMyEvent(x: Int, y: Int) {
					emit MyEvent(x: x, y: y)
				}
			}
		`)

		publicKey := b.RootKey().PublicKey(keys.PublicKeyWeightThreshold)

		address, err := b.CreateAccount([]flow.AccountPublicKey{publicKey}, accountScript, getNonce())
		assert.NoError(t, err)

		script := []byte(fmt.Sprintf(`
			import 0x%s
			
			transaction {
				execute {
					Test.emitMyEvent(x: 1, y: 2)
				}
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

		err = b.AddTransaction(tx)
		assert.NoError(t, err)

		result, err := b.ExecuteNextTransaction()
		assert.NoError(t, err)
		assert.True(t, result.Succeeded())

		block, err := b.CommitBlock()
		require.NoError(t, err)

		expectedType := fmt.Sprintf("account.%s.MyEvent", address.Hex())
		events, err := b.GetEvents(expectedType, block.Number, block.Number)
		require.NoError(t, err)
		require.Len(t, events, 1)

		actualEvent := events[0]

		eventValue, err := encodingValues.Decode(myEventType, actualEvent.Payload)
		assert.NoError(t, err)

		decodedEvent := eventValue.(values.Event)

		expectedID := flow.Event{TxHash: tx.Hash(), Index: 0}.ID()

		assert.Equal(t, expectedType, actualEvent.Type)
		assert.Equal(t, expectedID, actualEvent.ID())
		assert.Equal(t, values.NewInt(1), decodedEvent.Fields[0])
		assert.Equal(t, values.NewInt(2), decodedEvent.Fields[1])
	})
}
