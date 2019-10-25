package emulator_test

import (
	"fmt"
	"github.com/dapperlabs/flow-go/pkg/hash"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/pkg/constants"
	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/emulator/execution"
	"github.com/dapperlabs/flow-go/sdk/keys"
)

func TestEventEmitted(t *testing.T) {
	t.Run("EmittedFromTransaction", func(t *testing.T) {
		events := make([]types.Event, 0)

		b := emulator.NewEmulatedBlockchain(emulator.EmulatedBlockchainOptions{
			OnEventEmitted: func(event types.Event, blockNumber uint64, txHash crypto.Hash) {
				events = append(events, event)
			},
		})

		script := []byte(`
			event MyEvent(x: Int, y: Int)
			
			fun main() {
			  emit MyEvent(x: 1, y: 2)
			}
		`)

		tx := types.Transaction{
			Script:             script,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
		}

		hash.SetTransactionHash(&tx)

		sig, err := keys.SignTransaction(tx, b.RootKey())
		assert.Nil(t, err)

		tx.AddSignature(b.RootAccountAddress(), sig)

		err = b.SubmitTransaction(tx)
		assert.Nil(t, err)

		require.Len(t, events, 1)

		expectedID := fmt.Sprintf("tx.%s.MyEvent", tx.Hash.Hex())

		assert.Equal(t, expectedID, events[0].ID)
		assert.Equal(t, big.NewInt(1), events[0].Values["x"])
		assert.Equal(t, big.NewInt(2), events[0].Values["y"])
	})

	t.Run("EmittedFromScript", func(t *testing.T) {
		events := make([]types.Event, 0)

		b := emulator.NewEmulatedBlockchain(emulator.EmulatedBlockchainOptions{
			OnEventEmitted: func(event types.Event, blockNumber uint64, txHash crypto.Hash) {
				events = append(events, event)
			},
		})

		script := []byte(`
			event MyEvent(x: Int, y: Int)
			
			fun main() {
			  emit MyEvent(x: 1, y: 2)
			}
		`)

		_, err := b.CallScript(script)
		assert.Nil(t, err)

		require.Len(t, events, 1)

		expectedID := fmt.Sprintf("script.%s.MyEvent", execution.ScriptHash(script).Hex())

		assert.Equal(t, expectedID, events[0].ID)
		assert.Equal(t, big.NewInt(1), events[0].Values["x"])
		assert.Equal(t, big.NewInt(2), events[0].Values["y"])
	})

	t.Run("EmittedFromAccount", func(t *testing.T) {
		events := make([]types.Event, 0)

		b := emulator.NewEmulatedBlockchain(emulator.EmulatedBlockchainOptions{
			OnEventEmitted: func(event types.Event, blockNumber uint64, txHash crypto.Hash) {
				events = append(events, event)
			},
		})

		accountScript := []byte(`
			event MyEvent(x: Int, y: Int)
		`)

		publicKey := b.RootKey().PublicKey(constants.AccountKeyWeightThreshold)

		address, err := b.CreateAccount([]types.AccountPublicKey{publicKey}, accountScript, getNonce())
		assert.Nil(t, err)

		script := []byte(fmt.Sprintf(`
			import 0x%s
			
			fun main() {
				emit MyEvent(x: 1, y: 2)
			}
		`, address.Hex()))

		tx := types.Transaction{
			Script:             script,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
		}

		sig, err := keys.SignTransaction(tx, b.RootKey())
		assert.Nil(t, err)

		tx.AddSignature(b.RootAccountAddress(), sig)

		err = b.SubmitTransaction(tx)
		assert.Nil(t, err)

		require.Len(t, events, 2)

		// first event is AccountCreated event
		expectedEvent := events[1]

		expectedID := fmt.Sprintf("account.%s.MyEvent", address.Hex())

		assert.Equal(t, expectedID, expectedEvent.ID)
		assert.Equal(t, big.NewInt(1), expectedEvent.Values["x"])
		assert.Equal(t, big.NewInt(2), expectedEvent.Values["y"])
	})
}
