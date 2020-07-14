package fvm_test

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/testutil"
	"github.com/dapperlabs/flow-go/fvm"
	"github.com/dapperlabs/flow-go/fvm/state"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func createAccount(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, ledger state.Ledger) flow.Address {
	ctx = fvm.NewContextFromParent(
		ctx,
		fvm.WithRestrictedAccountCreation(false),
		fvm.WithTransactionProcessors([]fvm.TransactionProcessor{
			fvm.NewTransactionInvocator(),
		}),
	)

	txBody := flow.NewTransactionBody().
		SetScript([]byte(createAccountTransaction)).
		AddAuthorizer(chain.ServiceAddress())

	tx := fvm.Transaction(txBody)

	err := vm.Run(ctx, tx, ledger)
	require.NoError(t, err)
	require.NoError(t, tx.Err)

	require.Equal(t, string(flow.EventAccountCreated), tx.Events[0].EventType.TypeID)

	address := flow.Address(tx.Events[0].Fields[0].(cadence.Address))

	return address
}

func addAccountKey(
	t *testing.T,
	vm *fvm.VirtualMachine,
	ctx fvm.Context,
	ledger state.Ledger,
	address flow.Address,
) flow.AccountPublicKey {
	publicKeyA, cadencePublicKey := newAccountKey(t)

	txBody := flow.NewTransactionBody().
		SetScript([]byte(addAccountKeyTransaction)).
		AddArgument(cadencePublicKey).
		AddAuthorizer(address)

	tx := fvm.Transaction(txBody)

	err := vm.Run(ctx, tx, ledger)
	require.NoError(t, err)
	require.NoError(t, tx.Err)

	return publicKeyA
}

func addAccountCreator(
	t *testing.T,
	vm *fvm.VirtualMachine,
	chain flow.Chain,
	ctx fvm.Context,
	ledger state.Ledger,
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

	tx := fvm.Transaction(txBody)

	err := vm.Run(ctx, tx, ledger)
	require.NoError(t, err)
	require.NoError(t, tx.Err)
}

func removeAccountCreator(
	t *testing.T,
	vm *fvm.VirtualMachine,
	chain flow.Chain,
	ctx fvm.Context,
	ledger state.Ledger,
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

	tx := fvm.Transaction(txBody)

	err := vm.Run(ctx, tx, ledger)
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
		fvm.WithTransactionProcessors([]fvm.TransactionProcessor{
			fvm.NewTransactionInvocator(),
		}),
	}

	t.Run("Single account",
		vmTest(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, ledger state.Ledger) {
			payer := createAccount(t, vm, chain, ctx, ledger)

			txBody := flow.NewTransactionBody().
				SetScript([]byte(createAccountTransaction)).
				AddAuthorizer(payer)

			tx := fvm.Transaction(txBody)

			err := vm.Run(ctx, tx, ledger)
			require.NoError(t, err)

			assert.NoError(t, tx.Err)

			require.Len(t, tx.Events, 1)
			assert.Equal(t, string(flow.EventAccountCreated), tx.Events[0].EventType.TypeID)

			address := flow.Address(tx.Events[0].Fields[0].(cadence.Address))

			account, err := vm.GetAccount(ctx, address, ledger)
			require.NoError(t, err)
			require.NotNil(t, account)
		}, options...),
	)

	t.Run("Multiple accounts",
		vmTest(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, ledger state.Ledger) {
			const count = 3

			payer := createAccount(t, vm, chain, ctx, ledger)

			txBody := flow.NewTransactionBody().
				SetScript([]byte(createMultipleAccountsTransaction)).
				AddAuthorizer(payer)

			tx := fvm.Transaction(txBody)

			err := vm.Run(ctx, tx, ledger)
			require.NoError(t, err)

			assert.NoError(t, tx.Err)

			require.Len(t, tx.Events, count)

			for i := 0; i < count; i++ {
				require.Equal(t, string(flow.EventAccountCreated), tx.Events[i].EventType.TypeID)

				address := flow.Address(tx.Events[i].Fields[0].(cadence.Address))

				account, err := vm.GetAccount(ctx, address, ledger)
				require.NoError(t, err)
				require.NotNil(t, account)
			}
		}, options...),
	)
}

func TestCreateAccount_WithRestrictedAccountCreation(t *testing.T) {

	options := []fvm.Option{
		fvm.WithRestrictedAccountCreation(true),
		fvm.WithTransactionProcessors([]fvm.TransactionProcessor{
			fvm.NewTransactionInvocator(),
		}),
	}

	t.Run("Unauthorized account payer",
		vmTest(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, ledger state.Ledger) {
			payer := createAccount(t, vm, chain, ctx, ledger)

			txBody := flow.NewTransactionBody().
				SetScript([]byte(createAccountTransaction)).
				AddAuthorizer(payer)

			tx := fvm.Transaction(txBody)

			err := vm.Run(ctx, tx, ledger)
			require.NoError(t, err)

			assert.Error(t, tx.Err)
		}, options...),
	)

	t.Run("Authorized account payer",
		vmTest(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, ledger state.Ledger) {
			txBody := flow.NewTransactionBody().
				SetScript([]byte(createAccountTransaction)).
				AddAuthorizer(chain.ServiceAddress())

			tx := fvm.Transaction(txBody)

			err := vm.Run(ctx, tx, ledger)
			require.NoError(t, err)

			assert.NoError(t, tx.Err)
		}, options...),
	)

	t.Run("Account payer added to allowlist",
		vmTest(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, ledger state.Ledger) {
			payer := createAccount(t, vm, chain, ctx, ledger)
			addAccountCreator(t, vm, chain, ctx, ledger, payer)

			txBody := flow.NewTransactionBody().
				SetScript([]byte(createAccountTransaction)).
				SetPayer(payer).
				AddAuthorizer(payer)

			tx := fvm.Transaction(txBody)

			err := vm.Run(ctx, tx, ledger)
			require.NoError(t, err)

			assert.NoError(t, tx.Err)
		}, options...),
	)

	t.Run("Account payer removed from allowlist",
		vmTest(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, ledger state.Ledger) {
			payer := createAccount(t, vm, chain, ctx, ledger)
			addAccountCreator(t, vm, chain, ctx, ledger, payer)

			txBody := flow.NewTransactionBody().
				SetScript([]byte(createAccountTransaction)).
				AddAuthorizer(payer)

			validTx := fvm.Transaction(txBody)

			err := vm.Run(ctx, validTx, ledger)
			require.NoError(t, err)

			assert.NoError(t, validTx.Err)

			removeAccountCreator(t, vm, chain, ctx, ledger, payer)

			invalidTx := fvm.Transaction(txBody)

			err = vm.Run(ctx, invalidTx, ledger)
			require.NoError(t, err)

			assert.Error(t, invalidTx.Err)
		}, options...),
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
		fvm.WithTransactionProcessors([]fvm.TransactionProcessor{
			fvm.NewTransactionInvocator(),
		}),
	}

	t.Run("Add to empty key list",
		vmTest(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, ledger state.Ledger) {
			address := createAccount(t, vm, chain, ctx, ledger)

			before, err := vm.GetAccount(ctx, address, ledger)
			require.NoError(t, err)
			assert.Empty(t, before.Keys)

			publicKeyA, cadencePublicKey := newAccountKey(t)

			txBody := flow.NewTransactionBody().
				SetScript([]byte(addAccountKeyTransaction)).
				AddArgument(cadencePublicKey).
				AddAuthorizer(address)

			tx := fvm.Transaction(txBody)

			err = vm.Run(ctx, tx, ledger)
			require.NoError(t, err)

			assert.NoError(t, tx.Err)

			after, err := vm.GetAccount(ctx, address, ledger)
			require.NoError(t, err)

			require.Len(t, after.Keys, 1)

			publicKeyB := after.Keys[0]

			assert.Equal(t, publicKeyA.PublicKey, publicKeyB.PublicKey)
			assert.Equal(t, publicKeyA.SignAlgo, publicKeyB.SignAlgo)
			assert.Equal(t, publicKeyA.HashAlgo, publicKeyB.HashAlgo)
			assert.Equal(t, publicKeyA.Weight, publicKeyB.Weight)
		}, options...),
	)

	t.Run("Add to non-empty key list",
		vmTest(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, ledger state.Ledger) {
			address := createAccount(t, vm, chain, ctx, ledger)

			publicKey1 := addAccountKey(t, vm, ctx, ledger, address)

			before, err := vm.GetAccount(ctx, address, ledger)
			require.NoError(t, err)
			assert.Len(t, before.Keys, 1)

			publicKey2, publicKey2Arg := newAccountKey(t)

			txBody := flow.NewTransactionBody().
				SetScript([]byte(addAccountKeyTransaction)).
				AddArgument(publicKey2Arg).
				AddAuthorizer(address)

			tx := fvm.Transaction(txBody)

			err = vm.Run(ctx, tx, ledger)
			require.NoError(t, err)

			assert.NoError(t, tx.Err)

			after, err := vm.GetAccount(ctx, address, ledger)
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
		}, options...),
	)

	t.Run("Invalid key",
		vmTest(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, ledger state.Ledger) {
			address := createAccount(t, vm, chain, ctx, ledger)

			invalidPublicKey := testutil.BytesToCadenceArray([]byte{1, 2, 3})
			invalidPublicKeyArg, err := jsoncdc.Encode(invalidPublicKey)
			require.NoError(t, err)

			txBody := flow.NewTransactionBody().
				SetScript([]byte(addAccountKeyTransaction)).
				AddArgument(invalidPublicKeyArg).
				AddAuthorizer(address)

			tx := fvm.Transaction(txBody)

			err = vm.Run(ctx, tx, ledger)
			require.NoError(t, err)

			assert.Error(t, tx.Err)

			after, err := vm.GetAccount(ctx, address, ledger)
			require.NoError(t, err)

			assert.Empty(t, after.Keys)
		}, options...),
	)

	t.Run("Multiple keys",
		vmTest(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, ledger state.Ledger) {
			address := createAccount(t, vm, chain, ctx, ledger)

			before, err := vm.GetAccount(ctx, address, ledger)
			require.NoError(t, err)
			assert.Empty(t, before.Keys)

			publicKey1, publicKey1Arg := newAccountKey(t)
			publicKey2, publicKey2Arg := newAccountKey(t)

			txBody := flow.NewTransactionBody().
				SetScript([]byte(addMultipleAccountKeysTransaction)).
				AddArgument(publicKey1Arg).
				AddArgument(publicKey2Arg).
				AddAuthorizer(address)

			tx := fvm.Transaction(txBody)

			err = vm.Run(ctx, tx, ledger)
			require.NoError(t, err)

			assert.NoError(t, tx.Err)

			after, err := vm.GetAccount(ctx, address, ledger)
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
		}, options...),
	)
}

func TestRemoveAccountKey(t *testing.T) {

	options := []fvm.Option{
		fvm.WithRestrictedAccountCreation(false),
		fvm.WithTransactionProcessors([]fvm.TransactionProcessor{
			fvm.NewTransactionInvocator(),
		}),
	}

	t.Run("Non-existent key",
		vmTest(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, ledger state.Ledger) {
			address := createAccount(t, vm, chain, ctx, ledger)

			const keyCount = 2

			for i := 0; i < keyCount; i++ {
				_ = addAccountKey(t, vm, ctx, ledger, address)
			}

			before, err := vm.GetAccount(ctx, address, ledger)
			require.NoError(t, err)
			assert.Len(t, before.Keys, keyCount)

			for _, keyIndex := range []int{-1, keyCount, keyCount + 1} {
				keyIndexArg, err := jsoncdc.Encode(cadence.NewInt(keyIndex))
				require.NoError(t, err)

				txBody := flow.NewTransactionBody().
					SetScript([]byte(removeAccountKeyTransaction)).
					AddArgument(keyIndexArg).
					AddAuthorizer(address)

				tx := fvm.Transaction(txBody)

				err = vm.Run(ctx, tx, ledger)
				require.NoError(t, err)

				assert.Error(t, tx.Err)
			}

			after, err := vm.GetAccount(ctx, address, ledger)
			require.NoError(t, err)
			assert.Len(t, after.Keys, keyCount)

			for _, publicKey := range after.Keys {
				assert.False(t, publicKey.Revoked)
			}
		}, options...),
	)

	t.Run("Existing key",
		vmTest(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, ledger state.Ledger) {
			address := createAccount(t, vm, chain, ctx, ledger)

			const keyCount = 2
			const keyIndex = keyCount - 1

			for i := 0; i < keyCount; i++ {
				_ = addAccountKey(t, vm, ctx, ledger, address)
			}

			before, err := vm.GetAccount(ctx, address, ledger)
			require.NoError(t, err)
			assert.Len(t, before.Keys, keyCount)

			keyIndexArg, err := jsoncdc.Encode(cadence.NewInt(keyIndex))
			require.NoError(t, err)

			txBody := flow.NewTransactionBody().
				SetScript([]byte(removeAccountKeyTransaction)).
				AddArgument(keyIndexArg).
				AddAuthorizer(address)

			tx := fvm.Transaction(txBody)

			err = vm.Run(ctx, tx, ledger)
			require.NoError(t, err)

			assert.NoError(t, tx.Err)

			after, err := vm.GetAccount(ctx, address, ledger)
			require.NoError(t, err)
			assert.Len(t, after.Keys, keyCount)

			for _, publicKey := range after.Keys[:len(after.Keys)-1] {
				assert.False(t, publicKey.Revoked)
			}

			assert.True(t, after.Keys[keyIndex].Revoked)
		}, options...),
	)

	t.Run("Multiple keys",
		vmTest(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, ledger state.Ledger) {
			address := createAccount(t, vm, chain, ctx, ledger)

			const keyCount = 2

			for i := 0; i < keyCount; i++ {
				_ = addAccountKey(t, vm, ctx, ledger, address)
			}

			before, err := vm.GetAccount(ctx, address, ledger)
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

			tx := fvm.Transaction(txBody)

			err = vm.Run(ctx, tx, ledger)
			require.NoError(t, err)

			assert.NoError(t, tx.Err)

			after, err := vm.GetAccount(ctx, address, ledger)
			require.NoError(t, err)
			assert.Len(t, after.Keys, keyCount)

			for _, publicKey := range after.Keys {
				assert.True(t, publicKey.Revoked)
			}
		}, options...),
	)
}
