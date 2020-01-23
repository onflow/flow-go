package virtualmachine_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine"
	vmmock "github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine/mock"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestBlockContext_ExecuteTransaction(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	ledger := &vmmock.Ledger{}

	b := unittest.BlockFixture()

	vm := virtualmachine.New(rt)
	bc := vm.NewBlockContext(&b)

	t.Run("transaction success", func(t *testing.T) {
		tx := &flow.TransactionBody{
			ScriptAccounts: []flow.Address{unittest.AddressFixture()},
			Script: []byte(`
                transaction {
                  prepare(signer: Account) {}
                }
            `),
		}

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

                  prepare(signer: Account) {
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

		result, err := bc.ExecuteTransaction(ledger, tx)

		assert.NoError(t, err)
		assert.False(t, result.Succeeded())
		assert.Error(t, result.Error)
	})
}

func TestBlockContext_ExecuteScript(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	ledger := &vmmock.Ledger{}

	b := unittest.BlockFixture()

	vm := virtualmachine.New(rt)
	bc := vm.NewBlockContext(&b)

	t.Run("script success", func(t *testing.T) {
		script := []byte(`
			pub fun main(): Int {
				return 42
			}
		`)

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

		result, err := bc.ExecuteScript(ledger, script)

		assert.NoError(t, err)
		assert.False(t, result.Succeeded())
		assert.Error(t, result.Error)
	})
}
