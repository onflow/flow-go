package fvm

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

func makeTwoAccounts(t *testing.T) (flow.Address, flow.Address, *state.State) {

	ledger := state.NewMapLedger()
	st := state.NewState(ledger)

	a := flow.HexToAddress("1234")
	b := flow.HexToAddress("5678")

	//create accounts
	accounts := state.NewAccounts(st)
	err := accounts.Create(nil, a)
	require.NoError(t, err)
	err = accounts.Create(nil, b)
	require.NoError(t, err)

	err = st.Commit()
	require.NoError(t, err)

	return a, b, st
}

func TestAccountFreezing(t *testing.T) {

	t.Run("setFrozenAccount can be enabled", func(t *testing.T) {

		address, _, st := makeTwoAccounts(t)
		accounts := state.NewAccounts(st)

		// account should no be frozen
		frozen, err := accounts.GetAccountFrozen(address)
		require.NoError(t, err)
		require.False(t, frozen)

		rt := runtime.NewInterpreterRuntime()
		log := zerolog.Nop()
		vm := New(rt)
		txInvocator := NewTransactionInvocator(log)

		code := fmt.Sprintf(`
			transaction {
				execute {
					setAccountFrozen(0x%s, true)
				}
			}
		`, address.String())

		proc := Transaction(&flow.TransactionBody{Script: []byte(code)}, 0)

		context := NewContext(log, WithAccountFreezeAvailable(false))

		err = txInvocator.Process(vm, context, proc, st)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot find")
		require.Contains(t, err.Error(), "setAccountFrozen")

		context = NewContext(log, WithAccountFreezeAvailable(true))

		err = txInvocator.Process(vm, context, proc, st)
		require.NoError(t, err)

		// account should be frozen now
		frozen, err = accounts.GetAccountFrozen(address)
		require.NoError(t, err)
		require.True(t, frozen)
	})

	t.Run("frozen account is rejected", func(t *testing.T) {

		txChecker := NewTransactionAccountFrozenChecker()

		frozenAddress, notFrozenAddress, st := makeTwoAccounts(t)
		accounts := state.NewAccounts(st)

		// freeze account
		err := accounts.SetAccountFrozen(frozenAddress, true)
		require.NoError(t, err)

		// make sure freeze status is correct
		frozen, err := accounts.GetAccountFrozen(frozenAddress)
		require.NoError(t, err)
		require.True(t, frozen)

		frozen, err = accounts.GetAccountFrozen(notFrozenAddress)
		require.NoError(t, err)
		require.False(t, frozen)

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

		frozenAddress, notFrozenAddress, st := makeTwoAccounts(t)
		accounts := state.NewAccounts(st)

		rt := runtime.NewInterpreterRuntime()

		log := zerolog.Nop()
		txInvocator := NewTransactionInvocator(log)
		vm := New(rt)

		// deploy code to accounts
		whateverContractCode := `
			pub contract Whatever {
				pub fun say() {
					log("Düsseldorf")
				}
			}
		`

		deployContract := []byte(fmt.Sprintf(
			`
			 transaction {
			   prepare(signer: AuthAccount) {
				   signer.contracts.add(name: "Whatever", code: "%s".decodeHex())
			   }
			 }
	   `, hex.EncodeToString([]byte(whateverContractCode)),
		))

		procFrozen := Transaction(&flow.TransactionBody{Script: deployContract, Authorizers: []flow.Address{frozenAddress}, Payer: frozenAddress}, 0)
		procNotFrozen := Transaction(&flow.TransactionBody{Script: deployContract, Authorizers: []flow.Address{notFrozenAddress}, Payer: notFrozenAddress}, 0)
		deployContext := NewContext(zerolog.Nop(), WithServiceAccount(false), WithRestrictedDeployment(false), WithCadenceLogging(false))
		deployTxInvocator := NewTransactionInvocator(zerolog.Nop())
		deployRt := runtime.NewInterpreterRuntime()

		deployVm := New(deployRt)

		err := deployTxInvocator.Process(deployVm, deployContext, procFrozen, st)
		require.NoError(t, err)
		err = deployTxInvocator.Process(deployVm, deployContext, procNotFrozen, st)
		require.NoError(t, err)

		// both contracts should load now
		context := NewContext(log, WithCadenceLogging(true))

		code := func(a flow.Address) []byte {
			return []byte(fmt.Sprintf(`
				import Whatever from 0x%s
	
				transaction {
					execute {
						Whatever.say()
					}
				}
			`, a.String()))
		}

		// code from not frozen loads fine
		proc := Transaction(&flow.TransactionBody{Script: code(frozenAddress)}, 0)

		err = txInvocator.Process(vm, context, proc, st)
		require.NoError(t, err)
		require.Len(t, proc.Logs, 1)
		require.Contains(t, proc.Logs[0], "Düsseldorf")

		proc = Transaction(&flow.TransactionBody{Script: code(notFrozenAddress)}, 0)

		err = txInvocator.Process(vm, context, proc, st)
		require.NoError(t, err)
		require.Len(t, proc.Logs, 1)
		require.Contains(t, proc.Logs[0], "Düsseldorf")

		// freeze account
		err = accounts.SetAccountFrozen(frozenAddress, true)
		require.NoError(t, err)

		// make sure freeze status is correct
		frozen, err := accounts.GetAccountFrozen(frozenAddress)
		require.NoError(t, err)
		require.True(t, frozen)

		frozen, err = accounts.GetAccountFrozen(notFrozenAddress)
		require.NoError(t, err)
		require.False(t, frozen)

		// loading code from frozen account triggers error
		proc = Transaction(&flow.TransactionBody{Script: code(frozenAddress)}, 0)

		err = txInvocator.Process(vm, context, proc, st)
		require.Error(t, err)

		// find frozen account specific error
		require.IsType(t, runtime.Error{}, err)
		err = err.(runtime.Error).Err

		require.IsType(t, &runtime.ParsingCheckingError{}, err)
		err = err.(*runtime.ParsingCheckingError).Err

		require.IsType(t, &sema.CheckerError{}, err)
		checkerErr := err.(*sema.CheckerError)

		checkerErrors := checkerErr.ChildErrors()

		require.Len(t, checkerErrors, 2)
		require.IsType(t, &sema.ImportedProgramError{}, checkerErrors[0])

		importedCheckerErrors := checkerErrors[0].(*sema.ImportedProgramError).CheckerError.Errors
		require.Len(t, importedCheckerErrors, 1)

		require.IsType(t, &AccountFrozenError{}, importedCheckerErrors[0])
		require.Equal(t, frozenAddress, importedCheckerErrors[0].(*AccountFrozenError).Address)
	})
}
