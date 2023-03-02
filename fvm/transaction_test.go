package fvm_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/derived"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func makeTwoAccounts(
	t *testing.T,
	aPubKeys []flow.AccountPublicKey,
	bPubKeys []flow.AccountPublicKey,
) (
	flow.Address,
	flow.Address,
	*state.TransactionState,
) {

	txnState := state.NewTransactionState(
		utils.NewSimpleView(),
		state.DefaultParameters(),
	)

	a := flow.HexToAddress("1234")
	b := flow.HexToAddress("5678")

	// create accounts
	accounts := environment.NewAccounts(txnState)
	err := accounts.Create(aPubKeys, a)
	require.NoError(t, err)
	err = accounts.Create(bPubKeys, b)
	require.NoError(t, err)

	return a, b, txnState
}

func TestAccountFreezing(t *testing.T) {
	// TODO: remove freezing feature
	t.Skip("Skip as we are removing the freezing feature.")

	chain := flow.Mainnet.Chain()
	serviceAddress := chain.ServiceAddress()

	t.Run("setFrozenAccount can be enabled", func(t *testing.T) {
		address, _, st := makeTwoAccounts(t, nil, nil)
		accounts := environment.NewAccounts(st)
		derivedBlockData := derived.NewEmptyDerivedBlockData()

		// account should no be frozen
		frozen, err := accounts.GetAccountFrozen(address)
		require.NoError(t, err)
		require.False(t, frozen)

		code := fmt.Sprintf(`
			transaction {
				prepare(auth: AuthAccount) {
					setAccountFrozen(0x%s, true)
				}
			}
		`, address.String())

		tx := flow.TransactionBody{Script: []byte(code)}
		tx.AddAuthorizer(chain.ServiceAddress())
		proc := fvm.Transaction(&tx, derivedBlockData.NextTxIndexForTestingOnly())

		context := fvm.NewContext(
			fvm.WithChain(chain),
			fvm.WithAuthorizationChecksEnabled(false),
			fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
			fvm.WithDerivedBlockData(derivedBlockData))

		derivedBlockData = derived.NewEmptyDerivedBlockData()
		derivedTxnData, err := derivedBlockData.NewDerivedTransactionData(0, 0)
		require.NoError(t, err)

		err = fvm.Run(proc.NewExecutor(context, st, derivedTxnData))
		require.NoError(t, err)
		require.NoError(t, proc.Err)

		// account should be frozen now
		frozen, err = accounts.GetAccountFrozen(address)
		require.NoError(t, err)
		require.True(t, frozen)
	})

	t.Run("freezing account triggers program cache eviction", func(t *testing.T) {
		address, _, st := makeTwoAccounts(t, nil, nil)
		accounts := environment.NewAccounts(st)
		derivedBlockData := derived.NewEmptyDerivedBlockData()

		// account should no be frozen
		frozen, err := accounts.GetAccountFrozen(address)
		require.NoError(t, err)
		require.False(t, frozen)

		vm := fvm.NewVirtualMachine()

		// deploy code to account

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

		proc := fvm.Transaction(
			&flow.TransactionBody{Script: deployContract, Authorizers: []flow.Address{address}, Payer: address},
			derivedBlockData.NextTxIndexForTestingOnly())
		context := fvm.NewContext(
			fvm.WithServiceAccount(false),
			fvm.WithContractDeploymentRestricted(false),
			fvm.WithCadenceLogging(true),
			// run with limited processor to test just core of freezing, but still inside FVM
			fvm.WithAuthorizationChecksEnabled(false),
			fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
			fvm.WithDerivedBlockData(derivedBlockData))

		err = vm.Run(context, proc, st.ViewForTestingOnly())
		require.NoError(t, err)
		require.NoError(t, proc.Err)

		// contracts should load now

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

		proc = fvm.Transaction(
			&flow.TransactionBody{Script: code(address)},
			derivedBlockData.NextTxIndexForTestingOnly())
		err = vm.Run(context, proc, st.ViewForTestingOnly())
		require.NoError(t, err)
		require.NoError(t, proc.Err)
		require.Len(t, proc.Logs, 1)
		require.Contains(t, proc.Logs[0], "\"D\\u{fc}sseldorf\"")

		// verify cache is populated

		cadenceAddr := common.AddressLocation{
			Address: common.MustBytesToAddress(address[:]),
			Name:    "Whatever",
		}
		entry := derivedBlockData.GetProgramForTestingOnly(cadenceAddr)
		require.NotNil(t, entry)

		// freeze account

		freezeTx := fmt.Sprintf(`
				transaction {
					prepare(auth: AuthAccount) {
						setAccountFrozen(0x%s, true)
					}
				}
			`,
			address)
		tx := &flow.TransactionBody{Script: []byte(freezeTx)}
		tx.AddAuthorizer(chain.ServiceAddress())

		proc = fvm.Transaction(tx, derivedBlockData.NextTxIndexForTestingOnly())
		err = vm.Run(context, proc, st.ViewForTestingOnly())
		require.NoError(t, err)
		require.NoError(t, proc.Err)

		// verify cache is evicted

		entry = derivedBlockData.GetProgramForTestingOnly(cadenceAddr)
		require.Nil(t, entry)

		// loading code from frozen account triggers error

		proc = fvm.Transaction(
			&flow.TransactionBody{Script: code(address)},
			derivedBlockData.NextTxIndexForTestingOnly())

		err = vm.Run(context, proc, st.ViewForTestingOnly())
		require.NoError(t, err)
		require.Error(t, proc.Err)

		// find frozen account specific error
		require.True(t, errors.IsCadenceRuntimeError(proc.Err))

		var rtErr runtime.Error
		require.True(t, errors.As(proc.Err, &rtErr))

		err = rtErr.Err

		require.IsType(t, &runtime.ParsingCheckingError{}, err)
		err = err.(*runtime.ParsingCheckingError).Err

		require.IsType(t, &sema.CheckerError{}, err)
		checkerErr := err.(*sema.CheckerError)

		checkerErrors := checkerErr.ChildErrors()

		require.Len(t, checkerErrors, 2)
		require.IsType(t, &sema.ImportedProgramError{}, checkerErrors[0])

		importedCheckerError := checkerErrors[0].(*sema.ImportedProgramError).Err
		accountFrozenError := errors.FrozenAccountError{}

		require.True(t, errors.As(importedCheckerError, &accountFrozenError))
		require.Equal(t, address, accountFrozenError.Address())
	})

	t.Run("code from frozen account cannot be loaded", func(t *testing.T) {

		frozenAddress, notFrozenAddress, st := makeTwoAccounts(t, nil, nil)
		accounts := environment.NewAccounts(st)
		derivedBlockData := derived.NewEmptyDerivedBlockData()

		vm := fvm.NewVirtualMachine()

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

		procFrozen := fvm.Transaction(
			&flow.TransactionBody{Script: deployContract, Authorizers: []flow.Address{frozenAddress}, Payer: frozenAddress},
			derivedBlockData.NextTxIndexForTestingOnly())
		context := fvm.NewContext(
			fvm.WithServiceAccount(false),
			fvm.WithContractDeploymentRestricted(false),
			fvm.WithCadenceLogging(true),
			// run with limited processor to test just core of freezing, but still inside FVM
			fvm.WithAuthorizationChecksEnabled(false),
			fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
			fvm.WithDerivedBlockData(derivedBlockData))

		err := vm.Run(context, procFrozen, st.ViewForTestingOnly())
		require.NoError(t, err)
		require.NoError(t, procFrozen.Err)

		procNotFrozen := fvm.Transaction(
			&flow.TransactionBody{Script: deployContract, Authorizers: []flow.Address{notFrozenAddress}, Payer: notFrozenAddress},
			derivedBlockData.NextTxIndexForTestingOnly())
		err = vm.Run(context, procNotFrozen, st.ViewForTestingOnly())
		require.NoError(t, err)
		require.NoError(t, procNotFrozen.Err)

		// both contracts should load now

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
		proc := fvm.Transaction(
			&flow.TransactionBody{Script: code(frozenAddress), Payer: serviceAddress},
			derivedBlockData.NextTxIndexForTestingOnly())

		err = vm.Run(context, proc, st.ViewForTestingOnly())
		require.NoError(t, err)
		require.NoError(t, proc.Err)
		require.Len(t, proc.Logs, 1)
		require.Contains(t, proc.Logs[0], "\"D\\u{fc}sseldorf\"")

		proc = fvm.Transaction(
			&flow.TransactionBody{Script: code(notFrozenAddress)},
			derivedBlockData.NextTxIndexForTestingOnly())
		err = vm.Run(context, proc, st.ViewForTestingOnly())
		require.NoError(t, err)
		require.NoError(t, proc.Err)
		require.Len(t, proc.Logs, 1)
		require.Contains(t, proc.Logs[0], "\"D\\u{fc}sseldorf\"")

		// freeze account

		freezeTx := fmt.Sprintf(`
				transaction {
					prepare(auth: AuthAccount) {
						setAccountFrozen(0x%s, true)
					}
				}
			`,
			frozenAddress)
		tx := &flow.TransactionBody{Script: []byte(freezeTx)}
		tx.AddAuthorizer(chain.ServiceAddress())

		proc = fvm.Transaction(tx, derivedBlockData.NextTxIndexForTestingOnly())
		err = vm.Run(context, proc, st.ViewForTestingOnly())
		require.NoError(t, err)
		require.NoError(t, proc.Err)

		// make sure freeze status is correct
		frozen, err := accounts.GetAccountFrozen(frozenAddress)
		require.NoError(t, err)
		require.True(t, frozen)

		frozen, err = accounts.GetAccountFrozen(notFrozenAddress)
		require.NoError(t, err)
		require.False(t, frozen)

		// loading code from frozen account triggers error
		proc = fvm.Transaction(
			&flow.TransactionBody{Script: code(frozenAddress)},
			derivedBlockData.NextTxIndexForTestingOnly())

		err = vm.Run(context, proc, st.ViewForTestingOnly())
		require.NoError(t, err)
		require.Error(t, proc.Err)

		// find frozen account specific error
		require.True(t, errors.IsCadenceRuntimeError(proc.Err))

		var rtErr runtime.Error
		require.True(t, errors.As(proc.Err, &rtErr))

		err = rtErr.Err

		require.IsType(t, &runtime.ParsingCheckingError{}, err)
		err = err.(*runtime.ParsingCheckingError).Err

		require.IsType(t, &sema.CheckerError{}, err)
		checkerErr := err.(*sema.CheckerError)

		checkerErrors := checkerErr.ChildErrors()

		require.Len(t, checkerErrors, 2)
		require.IsType(t, &sema.ImportedProgramError{}, checkerErrors[0])

		importedCheckerError := checkerErrors[0].(*sema.ImportedProgramError).Err
		accountFrozenError := errors.FrozenAccountError{}

		require.True(t, errors.As(importedCheckerError, &accountFrozenError))
		require.Equal(t, frozenAddress, accountFrozenError.Address())
	})

	t.Run("service account cannot freeze itself", func(t *testing.T) {

		vm := fvm.NewVirtualMachine()
		// create default context
		derivedBlockData := derived.NewEmptyDerivedBlockData()
		context := fvm.NewContext(
			fvm.WithDerivedBlockData(derivedBlockData))

		ledger := testutil.RootBootstrappedLedger(vm, context)

		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := testutil.CreateAccounts(vm, ledger, derivedBlockData, privateKeys, context.Chain)
		require.NoError(t, err)

		address := accounts[0]

		codeAccount := fmt.Sprintf(`
			transaction {
				prepare(auth: AuthAccount) {}
				execute {
					setAccountFrozen(0x%s, true)
				}
			}
		`, address.String())

		codeService := fmt.Sprintf(`
			transaction {
				prepare(auth: AuthAccount) {}
				execute {
					setAccountFrozen(0x%s, true)
				}
			}
		`, serviceAddress.String())

		// sign tx by service account now
		txBody := &flow.TransactionBody{Script: []byte(codeAccount)}
		txBody.SetProposalKey(serviceAddress, 0, 0)
		txBody.SetPayer(serviceAddress)
		txBody.AddAuthorizer(serviceAddress)

		err = testutil.SignEnvelope(txBody, serviceAddress, unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody, derivedBlockData.NextTxIndexForTestingOnly())
		err = vm.Run(context, tx, ledger)
		require.NoError(t, err)
		require.NoError(t, tx.Err)

		accountsService := environment.NewAccounts(state.NewTransactionState(
			ledger,
			state.DefaultParameters(),
		))

		frozen, err := accountsService.GetAccountFrozen(address)
		require.NoError(t, err)
		require.True(t, frozen)

		// make sure service account is not frozen before
		frozen, err = accountsService.GetAccountFrozen(serviceAddress)
		require.NoError(t, err)
		require.False(t, frozen)

		// service account cannot be frozen
		txBody = &flow.TransactionBody{Script: []byte(codeService)}
		txBody.SetProposalKey(serviceAddress, 0, 1)
		txBody.SetPayer(serviceAddress)
		txBody.AddAuthorizer(serviceAddress)

		err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(txBody, serviceAddress, unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		tx = fvm.Transaction(txBody, derivedBlockData.NextTxIndexForTestingOnly())
		err = vm.Run(context, tx, ledger)
		require.NoError(t, err)
		require.Error(t, tx.Err)

		accountsService = environment.NewAccounts(state.NewTransactionState(
			ledger,
			state.DefaultParameters(),
		))

		frozen, err = accountsService.GetAccountFrozen(serviceAddress)
		require.NoError(t, err)
		require.False(t, frozen)
	})

	t.Run("frozen account fail just tx, not execution", func(t *testing.T) {

		frozenAddress, notFrozenAddress, st := makeTwoAccounts(t, nil, nil)
		accounts := environment.NewAccounts(st)

		vm := fvm.NewVirtualMachine()

		// deploy code to accounts
		whateverCode := []byte(`
			transaction {
				prepare(auth: AuthAccount) {
					log("Szczebrzeszyn")
				}
			}
		`)

		derivedBlockData := derived.NewEmptyDerivedBlockData()
		context := fvm.NewContext(
			fvm.WithServiceAccount(false),
			fvm.WithContractDeploymentRestricted(false),
			fvm.WithCadenceLogging(true),
			// run with limited processor to test just core of freezing, but still inside FVM
			fvm.WithAccountKeyWeightThreshold(-1),
			fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
			fvm.WithDerivedBlockData(derivedBlockData))

		// freeze account

		freezeTx := fmt.Sprintf(`
				transaction {
					prepare(auth: AuthAccount) {
						setAccountFrozen(0x%s, true)
					}
				}
			`,
			frozenAddress)
		tx := &flow.TransactionBody{Script: []byte(freezeTx)}
		tx.AddAuthorizer(chain.ServiceAddress())

		proc := fvm.Transaction(tx, derivedBlockData.NextTxIndexForTestingOnly())

		derivedTxnData, err := derivedBlockData.NewDerivedTransactionData(0, 0)
		require.NoError(t, err)

		err = fvm.Run(proc.NewExecutor(
			fvm.NewContextFromParent(
				context,
				fvm.WithAuthorizationChecksEnabled(false),
			),
			st,
			derivedTxnData))
		require.NoError(t, err)
		require.NoError(t, proc.Err)

		// make sure freeze status is correct
		var frozen bool
		frozen, err = accounts.GetAccountFrozen(frozenAddress)
		require.NoError(t, err)
		require.True(t, frozen)

		frozen, err = accounts.GetAccountFrozen(notFrozenAddress)
		require.NoError(t, err)
		require.False(t, frozen)

		t.Run("authorizer", func(t *testing.T) {

			notFrozenProc := fvm.Transaction(
				&flow.TransactionBody{
					Script:      whateverCode,
					Authorizers: []flow.Address{notFrozenAddress},
					ProposalKey: flow.ProposalKey{Address: notFrozenAddress},
					Payer:       notFrozenAddress},
				derivedBlockData.NextTxIndexForTestingOnly())
			// tx run OK by nonfrozen account
			err = vm.Run(context, notFrozenProc, st.ViewForTestingOnly())
			require.NoError(t, err)
			require.NoError(t, notFrozenProc.Err)

			frozenProc := fvm.Transaction(
				&flow.TransactionBody{
					Script:      whateverCode,
					Authorizers: []flow.Address{frozenAddress},
					ProposalKey: flow.ProposalKey{Address: notFrozenAddress},
					Payer:       notFrozenAddress},
				derivedBlockData.NextTxIndexForTestingOnly())
			err = vm.Run(context, frozenProc, st.ViewForTestingOnly())
			require.NoError(t, err)
			require.Error(t, frozenProc.Err)

			require.Equal(
				t,
				errors.ErrCodeFrozenAccountError,
				frozenProc.Err.Code())

			// The outer most coded error is a wrapper, not the actual
			// FrozenAccountError itself.
			_, ok := frozenProc.Err.(errors.FrozenAccountError)
			require.False(t, ok)

			// find frozen account specific error
			var accountFrozenErr errors.FrozenAccountError
			ok = errors.As(frozenProc.Err, &accountFrozenErr)
			require.True(t, ok)
			require.Equal(t, frozenAddress, accountFrozenErr.Address())
		})

		t.Run("proposal", func(t *testing.T) {

			notFrozenProc := fvm.Transaction(
				&flow.TransactionBody{
					Script:      whateverCode,
					Authorizers: []flow.Address{notFrozenAddress},
					ProposalKey: flow.ProposalKey{Address: notFrozenAddress},
					Payer:       notFrozenAddress,
				},
				derivedBlockData.NextTxIndexForTestingOnly())

			// tx run OK by nonfrozen account
			err = vm.Run(context, notFrozenProc, st.ViewForTestingOnly())
			require.NoError(t, err)
			require.NoError(t, notFrozenProc.Err)

			frozenProc := fvm.Transaction(
				&flow.TransactionBody{
					Script:      whateverCode,
					Authorizers: []flow.Address{notFrozenAddress},
					ProposalKey: flow.ProposalKey{Address: frozenAddress},
					Payer:       notFrozenAddress,
				},
				derivedBlockData.NextTxIndexForTestingOnly())
			err = vm.Run(context, frozenProc, st.ViewForTestingOnly())
			require.NoError(t, err)
			require.Error(t, frozenProc.Err)

			require.Equal(
				t,
				errors.ErrCodeFrozenAccountError,
				frozenProc.Err.Code())

			// The outer most coded error is a wrapper, not the actual
			// FrozenAccountError itself.
			_, ok := frozenProc.Err.(errors.FrozenAccountError)
			require.False(t, ok)

			// find frozen account specific error
			var accountFrozenErr errors.FrozenAccountError
			ok = errors.As(frozenProc.Err, &accountFrozenErr)
			require.True(t, ok)
			require.Equal(t, frozenAddress, accountFrozenErr.Address())
		})

		t.Run("payer", func(t *testing.T) {

			notFrozenProc := fvm.Transaction(
				&flow.TransactionBody{
					Script:      whateverCode,
					Authorizers: []flow.Address{notFrozenAddress},
					ProposalKey: flow.ProposalKey{Address: notFrozenAddress},
					Payer:       notFrozenAddress,
				},
				derivedBlockData.NextTxIndexForTestingOnly())

			// tx run OK by nonfrozen account
			err = vm.Run(context, notFrozenProc, st.ViewForTestingOnly())
			require.NoError(t, err)
			require.NoError(t, notFrozenProc.Err)

			frozenProc := fvm.Transaction(
				&flow.TransactionBody{
					Script:      whateverCode,
					Authorizers: []flow.Address{notFrozenAddress},
					ProposalKey: flow.ProposalKey{Address: notFrozenAddress},
					Payer:       frozenAddress,
				},
				derivedBlockData.NextTxIndexForTestingOnly())
			err = vm.Run(context, frozenProc, st.ViewForTestingOnly())
			require.NoError(t, err)
			require.Error(t, frozenProc.Err)

			require.Equal(
				t,
				errors.ErrCodeFrozenAccountError,
				frozenProc.Err.Code())

			// The outer most coded error is a wrapper, not the actual
			// FrozenAccountError itself.
			_, ok := frozenProc.Err.(errors.FrozenAccountError)
			require.False(t, ok)

			// find frozen account specific error
			var accountFrozenErr errors.FrozenAccountError
			ok = errors.As(frozenProc.Err, &accountFrozenErr)
			require.True(t, ok)
			require.Equal(t, frozenAddress, accountFrozenErr.Address())
		})
	})
}
