package virtualmachine_test

import (
	"testing"

	"github.com/dapperlabs/cadence/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestBlockContext_ExecuteTransaction(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	h := unittest.BlockHeaderFixture()

	vm := virtualmachine.New(rt)
	bc := vm.NewBlockContext(&h)

	t.Run("transaction success", func(t *testing.T) {
		tx := &flow.TransactionBody{
			ScriptAccounts: []flow.Address{unittest.AddressFixture()},
			Script: []byte(`
                transaction {
                  prepare(signer: AuthAccount) {}
                }
            `),
		}

		ledger := make(virtualmachine.MapLedger)

		result, err := bc.ExecuteTransaction(ledger, tx)

		assert.NoError(t, err)
		assert.True(t, result.Succeeded())
		assert.NoError(t, result.Error)
	})

	t.Run("transaction failure", func(t *testing.T) {
		tx := &flow.TransactionBody{
			Script: []byte(`
                transaction {
                  var x: Int

                  prepare(signer: AuthAccount) {
                    self.x = 0
                  }

                  execute {
                    self.x = 1
                  }

                  post {
                    self.x == 2
                  }
                }
            `),
		}

		ledger := make(virtualmachine.MapLedger)

		result, err := bc.ExecuteTransaction(ledger, tx)

		assert.NoError(t, err)
		assert.False(t, result.Succeeded())
		assert.Error(t, result.Error)
	})

	t.Run("transaction logs", func(t *testing.T) {
		tx := &flow.TransactionBody{
			Script: []byte(`
                transaction {
                  execute {
				    log("foo")
				    log("bar")
				  }
                }
            `),
		}

		ledger := make(virtualmachine.MapLedger)

		result, err := bc.ExecuteTransaction(ledger, tx)
		assert.NoError(t, err)

		require.Len(t, result.Logs, 2)
		assert.Equal(t, "\"foo\"", result.Logs[0])
		assert.Equal(t, "\"bar\"", result.Logs[1])
	})

	t.Run("transaction events", func(t *testing.T) {
		tx := &flow.TransactionBody{
			Script: []byte(`
                transaction {
                  execute {
				    AuthAccount(publicKeys: [], code: [])
				  }
                }
            `),
		}

		ledger := make(virtualmachine.MapLedger)

		result, err := bc.ExecuteTransaction(ledger, tx)
		assert.NoError(t, err)

		assert.True(t, result.Succeeded())
		assert.NoError(t, result.Error)

		require.Len(t, result.Events, 1)
		assert.EqualValues(t, "flow.AccountCreated", result.Events[0].Type.ID())
	})
}

func TestBlockContext_ExecuteScript(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	h := unittest.BlockHeaderFixture()

	vm := virtualmachine.New(rt)
	bc := vm.NewBlockContext(&h)

	t.Run("script success", func(t *testing.T) {
		script := []byte(`
			pub fun main(): Int {
				return 42
			}
		`)

		ledger := make(virtualmachine.MapLedger)

		result, err := bc.ExecuteScript(ledger, script)
		assert.NoError(t, err)
		assert.True(t, result.Succeeded())
	})

	t.Run("script failure", func(t *testing.T) {
		script := []byte(`
			pub fun main(): Int {
				assert 1 == 2
				return 42
			}
		`)

		ledger := make(virtualmachine.MapLedger)

		result, err := bc.ExecuteScript(ledger, script)

		assert.NoError(t, err)
		assert.False(t, result.Succeeded())
		assert.Error(t, result.Error)
	})

	t.Run("script logs", func(t *testing.T) {
		script := []byte(`
			pub fun main(): Int {
				log("foo")
				log("bar")
				return 42
			}
		`)

		ledger := make(virtualmachine.MapLedger)

		result, err := bc.ExecuteScript(ledger, script)
		assert.NoError(t, err)

		require.Len(t, result.Logs, 2)
		assert.Equal(t, "\"foo\"", result.Logs[0])
		assert.Equal(t, "\"bar\"", result.Logs[1])
	})
}
