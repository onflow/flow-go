package fvm

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

func TestAccountFreezing(t *testing.T) {

	ledger := state.NewMapLedger()
	st := state.NewState(ledger)

	frozenAddress := flow.HexToAddress("1234")
	notFrozenAddress := flow.HexToAddress("5678")

	accounts := state.NewAccounts(st)
	err := accounts.Create(nil, frozenAddress)
	require.NoError(t, err)
	err = accounts.Create(nil, notFrozenAddress)
	require.NoError(t, err)

	err = accounts.SetAccountFrozen(frozenAddress, true)
	require.NoError(t, err)

	err = accounts.SetContract("Whatever", notFrozenAddress, []byte(`
		pub contract Whatever {
			pub fun say() {
				log("whatever not frozen")
			}
		}
	`))
	require.NoError(t, err)

	err = st.Commit()
	require.NoError(t, err)

	t.Run("setFrozenAccount can be enabled", func(t *testing.T) {

		rt := runtime.NewInterpreterRuntime()

		buffer := &bytes.Buffer{}
		log := zerolog.New(buffer)
		txInvocator := NewTransactionInvocator(log)

		vm := New(rt)

		code := fmt.Sprintf(`
			transaction {
				execute {
					setAccountFrozen(0x%s, true)
				}
			}
		`, frozenAddress.String())

		proc := Transaction(&flow.TransactionBody{Script: []byte(code)}, 0)

		context := NewContext(log, WithAccountFreezeAvailable(false))

		err = txInvocator.Process(vm, context, proc, st)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot find")
		require.Contains(t, err.Error(), "setAccountFrozen")

		context = NewContext(log, WithAccountFreezeAvailable(true))

		err = txInvocator.Process(vm, context, proc, st)
		require.NoError(t, err)
	})

	t.Run("frozen account is rejected", func(t *testing.T) {

		txChecker := NewTransactionAccountFrozenChecker()

		// Authorizers

		// no account associated with tx so it should work
		tx := &flow.TransactionBody{}
		err = txChecker.checkAccountNotFrozen(tx, st)
		require.NoError(t, err)

		tx = &flow.TransactionBody{Authorizers: []flow.Address{notFrozenAddress}}
		err = txChecker.checkAccountNotFrozen(tx, st)
		require.NoError(t, err)

		tx = &flow.TransactionBody{Authorizers: []flow.Address{frozenAddress}}
		err = txChecker.checkAccountNotFrozen(tx, st)
		require.Error(t, err)

		// all addresses must not be frozen
		tx = &flow.TransactionBody{Authorizers: []flow.Address{frozenAddress, notFrozenAddress}}
		err = txChecker.checkAccountNotFrozen(tx, st)
		require.Error(t, err)

		// Payer should be part of authorizers account, but lets check it separately for completeness

		tx = &flow.TransactionBody{Payer: notFrozenAddress}
		err = txChecker.checkAccountNotFrozen(tx, st)
		require.NoError(t, err)

		tx = &flow.TransactionBody{Payer: frozenAddress}
		err = txChecker.checkAccountNotFrozen(tx, st)
		require.Error(t, err)

		// Proposal account

		tx = &flow.TransactionBody{ProposalKey: flow.ProposalKey{Address: frozenAddress}}
		err = txChecker.checkAccountNotFrozen(tx, st)
		require.Error(t, err)

		tx = &flow.TransactionBody{ProposalKey: flow.ProposalKey{Address: notFrozenAddress}}
		err = txChecker.checkAccountNotFrozen(tx, st)
		require.NoError(t, err)
	})

	t.Run("code from frozen account cannot be loaded", func(t *testing.T) {

		rt := runtime.NewInterpreterRuntime()

		buffer := &bytes.Buffer{}
		log := zerolog.New(buffer)
		txInvocator := NewTransactionInvocator(log)

		vm := New(rt)

		code := fmt.Sprintf(`
			import Whatever from 0x%s

			transaction {
				execute {
					Whatever.say()
				}
			}
		`, notFrozenAddress.String())

		proc := Transaction(&flow.TransactionBody{Script: []byte(code)}, 0)

		context := NewContext(log)

		err = txInvocator.Process(vm, context, proc, st)
		require.NoError(t, err)
		require.Contains(t, buffer, "whatever not frozen")

	})

}
