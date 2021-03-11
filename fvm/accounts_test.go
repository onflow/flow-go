package fvm_test

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func createAccount(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) flow.Address {
	ctx = fvm.NewContextFromParent(
		ctx,
		fvm.WithRestrictedAccountCreation(false),
		fvm.WithTransactionProcessors(fvm.NewTransactionInvocator(zerolog.Nop())),
	)

	txBody := flow.NewTransactionBody().
		SetScript([]byte(createAccountTransaction)).
		AddAuthorizer(chain.ServiceAddress())

	tx := fvm.Transaction(txBody, 0)

	err := vm.Run(ctx, tx, view, programs)
	require.NoError(t, err)
	require.NoError(t, tx.Err)

	require.Equal(t, flow.EventAccountCreated, tx.Events[0].Type)

	data, err := jsoncdc.Decode(tx.Events[0].Payload)
	require.NoError(t, err)
	address := flow.Address(data.(cadence.Event).Fields[0].(cadence.Address))

	return address
}

func addAccountKey(
	t *testing.T,
	vm *fvm.VirtualMachine,
	ctx fvm.Context,
	view state.View,
	programs *programs.Programs,
	address flow.Address,
) flow.AccountPublicKey {
	publicKeyA, cadencePublicKey := newAccountKey(t)

	txBody := flow.NewTransactionBody().
		SetScript([]byte(addAccountKeyTransaction)).
		AddArgument(cadencePublicKey).
		AddAuthorizer(address)

	tx := fvm.Transaction(txBody, 0)

	err := vm.Run(ctx, tx, view, programs)
	require.NoError(t, err)
	require.NoError(t, tx.Err)

	return publicKeyA
}

func addAccountCreator(
	t *testing.T,
	vm *fvm.VirtualMachine,
	chain flow.Chain,
	ctx fvm.Context,
	view state.View,
	programs *programs.Programs,
	account flow.Address,
) {
	script := []byte(
		fmt.Sprintf(addAccountCreatorTransactionTemplate,
			chain.ServiceAddress().String(),
			account.String(),
		),
	)

	txBody := flow.NewTransactionBody().
		SetScript(script).
		AddAuthorizer(chain.ServiceAddress())

	tx := fvm.Transaction(txBody, 0)

	err := vm.Run(ctx, tx, view, programs)
	require.NoError(t, err)
	require.NoError(t, tx.Err)
}

func removeAccountCreator(
	t *testing.T,
	vm *fvm.VirtualMachine,
	chain flow.Chain,
	ctx fvm.Context,
	view state.View,
	programs *programs.Programs,
	account flow.Address,
) {
	script := []byte(
		fmt.Sprintf(
			removeAccountCreatorTransactionTemplate,
			chain.ServiceAddress(),
			account.String(),
		),
	)

	txBody := flow.NewTransactionBody().
		SetScript(script).
		AddAuthorizer(chain.ServiceAddress())

	tx := fvm.Transaction(txBody, 0)

	err := vm.Run(ctx, tx, view, programs)
	require.NoError(t, err)
	require.NoError(t, tx.Err)
}

const createAccountTransaction = `
transaction {
  prepare(signer: AuthAccount) {
    let account = AuthAccount(payer: signer)
  }
}
`

const createMultipleAccountsTransaction = `
transaction {
  prepare(signer: AuthAccount) {
    let accountA = AuthAccount(payer: signer)
    let accountB = AuthAccount(payer: signer)
    let accountC = AuthAccount(payer: signer)
  }
}
`

const addAccountKeyTransaction = `
transaction(key: [UInt8]) {
  prepare(signer: AuthAccount) {
    signer.addPublicKey(key)
  }
}
`

const addMultipleAccountKeysTransaction = `
transaction(key1: [UInt8], key2: [UInt8]) {
  prepare(signer: AuthAccount) {
    signer.addPublicKey(key1)
    signer.addPublicKey(key2)
  }
}
`

const removeAccountKeyTransaction = `
transaction(key: Int) {
  prepare(signer: AuthAccount) {
    signer.removePublicKey(key)
  }
}
`

const removeMultipleAccountKeysTransaction = `
transaction(key1: Int, key2: Int) {
  prepare(signer: AuthAccount) {
    signer.removePublicKey(key1)
    signer.removePublicKey(key2)
  }
}
`

const removeAccountCreatorTransactionTemplate = `
import FlowServiceAccount from 0x%s
transaction {
	let serviceAccountAdmin: &FlowServiceAccount.Administrator
	prepare(signer: AuthAccount) {
		// Borrow reference to FlowServiceAccount Administrator resource.
		//
		self.serviceAccountAdmin = signer.borrow<&FlowServiceAccount.Administrator>(from: /storage/flowServiceAdmin)
			?? panic("Unable to borrow reference to administrator resource")
	}
	execute {
		// Remove account from account creator whitelist.
		//
		// Will emit AccountCreatorRemoved(accountCreator: accountCreator).
		//
		self.serviceAccountAdmin.removeAccountCreator(0x%s)
	}
}
`

const addAccountCreatorTransactionTemplate = `
import FlowServiceAccount from 0x%s
transaction {
	let serviceAccountAdmin: &FlowServiceAccount.Administrator
	prepare(signer: AuthAccount) {
		// Borrow reference to FlowServiceAccount Administrator resource.
		//
		self.serviceAccountAdmin = signer.borrow<&FlowServiceAccount.Administrator>(from: /storage/flowServiceAdmin)
			?? panic("Unable to borrow reference to administrator resource")
	}
	execute {
		// Add account to account creator whitelist.
		//
		// Will emit AccountCreatorAdded(accountCreator: accountCreator).
		//
		self.serviceAccountAdmin.addAccountCreator(0x%s)
	}
}
`

func newAccountKey(t *testing.T) (flow.AccountPublicKey, []byte) {
	privateKey, _ := unittest.AccountKeyFixture()
	publicKeyA := privateKey.PublicKey(fvm.AccountKeyWeightThreshold)

	encodedPublicKey, err := flow.EncodeRuntimeAccountPublicKey(publicKeyA)
	require.NoError(t, err)

	cadencePublicKey := testutil.BytesToCadenceArray(encodedPublicKey)
	encodedCadencePublicKey, err := jsoncdc.Encode(cadencePublicKey)
	require.NoError(t, err)

	return publicKeyA, encodedCadencePublicKey
}

func TestCreateAccount(t *testing.T) {

	options := []fvm.Option{
		fvm.WithRestrictedAccountCreation(false),
		fvm.WithTransactionProcessors(fvm.NewTransactionInvocator(zerolog.Nop())),
	}

	t.Run("Single account",
		newVMTest().withContextOptions(options...).
			run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				payer := createAccount(t, vm, chain, ctx, view, programs)

				txBody := flow.NewTransactionBody().
					SetScript([]byte(createAccountTransaction)).
					AddAuthorizer(payer)

				tx := fvm.Transaction(txBody, 0)

				err := vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				assert.NoError(t, tx.Err)

				require.Len(t, tx.Events, 1)

				require.Equal(t, flow.EventAccountCreated, tx.Events[0].Type)

				data, err := jsoncdc.Decode(tx.Events[0].Payload)
				require.NoError(t, err)
				address := flow.Address(data.(cadence.Event).Fields[0].(cadence.Address))

				account, err := vm.GetAccount(ctx, address, view, programs)
				require.NoError(t, err)
				require.NotNil(t, account)
			}),
	)

	t.Run("Multiple accounts",
		newVMTest().withContextOptions(options...).
			run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				const count = 3

				payer := createAccount(t, vm, chain, ctx, view, programs)

				txBody := flow.NewTransactionBody().
					SetScript([]byte(createMultipleAccountsTransaction)).
					AddAuthorizer(payer)

				tx := fvm.Transaction(txBody, 0)

				err := vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				assert.NoError(t, tx.Err)

				require.Len(t, tx.Events, count)

				for i := 0; i < count; i++ {
					require.Equal(t, flow.EventAccountCreated, tx.Events[i].Type)

					data, err := jsoncdc.Decode(tx.Events[i].Payload)
					require.NoError(t, err)
					address := flow.Address(data.(cadence.Event).Fields[0].(cadence.Address))

					account, err := vm.GetAccount(ctx, address, view, programs)
					require.NoError(t, err)
					require.NotNil(t, account)
				}
			}),
	)
}

func TestCreateAccount_WithRestrictedAccountCreation(t *testing.T) {

	options := []fvm.Option{
		fvm.WithRestrictedAccountCreation(true),
		fvm.WithTransactionProcessors(fvm.NewTransactionInvocator(zerolog.Nop())),
	}

	t.Run("Unauthorized account payer",
		newVMTest().withContextOptions(options...).
			run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				payer := createAccount(t, vm, chain, ctx, view, programs)

				txBody := flow.NewTransactionBody().
					SetScript([]byte(createAccountTransaction)).
					AddAuthorizer(payer)

				tx := fvm.Transaction(txBody, 0)

				err := vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				assert.Error(t, tx.Err)
			}),
	)

	t.Run("Authorized account payer",
		newVMTest().withContextOptions(options...).
			run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				txBody := flow.NewTransactionBody().
					SetScript([]byte(createAccountTransaction)).
					AddAuthorizer(chain.ServiceAddress())

				tx := fvm.Transaction(txBody, 0)

				err := vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				assert.NoError(t, tx.Err)
			}),
	)

	t.Run("Account payer added to allowlist",
		newVMTest().withContextOptions(options...).
			run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				payer := createAccount(t, vm, chain, ctx, view, programs)
				addAccountCreator(t, vm, chain, ctx, view, programs, payer)

				txBody := flow.NewTransactionBody().
					SetScript([]byte(createAccountTransaction)).
					SetPayer(payer).
					AddAuthorizer(payer)

				tx := fvm.Transaction(txBody, 0)

				err := vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				assert.NoError(t, tx.Err)
			}),
	)

	t.Run("Account payer removed from allowlist",
		newVMTest().withContextOptions(options...).
			run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				payer := createAccount(t, vm, chain, ctx, view, programs)
				addAccountCreator(t, vm, chain, ctx, view, programs, payer)

				txBody := flow.NewTransactionBody().
					SetScript([]byte(createAccountTransaction)).
					AddAuthorizer(payer)

				validTx := fvm.Transaction(txBody, 0)

				err := vm.Run(ctx, validTx, view, programs)
				require.NoError(t, err)

				assert.NoError(t, validTx.Err)

				removeAccountCreator(t, vm, chain, ctx, view, programs, payer)

				invalidTx := fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, invalidTx, view, programs)
				require.NoError(t, err)

				assert.Error(t, invalidTx.Err)
			}),
	)
}

func TestCreateAccount_WithFees(t *testing.T) {
	// TODO: add test cases for account fees
	// - Create account with sufficient balance
	// - Create account with insufficient balance
}

func TestUpdateAccountCode(t *testing.T) {
	// TODO: add test cases for updating account code
	// - empty code
	// - invalid Cadence code
	// - set new
	// - update existing
	// - remove existing
}

func TestAddAccountKey(t *testing.T) {

	options := []fvm.Option{
		fvm.WithRestrictedAccountCreation(false),
		fvm.WithTransactionProcessors(fvm.NewTransactionInvocator(zerolog.Nop())),
	}

	t.Run("Add to empty key list",
		newVMTest().withContextOptions(options...).
			run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				address := createAccount(t, vm, chain, ctx, view, programs)

				before, err := vm.GetAccount(ctx, address, view, programs)
				require.NoError(t, err)
				assert.Empty(t, before.Keys)

				publicKeyA, cadencePublicKey := newAccountKey(t)

				txBody := flow.NewTransactionBody().
					SetScript([]byte(addAccountKeyTransaction)).
					AddArgument(cadencePublicKey).
					AddAuthorizer(address)

				tx := fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				assert.NoError(t, tx.Err)

				after, err := vm.GetAccount(ctx, address, view, programs)
				require.NoError(t, err)

				require.Len(t, after.Keys, 1)

				publicKeyB := after.Keys[0]

				assert.Equal(t, publicKeyA.PublicKey, publicKeyB.PublicKey)
				assert.Equal(t, publicKeyA.SignAlgo, publicKeyB.SignAlgo)
				assert.Equal(t, publicKeyA.HashAlgo, publicKeyB.HashAlgo)
				assert.Equal(t, publicKeyA.Weight, publicKeyB.Weight)
			}),
	)

	t.Run("Add to non-empty key list",
		newVMTest().withContextOptions(options...).
			run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				address := createAccount(t, vm, chain, ctx, view, programs)

				publicKey1 := addAccountKey(t, vm, ctx, view, programs, address)

				before, err := vm.GetAccount(ctx, address, view, programs)
				require.NoError(t, err)
				assert.Len(t, before.Keys, 1)

				publicKey2, publicKey2Arg := newAccountKey(t)

				txBody := flow.NewTransactionBody().
					SetScript([]byte(addAccountKeyTransaction)).
					AddArgument(publicKey2Arg).
					AddAuthorizer(address)

				tx := fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				assert.NoError(t, tx.Err)

				after, err := vm.GetAccount(ctx, address, view, programs)
				require.NoError(t, err)

				expectedKeys := []flow.AccountPublicKey{
					publicKey1,
					publicKey2,
				}

				require.Len(t, after.Keys, len(expectedKeys))

				for i, expectedKey := range expectedKeys {
					actualKey := after.Keys[i]
					assert.Equal(t, i, actualKey.Index)
					assert.Equal(t, expectedKey.PublicKey, actualKey.PublicKey)
					assert.Equal(t, expectedKey.SignAlgo, actualKey.SignAlgo)
					assert.Equal(t, expectedKey.HashAlgo, actualKey.HashAlgo)
					assert.Equal(t, expectedKey.Weight, actualKey.Weight)
				}
			}),
	)

	t.Run("Invalid key",
		newVMTest().withContextOptions(options...).
			run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				address := createAccount(t, vm, chain, ctx, view, programs)

				invalidPublicKey := testutil.BytesToCadenceArray([]byte{1, 2, 3})
				invalidPublicKeyArg, err := jsoncdc.Encode(invalidPublicKey)
				require.NoError(t, err)

				txBody := flow.NewTransactionBody().
					SetScript([]byte(addAccountKeyTransaction)).
					AddArgument(invalidPublicKeyArg).
					AddAuthorizer(address)

				tx := fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				assert.Error(t, tx.Err)

				after, err := vm.GetAccount(ctx, address, view, programs)
				require.NoError(t, err)

				assert.Empty(t, after.Keys)
			}),
	)

	t.Run("Multiple keys",
		newVMTest().withContextOptions(options...).
			run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				address := createAccount(t, vm, chain, ctx, view, programs)

				before, err := vm.GetAccount(ctx, address, view, programs)
				require.NoError(t, err)
				assert.Empty(t, before.Keys)

				publicKey1, publicKey1Arg := newAccountKey(t)
				publicKey2, publicKey2Arg := newAccountKey(t)

				txBody := flow.NewTransactionBody().
					SetScript([]byte(addMultipleAccountKeysTransaction)).
					AddArgument(publicKey1Arg).
					AddArgument(publicKey2Arg).
					AddAuthorizer(address)

				tx := fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				assert.NoError(t, tx.Err)

				after, err := vm.GetAccount(ctx, address, view, programs)
				require.NoError(t, err)

				expectedKeys := []flow.AccountPublicKey{
					publicKey1,
					publicKey2,
				}

				require.Len(t, after.Keys, len(expectedKeys))

				for i, expectedKey := range expectedKeys {
					actualKey := after.Keys[i]
					assert.Equal(t, expectedKey.PublicKey, actualKey.PublicKey)
					assert.Equal(t, expectedKey.SignAlgo, actualKey.SignAlgo)
					assert.Equal(t, expectedKey.HashAlgo, actualKey.HashAlgo)
					assert.Equal(t, expectedKey.Weight, actualKey.Weight)
				}
			}),
	)
}

func TestRemoveAccountKey(t *testing.T) {

	options := []fvm.Option{
		fvm.WithRestrictedAccountCreation(false),
		fvm.WithTransactionProcessors(fvm.NewTransactionInvocator(zerolog.Nop())),
	}

	t.Run("Non-existent key",
		newVMTest().withContextOptions(options...).
			run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				address := createAccount(t, vm, chain, ctx, view, programs)

				const keyCount = 2

				for i := 0; i < keyCount; i++ {
					_ = addAccountKey(t, vm, ctx, view, programs, address)
				}

				before, err := vm.GetAccount(ctx, address, view, programs)
				require.NoError(t, err)
				assert.Len(t, before.Keys, keyCount)

				for i, keyIndex := range []int{-1, keyCount, keyCount + 1} {
					keyIndexArg, err := jsoncdc.Encode(cadence.NewInt(keyIndex))
					require.NoError(t, err)

					txBody := flow.NewTransactionBody().
						SetScript([]byte(removeAccountKeyTransaction)).
						AddArgument(keyIndexArg).
						AddAuthorizer(address)

					tx := fvm.Transaction(txBody, uint32(i))

					err = vm.Run(ctx, tx, view, programs)
					require.NoError(t, err)

					assert.Error(t, tx.Err)
				}

				after, err := vm.GetAccount(ctx, address, view, programs)
				require.NoError(t, err)
				assert.Len(t, after.Keys, keyCount)

				for _, publicKey := range after.Keys {
					assert.False(t, publicKey.Revoked)
				}
			}),
	)

	t.Run("Existing key",
		newVMTest().withContextOptions(options...).
			run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				address := createAccount(t, vm, chain, ctx, view, programs)

				const keyCount = 2
				const keyIndex = keyCount - 1

				for i := 0; i < keyCount; i++ {
					_ = addAccountKey(t, vm, ctx, view, programs, address)
				}

				before, err := vm.GetAccount(ctx, address, view, programs)
				require.NoError(t, err)
				assert.Len(t, before.Keys, keyCount)

				keyIndexArg, err := jsoncdc.Encode(cadence.NewInt(keyIndex))
				require.NoError(t, err)

				txBody := flow.NewTransactionBody().
					SetScript([]byte(removeAccountKeyTransaction)).
					AddArgument(keyIndexArg).
					AddAuthorizer(address)

				tx := fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				assert.NoError(t, tx.Err)

				after, err := vm.GetAccount(ctx, address, view, programs)
				require.NoError(t, err)
				assert.Len(t, after.Keys, keyCount)

				for _, publicKey := range after.Keys[:len(after.Keys)-1] {
					assert.False(t, publicKey.Revoked)
				}

				assert.True(t, after.Keys[keyIndex].Revoked)
			}),
	)

	t.Run("Multiple keys",
		newVMTest().withContextOptions(options...).
			run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				address := createAccount(t, vm, chain, ctx, view, programs)

				const keyCount = 2

				for i := 0; i < keyCount; i++ {
					_ = addAccountKey(t, vm, ctx, view, programs, address)
				}

				before, err := vm.GetAccount(ctx, address, view, programs)
				require.NoError(t, err)
				assert.Len(t, before.Keys, keyCount)

				txBody := flow.NewTransactionBody().
					SetScript([]byte(removeMultipleAccountKeysTransaction)).
					AddAuthorizer(address)

				for i := 0; i < keyCount; i++ {
					keyIndexArg, err := jsoncdc.Encode(cadence.NewInt(i))
					require.NoError(t, err)

					txBody.AddArgument(keyIndexArg)
				}

				tx := fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				assert.NoError(t, tx.Err)

				after, err := vm.GetAccount(ctx, address, view, programs)
				require.NoError(t, err)
				assert.Len(t, after.Keys, keyCount)

				for _, publicKey := range after.Keys {
					assert.True(t, publicKey.Revoked)
				}
			}),
	)
}
