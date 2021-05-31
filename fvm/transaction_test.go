package fvm_test

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func makeTwoAccounts(t *testing.T, aPubKeys []flow.AccountPublicKey, bPubKeys []flow.AccountPublicKey) (flow.Address, flow.Address, *state.StateHolder) {

	ledger := utils.NewSimpleView()
	sth := state.NewStateHolder(state.NewState(ledger))

	a := flow.HexToAddress("1234")
	b := flow.HexToAddress("5678")

	//create accounts
	accounts := state.NewAccounts(sth)
	err := accounts.Create(aPubKeys, a)
	require.NoError(t, err)
	err = accounts.Create(bPubKeys, b)
	require.NoError(t, err)

	return a, b, sth
}

func TestAccountFreezing(t *testing.T) {

	chain := flow.Mainnet.Chain()
	serviceAddress := chain.ServiceAddress()

	t.Run("setFrozenAccount can be enabled", func(t *testing.T) {

		address, _, st := makeTwoAccounts(t, nil, nil)
		accounts := state.NewAccounts(st)
		programsStorage := programs.NewEmptyPrograms()

		// account should no be frozen
		frozen, err := accounts.GetAccountFrozen(address)
		require.NoError(t, err)
		require.False(t, frozen)

		rt := fvm.NewInterpreterRuntime()
		log := zerolog.Nop()
		vm := fvm.NewVirtualMachine(rt)
		txInvocator := fvm.NewTransactionInvocator(log)

		code := fmt.Sprintf(`
			transaction {
				prepare(auth: AuthAccount) {
					setAccountFrozen(0x%s, true)
				}
			}
		`, address.String())

		tx := flow.TransactionBody{Script: []byte(code)}
		tx.AddAuthorizer(chain.ServiceAddress())
		proc := fvm.Transaction(&tx, 0)

		context := fvm.NewContext(log, fvm.WithAccountFreezeAvailable(false), fvm.WithChain(chain))

		err = txInvocator.Process(vm, &context, proc, st, programsStorage)
		require.Error(t, err)
		require.Contains(t, err.Error(), "cannot find")
		require.Contains(t, err.Error(), "setAccountFrozen")

		context = fvm.NewContext(log, fvm.WithAccountFreezeAvailable(true), fvm.WithChain(chain))

		err = txInvocator.Process(vm, &context, proc, st, programsStorage)
		require.NoError(t, err)

		// account should be frozen now
		frozen, err = accounts.GetAccountFrozen(address)
		require.NoError(t, err)
		require.True(t, frozen)
	})

	t.Run("frozen account is rejected", func(t *testing.T) {

		txChecker := fvm.NewTransactionAccountFrozenChecker()

		frozenAddress, notFrozenAddress, st := makeTwoAccounts(t, nil, nil)
		accounts := state.NewAccounts(st)
		programsStorage := programs.NewEmptyPrograms()

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
		tx := fvm.Transaction(&flow.TransactionBody{}, 0)
		err = txChecker.Process(nil, &fvm.Context{}, tx, st, programsStorage)
		require.NoError(t, err)

		tx = fvm.Transaction(&flow.TransactionBody{Authorizers: []flow.Address{notFrozenAddress}}, 0)
		err = txChecker.Process(nil, &fvm.Context{}, tx, st, programsStorage)
		require.NoError(t, err)

		tx = fvm.Transaction(&flow.TransactionBody{Authorizers: []flow.Address{frozenAddress}}, 0)
		err = txChecker.Process(nil, &fvm.Context{}, tx, st, programsStorage)
		require.Error(t, err)

		// all addresses must not be frozen
		tx = fvm.Transaction(&flow.TransactionBody{Authorizers: []flow.Address{frozenAddress, notFrozenAddress}}, 0)
		err = txChecker.Process(nil, &fvm.Context{}, tx, st, programsStorage)
		require.Error(t, err)

		// Payer should be part of authorizers account, but lets check it separately for completeness

		tx = fvm.Transaction(&flow.TransactionBody{Payer: notFrozenAddress}, 0)
		err = txChecker.Process(nil, &fvm.Context{}, tx, st, programsStorage)
		require.NoError(t, err)

		tx = fvm.Transaction(&flow.TransactionBody{Payer: frozenAddress}, 0)
		err = txChecker.Process(nil, &fvm.Context{}, tx, st, programsStorage)
		require.Error(t, err)

		// Proposal account

		tx = fvm.Transaction(&flow.TransactionBody{ProposalKey: flow.ProposalKey{Address: frozenAddress}}, 0)
		err = txChecker.Process(nil, &fvm.Context{}, tx, st, programsStorage)
		require.Error(t, err)

		tx = fvm.Transaction(&flow.TransactionBody{ProposalKey: flow.ProposalKey{Address: notFrozenAddress}}, 0)
		err = txChecker.Process(nil, &fvm.Context{}, tx, st, programsStorage)
		require.NoError(t, err)
	})

	t.Run("code from frozen account cannot be loaded", func(t *testing.T) {

		frozenAddress, notFrozenAddress, st := makeTwoAccounts(t, nil, nil)
		accounts := state.NewAccounts(st)
		programsStorage := programs.NewEmptyPrograms()

		rt := fvm.NewInterpreterRuntime()

		vm := fvm.NewVirtualMachine(rt)

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

		procFrozen := fvm.Transaction(&flow.TransactionBody{Script: deployContract, Authorizers: []flow.Address{frozenAddress}, Payer: frozenAddress}, 0)
		procNotFrozen := fvm.Transaction(&flow.TransactionBody{Script: deployContract, Authorizers: []flow.Address{notFrozenAddress}, Payer: notFrozenAddress}, 0)
		context := fvm.NewContext(zerolog.Nop(),
			fvm.WithServiceAccount(false),
			fvm.WithRestrictedDeployment(false),
			fvm.WithCadenceLogging(true),
			fvm.WithTransactionProcessors( // run with limited processor to test just core of freezing, but still inside FVM
				fvm.NewTransactionAccountFrozenChecker(),
				fvm.NewTransactionAccountFrozenEnabler(),
				fvm.NewTransactionInvocator(zerolog.Nop())))

		err := vm.Run(context, procFrozen, st.State().View(), programsStorage)
		require.NoError(t, err)
		require.NoError(t, procFrozen.Err)

		err = vm.Run(context, procNotFrozen, st.State().View(), programsStorage)
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
		proc := fvm.Transaction(&flow.TransactionBody{Script: code(frozenAddress), Payer: serviceAddress}, 0)

		err = vm.Run(context, proc, st.State().View(), programsStorage)
		require.NoError(t, err)
		require.NoError(t, proc.Err)
		require.Len(t, proc.Logs, 1)
		require.Contains(t, proc.Logs[0], "Düsseldorf")

		proc = fvm.Transaction(&flow.TransactionBody{Script: code(notFrozenAddress)}, 0)
		err = vm.Run(context, proc, st.State().View(), programsStorage)
		require.NoError(t, err)
		require.NoError(t, proc.Err)
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
		proc = fvm.Transaction(&flow.TransactionBody{Script: code(frozenAddress)}, 0)

		err = vm.Run(context, proc, st.State().View(), programsStorage)
		require.NoError(t, err)
		require.Error(t, proc.Err)

		// find frozen account specific error
		require.IsType(t, &errors.CadenceRuntimeError{}, proc.Err)
		err = proc.Err.(*errors.CadenceRuntimeError).Unwrap()

		require.IsType(t, &runtime.Error{}, err)
		err = err.(*runtime.Error).Err

		require.IsType(t, &runtime.ParsingCheckingError{}, err)
		err = err.(*runtime.ParsingCheckingError).Err

		require.IsType(t, &sema.CheckerError{}, err)
		checkerErr := err.(*sema.CheckerError)

		checkerErrors := checkerErr.ChildErrors()

		require.Len(t, checkerErrors, 2)
		require.IsType(t, &sema.ImportedProgramError{}, checkerErrors[0])

		importedCheckerError := checkerErrors[0].(*sema.ImportedProgramError).Err
		accountFrozenError := &errors.FrozenAccountError{}

		require.True(t, errors.As(importedCheckerError, &accountFrozenError))
		require.Equal(t, frozenAddress, accountFrozenError.Address())
	})

	t.Run("default settings allow only service account to freeze accounts", func(t *testing.T) {

		rt := fvm.NewInterpreterRuntime()
		log := zerolog.Nop()
		vm := fvm.NewVirtualMachine(rt)
		// create default context
		context := fvm.NewContext(log)
		programsStorage := programs.NewEmptyPrograms()

		ledger := testutil.RootBootstrappedLedger(vm, context)

		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := testutil.CreateAccounts(vm, ledger, programsStorage, privateKeys, context.Chain)
		require.NoError(t, err)

		address := accounts[0]

		code := fmt.Sprintf(`
			transaction {
				prepare(auth: AuthAccount) {}
				execute {
					setAccountFrozen(0x%s, true)
				}
			}
		`, address.String())

		txBody := &flow.TransactionBody{Script: []byte(code)}
		txBody.SetPayer(accounts[0])
		txBody.SetProposalKey(accounts[0], 0, 0)

		err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		tx := fvm.Transaction(txBody, 0)
		err = vm.Run(context, tx, ledger, programsStorage)
		require.NoError(t, err)
		require.Error(t, tx.Err)

		require.Contains(t, tx.Err.Error(), "cannot find")
		require.Contains(t, tx.Err.Error(), "setAccountFrozen")
		require.Equal(t, (&errors.CadenceRuntimeError{}).Code(), tx.Err.Code())

		// sign tx by service account now
		txBody = &flow.TransactionBody{Script: []byte(code)}
		txBody.AddAuthorizer(serviceAddress)
		txBody.SetPayer(serviceAddress)
		txBody.SetProposalKey(serviceAddress, 0, 0)

		err = testutil.SignPayload(txBody, serviceAddress, unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		err = testutil.SignEnvelope(txBody, serviceAddress, unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		tx = fvm.Transaction(txBody, 0)
		err = vm.Run(context, tx, ledger, programsStorage)
		require.NoError(t, err)

		require.NoError(t, tx.Err)

	})

	t.Run("service account cannot freeze itself", func(t *testing.T) {

		rt := fvm.NewInterpreterRuntime()
		log := zerolog.Nop()
		vm := fvm.NewVirtualMachine(rt)
		// create default context
		context := fvm.NewContext(log)
		programsStorage := programs.NewEmptyPrograms()

		ledger := testutil.RootBootstrappedLedger(vm, context)

		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := testutil.CreateAccounts(vm, ledger, programsStorage, privateKeys, context.Chain)
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

		err = testutil.SignPayload(txBody, serviceAddress, unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		err = testutil.SignEnvelope(txBody, serviceAddress, unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody, 0)
		err = vm.Run(context, tx, ledger, programsStorage)
		require.NoError(t, err)
		require.NoError(t, tx.Err)

		accountsService := state.NewAccounts(state.NewStateHolder(state.NewState(ledger)))

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

		tx = fvm.Transaction(txBody, 0)
		err = vm.Run(context, tx, ledger, programsStorage)
		require.NoError(t, err)
		require.Error(t, tx.Err)

		accountsService = state.NewAccounts(state.NewStateHolder(state.NewState(ledger)))

		frozen, err = accountsService.GetAccountFrozen(serviceAddress)
		require.NoError(t, err)
		require.False(t, frozen)
	})

	t.Run("frozen account fail just tx, not execution", func(t *testing.T) {

		frozenAddress, notFrozenAddress, st := makeTwoAccounts(t, nil, nil)
		accounts := state.NewAccounts(st)
		programsStorage := programs.NewEmptyPrograms()

		rt := fvm.NewInterpreterRuntime()
		vm := fvm.NewVirtualMachine(rt)

		// deploy code to accounts
		whateverCode := []byte(`
			transaction {
				prepare(auth: AuthAccount) {
					log("Szczebrzeszyn")
				}
			}
		`)

		context := fvm.NewContext(zerolog.Nop(),
			fvm.WithServiceAccount(false),
			fvm.WithRestrictedDeployment(false),
			fvm.WithCadenceLogging(true),
			fvm.WithTransactionProcessors( // run with limited processor to test just core of freezing, but still inside FVM
				fvm.NewTransactionAccountFrozenChecker(),
				fvm.NewTransactionAccountFrozenEnabler(),
				fvm.NewTransactionInvocator(zerolog.Nop())))

		err := accounts.SetAccountFrozen(frozenAddress, true)
		require.NoError(t, err)

		// make sure freeze status is correct
		frozen, err := accounts.GetAccountFrozen(frozenAddress)
		require.NoError(t, err)
		require.True(t, frozen)

		frozen, err = accounts.GetAccountFrozen(notFrozenAddress)
		require.NoError(t, err)
		require.False(t, frozen)

		t.Run("authorizer", func(t *testing.T) {

			notFrozenProc := fvm.Transaction(&flow.TransactionBody{Script: whateverCode, Authorizers: []flow.Address{notFrozenAddress}}, 0)
			frozenProc := fvm.Transaction(&flow.TransactionBody{Script: whateverCode, Authorizers: []flow.Address{frozenAddress}}, 0)

			// tx run OK by nonfrozen account
			err = vm.Run(context, notFrozenProc, st.State().View(), programsStorage)
			require.NoError(t, err)
			require.NoError(t, notFrozenProc.Err)

			err = vm.Run(context, frozenProc, st.State().View(), programsStorage)
			require.NoError(t, err)
			require.Error(t, frozenProc.Err)

			// find frozen account specific error
			require.IsType(t, &errors.FrozenAccountError{}, frozenProc.Err)
			accountFrozenError := frozenProc.Err.(*errors.FrozenAccountError)
			require.Equal(t, frozenAddress, accountFrozenError.Address())
		})

		t.Run("proposal", func(t *testing.T) {

			notFrozenProc := fvm.Transaction(&flow.TransactionBody{Script: whateverCode, Authorizers: []flow.Address{serviceAddress}, ProposalKey: flow.ProposalKey{Address: notFrozenAddress}}, 0)
			frozenProc := fvm.Transaction(&flow.TransactionBody{Script: whateverCode, Authorizers: []flow.Address{serviceAddress}, ProposalKey: flow.ProposalKey{Address: frozenAddress}}, 0)

			// tx run OK by nonfrozen account
			err = vm.Run(context, notFrozenProc, st.State().View(), programsStorage)
			require.NoError(t, err)
			require.NoError(t, notFrozenProc.Err)

			err = vm.Run(context, frozenProc, st.State().View(), programsStorage)
			require.NoError(t, err)
			require.Error(t, frozenProc.Err)

			// find frozen account specific error
			require.IsType(t, &errors.FrozenAccountError{}, frozenProc.Err)
			accountFrozenError := frozenProc.Err.(*errors.FrozenAccountError)
			require.Equal(t, frozenAddress, accountFrozenError.Address())
		})

		t.Run("payer", func(t *testing.T) {

			notFrozenProc := fvm.Transaction(&flow.TransactionBody{Script: whateverCode, Authorizers: []flow.Address{serviceAddress}, Payer: notFrozenAddress}, 0)
			frozenProc := fvm.Transaction(&flow.TransactionBody{Script: whateverCode, Authorizers: []flow.Address{serviceAddress}, Payer: frozenAddress}, 0)

			// tx run OK by nonfrozen account
			err = vm.Run(context, notFrozenProc, st.State().View(), programsStorage)
			require.NoError(t, err)
			require.NoError(t, notFrozenProc.Err)

			err = vm.Run(context, frozenProc, st.State().View(), programsStorage)
			require.NoError(t, err)
			require.Error(t, frozenProc.Err)

			// find frozen account specific error
			require.IsType(t, &errors.FrozenAccountError{}, frozenProc.Err)
			accountFrozenError := frozenProc.Err.(*errors.FrozenAccountError)
			require.Equal(t, frozenAddress, accountFrozenError.Address())
		})

	})
}

func TestHashing(t *testing.T) {

	t.Parallel()

	rt := fvm.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.NewVirtualMachine(rt)

	ctx := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	ledger := testutil.RootBootstrappedLedger(vm, ctx)

	hashScript := func(hashName string) []byte {
		return []byte(fmt.Sprintf(
			`
				import Crypto
				
				pub fun main(data: [UInt8]): [UInt8] {
					return Crypto.hash(data, algorithm: HashAlgorithm.%s)
				}
			`, hashName))
	}
	hashWithTagScript := func(hashName string) []byte {
		return []byte(fmt.Sprintf(
			`
				import Crypto
				
				pub fun main(data: [UInt8], tag: String): [UInt8] {
					return Crypto.hashWithTag(data, tag: tag, algorithm: HashAlgorithm.%s)
				}
			`, hashName))
	}

	data := []byte("some random message")
	encodedBytes := make([]cadence.Value, len(data))
	for i := range encodedBytes {
		encodedBytes[i] = cadence.NewUInt8(data[i])
	}
	cadenceData := jsoncdc.MustEncode(cadence.NewArray(encodedBytes))

	// ===== Test Cases =====
	cases := []struct {
		Algo    runtime.HashAlgorithm
		WithTag bool
		Tag     string
		Check   func(t *testing.T, result string, scriptErr errors.Error, executionErr error)
	}{
		{
			Algo:    runtime.HashAlgorithmSHA2_256,
			WithTag: false,
			Check: func(t *testing.T, result string, scriptErr errors.Error, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "68fb87dfba69b956f4ba98b748a75a604f99b38a4f2740290037957f7e830da8", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmSHA2_384,
			WithTag: false,
			Check: func(t *testing.T, result string, scriptErr errors.Error, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "a9b3e62ab9b2a33020e015f245b82e063afd1398211326408bc8fc31c2c15859594b0aee263fbb02f6d8b5065ad49df2", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmSHA3_256,
			WithTag: false,
			Check: func(t *testing.T, result string, scriptErr errors.Error, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "38effea5ab9082a2cb0dc9adfafaf88523e8f3ce74bfbeac85ffc719cc2c4677", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmSHA3_384,
			WithTag: false,
			Check: func(t *testing.T, result string, scriptErr errors.Error, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "f41e8de9af0c1f46fc56d5a776f1bd500530879a85f3b904821810295927e13a54f3e936dddb84669021052eb12966c3", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmSHA2_256,
			WithTag: true,
			Tag:     "some_tag",
			Check: func(t *testing.T, result string, scriptErr errors.Error, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "cd323b6743f8c72d1b19697aebb0c6c90240c258cd58a2f183c4fde9753143e2", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmSHA2_384,
			WithTag: true,
			Tag:     "some_tag",
			Check: func(t *testing.T, result string, scriptErr errors.Error, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "bb7c965165ab02e8196cc2ba1159a3212ed94b8ca27b34698a08bd8f1dcfe2cd5806561e3d66fde304f786069fb89094", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmSHA3_256,
			WithTag: true,
			Tag:     "some_tag",
			Check: func(t *testing.T, result string, scriptErr errors.Error, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "8cd65b26b10c596628357d9ab47c4730228d27db11580ddd584e490107cb9c76", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmSHA3_384,
			WithTag: true,
			Tag:     "some_tag",
			Check: func(t *testing.T, result string, scriptErr errors.Error, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "8f3d63c56dda4ea9dd638a22bda104b1e272feb11c415ac48f4d3dd1dfd3de5f091a5417501615968d5e3729a74f03fa", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmKMAC128_BLS_BLS12_381,
			WithTag: false,
			Check: func(t *testing.T, result string, scriptErr errors.Error, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "627d7e8fe50384601ca550ceecb61c23e9cbde7feb75ae6b53227f128f2dc3b78b543a044058403e4822f88cb7040d90d588c9e8575f0de3012fe7edaf02b9997a8a5fad234d21b2af359ec3abaeaf4a7ef60e5f04623a983bd5e071f4113678710e910d48ac4d1713073a707ab9057867e0ba32aca6b33010b1d20b8006dd25", result)
			},
		},
		{
			Algo:    runtime.HashAlgorithmKMAC128_BLS_BLS12_381,
			WithTag: true,
			Tag:     "some_tag",
			Check: func(t *testing.T, result string, scriptErr errors.Error, executionErr error) {
				require.NoError(t, scriptErr)
				require.NoError(t, executionErr)
				require.Equal(t, "dc6889f9ca46803a9c7759068989dfc3cffe632fd991e25f6589603c73b7891e2f4736eebe5248f211bbddaa3d763b1b9318185eaf3ab3bfd6f159f345c3148795e4ff3ad376c98d5616febebcf4520ca2a83dda4be2f98b1ead9fb5a622355305b156e06db173a9e1d7af973b11acc1e714cd3aa0fb367dfaadc5a957b4742b", result)
			},
		},
	}
	// ======================

	for i, c := range cases {
		t.Run(fmt.Sprintf("case %d: %s with tag: %v", i, c.Algo, c.WithTag), func(t *testing.T) {
			code := hashScript(c.Algo.Name())
			if c.WithTag {
				code = hashWithTagScript(c.Algo.Name())
			}

			script := fvm.Script(code)

			if c.WithTag {
				script = script.WithArguments(
					cadenceData,
					jsoncdc.MustEncode(cadence.String(c.Tag)),
				)
			} else {
				script = script.WithArguments(
					cadenceData,
				)
			}

			err := vm.Run(ctx, script, ledger, programs.NewEmptyPrograms())

			byteResult := make([]byte, 0)
			if err == nil && script.Err == nil {
				cadenceArray := script.Value.(cadence.Array)
				for _, value := range cadenceArray.Values {
					byteResult = append(byteResult, value.(cadence.UInt8).ToGoValue().(uint8))
				}
			}

			c.Check(t, hex.EncodeToString(byteResult), script.Err, err)
		})
	}
}
