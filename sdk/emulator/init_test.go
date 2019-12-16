package emulator_test

import (
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/emulator/storage/badger"
	"github.com/dapperlabs/flow-go/sdk/emulator/types"
	"github.com/dapperlabs/flow-go/sdk/keys"
)

func TestInitialization(t *testing.T) {
	dir, err := ioutil.TempDir("", "badger-test")
	require.Nil(t, err)
	defer os.RemoveAll(dir)
	store, err := badger.New(badger.WithPath(dir))
	require.Nil(t, err)
	defer store.Close()

	t.Run("should inject initial state when initialized with empty store", func(t *testing.T) {
		b, _ := emulator.NewEmulatedBlockchain(emulator.WithStore(store))

		rootAcct, err := b.GetAccount(flow.RootAddress)
		assert.NoError(t, err)
		assert.NotNil(t, rootAcct)

		latestBlock, err := b.GetLatestBlock()
		assert.NoError(t, err)
		assert.EqualValues(t, 0, latestBlock.Number)
		assert.Equal(t, types.GenesisBlock().Hash(), latestBlock.Hash())
	})

	t.Run("should restore state when initialized with non-empty store", func(t *testing.T) {
		b, _ := emulator.NewEmulatedBlockchain(emulator.WithStore(store))

		counterAddress, err := b.CreateAccount(nil, []byte(counterScript), getNonce())
		require.NoError(t, err)

		// Submit a transaction adds some ledger state and event state
		script := fmt.Sprintf(
			`
                import 0x%s

                pub event MyEvent(x: Int)

                transaction {

                  prepare(acct: Account) {
                    emit MyEvent(x: 1)

                    let counter <- Counting.createCounter()
                    counter.add(1)

                    let existing <- acct.storage[Counting.Counter] <- counter
                    destroy existing
                    acct.published[&Counting.Counter] = &acct.storage[Counting.Counter] as Counting.Counter
                  }
                }
            `,
			counterAddress,
		)

		tx := flow.Transaction{
			Script:         []byte(script),
			Nonce:          getNonce(),
			ComputeLimit:   10,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{b.RootAccountAddress()},
		}

		sig, err := keys.SignTransaction(tx, b.RootKey())
		assert.NoError(t, err)
		tx.AddSignature(b.RootAccountAddress(), sig)

		result, err := b.SubmitTransaction(tx)
		assert.NoError(t, err)
		assert.True(t, result.Succeeded())

		block, err := b.CommitBlock()
		assert.NoError(t, err)
		require.NotNil(t, block)

		minedTx, err := b.GetTransaction(tx.Hash())
		assert.NoError(t, err)

		minedEvents, err := b.GetEvents("", block.Number, block.Number)

		// Create a new blockchain with the same store
		b, _ = emulator.NewEmulatedBlockchain(emulator.WithStore(store))

		t.Run("should be able to read blocks", func(t *testing.T) {
			latestBlock, err := b.GetLatestBlock()
			assert.NoError(t, err)
			assert.Equal(t, block.Hash(), latestBlock.Hash())

			blockByNumber, err := b.GetBlockByNumber(block.Number)
			assert.NoError(t, err)
			assert.Equal(t, block.Hash(), blockByNumber.Hash())

			blockByHash, err := b.GetBlockByHash(block.Hash())
			assert.NoError(t, err)
			assert.Equal(t, block.Hash(), blockByHash.Hash())
		})

		t.Run("should be able to read transactions", func(t *testing.T) {
			txByHash, err := b.GetTransaction(tx.Hash())
			assert.NoError(t, err)
			assert.Equal(t, minedTx, txByHash)
		})

		t.Run("should be able to read events", func(t *testing.T) {
			gotEvents, err := b.GetEvents("", block.Number, block.Number)
			assert.NoError(t, err)
			assert.Equal(t, minedEvents, gotEvents)
		})

		t.Run("should be able to read ledger state", func(t *testing.T) {
			readScript := fmt.Sprintf(
				`
                  import 0x%s

                  pub fun main(): Int {
                      return getAccount(0x%s).published[&Counting.Counter]?.count ?? 0
                  }
                `,
				counterAddress,
				b.RootAccountAddress(),
			)

			res, _, err := b.ExecuteScript([]byte(readScript))
			assert.NoError(t, err)

			assert.Equal(t, values.Int{Int: big.NewInt(1)}, res)
		})
	})
}
