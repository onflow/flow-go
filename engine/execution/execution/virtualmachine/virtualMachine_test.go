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

func TestExecuteTransaction(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	ledger := &vmmock.Ledger{}

	vm := virtualmachine.New(rt)

	t.Run("transaction success", func(t *testing.T) {
		tx := &flow.Transaction{
			TransactionBody: flow.TransactionBody{
				ScriptAccounts: []flow.Address{unittest.AddressFixture()},
				Script: []byte(`
                transaction {
                  prepare(signer: Account) {}
                }
            `),
			},
		}

		result, err := vm.ExecuteTransaction(ledger, tx)
		assert.NoError(t, err)
		assert.True(t, result.Succeeded())
		assert.NoError(t, result.Error)
	})

	t.Run("transaction failure", func(t *testing.T) {
		tx := &flow.Transaction{
			TransactionBody: flow.TransactionBody{
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
			},
		}

		result, err := vm.ExecuteTransaction(ledger, tx)
		assert.NoError(t, err)
		assert.False(t, result.Succeeded())
		assert.Error(t, result.Error)
	})
}
