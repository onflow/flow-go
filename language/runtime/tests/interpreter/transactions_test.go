package interpreter_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/interpreter"
)

func TestInterpretTransactions(t *testing.T) {
	t.Run("NoPrepareFunction", func(t *testing.T) {
		inter := parseCheckAndInterpret(t, `
          transaction {
            execute {
              let x = 1 + 2
            }
          }
        `)

		err := inter.InvokeTransaction(0)
		assert.NoError(t, err)
	})

	t.Run("SetTransactionField", func(t *testing.T) {
		inter := parseCheckAndInterpret(t, `
          transaction {
            
            var x: Int

            prepare() {
              self.x = 5
            }
            
            execute {
              let y = self.x + 1
            }
          }
        `)

		err := inter.InvokeTransaction(0)
		assert.NoError(t, err)
	})

	t.Run("PreConditions", func(t *testing.T) {
		inter := parseCheckAndInterpret(t, `
          transaction {
            
            var x: Int

            prepare() {
              self.x = 5
            }

            pre {
              self.x > 1
            }
          }
        `)

		err := inter.InvokeTransaction(0)
		assert.NoError(t, err)
	})

	t.Run("FailingPreConditions", func(t *testing.T) {
		inter := parseCheckAndInterpret(t, `
          transaction {
            
            var x: Int

            prepare() {
              self.x = 5
            }

            pre {
              self.x > 10
            }
          }
        `)

		err := inter.InvokeTransaction(0)
		require.IsType(t, &interpreter.ConditionError{}, err)

		conditionErr := err.(*interpreter.ConditionError)

		assert.Equal(t, conditionErr.ConditionKind, ast.ConditionKindPre)
	})

	t.Run("PostConditions", func(t *testing.T) {
		inter := parseCheckAndInterpret(t, `
          transaction {
            
            var x: Int

            prepare() {
              self.x = 5
            }
            
            execute {
              self.x = 10
            }

            post {
              self.x == 10
            }
          }
        `)

		err := inter.InvokeTransaction(0)
		assert.NoError(t, err)
	})

	t.Run("FailingPostConditions", func(t *testing.T) {
		inter := parseCheckAndInterpret(t, `
          transaction {
            
            var x: Int

            prepare() {
              self.x = 5
            }
            
            execute {
              self.x = 10
            }

            post {
              self.x == 5
            }
          }
        `)

		err := inter.InvokeTransaction(0)
		require.IsType(t, &interpreter.ConditionError{}, err)

		conditionErr := err.(*interpreter.ConditionError)

		assert.Equal(t, conditionErr.ConditionKind, ast.ConditionKindPost)
	})

	t.Run("MultipleTransactions", func(t *testing.T) {
		inter := parseCheckAndInterpret(t, `
          transaction {
            execute {
              let x = 1 + 2
            }
          }

          transaction {
            execute {
              let y = 3 + 4
            }
          }
        `)

		// first transaction
		err := inter.InvokeTransaction(0)
		assert.NoError(t, err)

		// second transaction
		err = inter.InvokeTransaction(1)
		assert.NoError(t, err)

		// third transaction is not declared
		err = inter.InvokeTransaction(2)
		assert.IsType(t, &interpreter.TransactionNotDeclaredError{}, err)
	})

	t.Run("TooFewArguments", func(t *testing.T) {
		inter := parseCheckAndInterpret(t, `
          transaction {
            prepare(signer: Account) {}
          }
        `)

		err := inter.InvokeTransaction(0)
		assert.IsType(t, &interpreter.ArgumentCountError{}, err)
	})

	t.Run("TooManyArguments", func(t *testing.T) {
		inter := parseCheckAndInterpret(t, `
          transaction {
            execute {}
          }

          transaction {
            prepare(signer: Account) {}

            execute {}
          }
        `)

		signer1 := interpreter.NewAccountValue(
			interpreter.AddressValue{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		)
		signer2 := interpreter.NewAccountValue(
			interpreter.AddressValue{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2},
		)

		// first transaction
		err := inter.InvokeTransaction(0, signer1)
		assert.IsType(t, &interpreter.ArgumentCountError{}, err)

		// second transaction
		err = inter.InvokeTransaction(0, signer1, signer2)
		assert.IsType(t, &interpreter.ArgumentCountError{}, err)
	})
}
