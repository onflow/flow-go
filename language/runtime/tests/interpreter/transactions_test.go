package interpreter_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
	"github.com/dapperlabs/flow-go/language/runtime/interpreter"
	"github.com/dapperlabs/flow-go/language/runtime/trampoline"
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
            prepare(signer: AuthAccount) {}
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
            prepare(signer: AuthAccount) {}

            execute {}
          }
        `)

		panicFunction := interpreter.NewHostFunctionValue(func(invocation interpreter.Invocation) trampoline.Trampoline {
			panic(errors.NewUnreachableError())
		})

		signer1 := interpreter.NewAuthAccountValue(
			interpreter.AddressValue{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			panicFunction,
			panicFunction,
			panicFunction,
		)
		signer2 := interpreter.NewAuthAccountValue(
			interpreter.AddressValue{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2},
			panicFunction,
			panicFunction,
			panicFunction,
		)

		// first transaction
		err := inter.InvokeTransaction(0, signer1)
		assert.IsType(t, &interpreter.ArgumentCountError{}, err)

		// second transaction
		err = inter.InvokeTransaction(0, signer1, signer2)
		assert.IsType(t, &interpreter.ArgumentCountError{}, err)
	})

	t.Run("Parameters", func(t *testing.T) {

		inter := parseCheckAndInterpret(t, `
          let values: [AnyStruct] = []

          transaction(x: Int, y: Bool) {

            prepare(signer: AuthAccount) {
              values.append(signer.address)
              values.append(y)
              values.append(x)
            }
          }
        `)

		transactionArguments := []interface{}{1, true}
		prepareArguments := []interface{}{
			interpreter.NewAuthAccountValue(interpreter.AddressValue{}, nil, nil, nil),
		}

		arguments := append(transactionArguments, prepareArguments...)

		err := inter.InvokeTransaction(0, arguments...)
		assert.NoError(t, err)

		values := inter.Globals["values"].Value

		require.IsType(t, values, &interpreter.ArrayValue{})

		assert.Equal(t,
			[]interpreter.Value{
				interpreter.AddressValue{},
				interpreter.BoolValue(true),
				interpreter.NewIntValue(1),
			},
			values.(*interpreter.ArrayValue).Values,
		)
	})

}
