package fvm_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime/format"
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
		fvm.WithTransactionProcessors(fvm.NewTransactionInvoker(zerolog.Nop())),
	)

	txBody := flow.NewTransactionBody().
		SetScript([]byte(createAccountTransaction)).
		AddAuthorizer(chain.ServiceAddress())

	tx := fvm.Transaction(txBody, 0)

	err := vm.Run(ctx, tx, view, programs)
	require.NoError(t, err)
	require.NoError(t, tx.Err)

	accountCreatedEvents := filterAccountCreatedEvents(tx.Events)

	require.Len(t, accountCreatedEvents, 1)

	data, err := jsoncdc.Decode(accountCreatedEvents[0].Payload)
	require.NoError(t, err)
	address := flow.Address(data.(cadence.Event).Fields[0].(cadence.Address))

	return address
}

type accountKeyAPIVersion string

const (
	accountKeyAPIVersionV1 accountKeyAPIVersion = "V1"
	accountKeyAPIVersionV2 accountKeyAPIVersion = "V2"
)

func addAccountKey(
	t *testing.T,
	vm *fvm.VirtualMachine,
	ctx fvm.Context,
	view state.View,
	programs *programs.Programs,
	address flow.Address,
	apiVersion accountKeyAPIVersion,
) flow.AccountPublicKey {

	privateKey, err := unittest.AccountKeyDefaultFixture()
	require.NoError(t, err)

	publicKeyA, cadencePublicKey := newAccountKey(t, privateKey, apiVersion)

	var addAccountKeyTx accountKeyAPIVersion
	if apiVersion == accountKeyAPIVersionV1 {
		addAccountKeyTx = addAccountKeyTransaction
	} else {
		addAccountKeyTx = addAccountKeyTransactionV2
	}

	txBody := flow.NewTransactionBody().
		SetScript([]byte(addAccountKeyTx)).
		AddArgument(cadencePublicKey).
		AddAuthorizer(address)

	tx := fvm.Transaction(txBody, 0)

	err = vm.Run(ctx, tx, view, programs)
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
const addAccountKeyTransactionV2 = `
transaction(key: [UInt8]) {
  prepare(signer: AuthAccount) {
    let publicKey = PublicKey(
	  publicKey: key,
	  signatureAlgorithm: SignatureAlgorithm.ECDSA_P256
	)
    signer.keys.add(
      publicKey: publicKey,
      hashAlgorithm: HashAlgorithm.SHA3_256,
      weight: 1000.0
    )
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

const addMultipleAccountKeysTransactionV2 = `
transaction(key1: [UInt8], key2: [UInt8]) {
  prepare(signer: AuthAccount) {
    for key in [key1, key2] {
      let publicKey = PublicKey(
	    publicKey: key,
	    signatureAlgorithm: SignatureAlgorithm.ECDSA_P256
	  )
      signer.keys.add(
        publicKey: publicKey,
        hashAlgorithm: HashAlgorithm.SHA3_256,
        weight: 1000.0
      )
    }
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

const revokeAccountKeyTransaction = `
transaction(keyIndex: Int) {
  prepare(signer: AuthAccount) {
    signer.keys.revoke(keyIndex: keyIndex)
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

const revokeMultipleAccountKeysTransaction = `
transaction(keyIndex1: Int, keyIndex2: Int) {
  prepare(signer: AuthAccount) {
    for keyIndex in [keyIndex1, keyIndex2] {
      signer.keys.revoke(keyIndex: keyIndex)
    }
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

const getAccountKeyTransaction = `
transaction(keyIndex: Int) {
  prepare(signer: AuthAccount) {
    var key :AccountKey? = signer.keys.get(keyIndex: keyIndex)
    log(key)
  }
}
`

const getMultipleAccountKeysTransaction = `
transaction(keyIndex1: Int, keyIndex2: Int) {
  prepare(signer: AuthAccount) {
    for keyIndex in [keyIndex1, keyIndex2] {
      var key :AccountKey? = signer.keys.get(keyIndex: keyIndex)
      log(key)
    }
  }
}
`

func newAccountKey(
	t *testing.T,
	privateKey *flow.AccountPrivateKey,
	apiVersion accountKeyAPIVersion,
) (
	publicKey flow.AccountPublicKey,
	encodedCadencePublicKey []byte,
) {
	publicKey = privateKey.PublicKey(fvm.AccountKeyWeightThreshold)

	var publicKeyBytes []byte
	if apiVersion == accountKeyAPIVersionV1 {
		var err error
		publicKeyBytes, err = flow.EncodeRuntimeAccountPublicKey(publicKey)
		require.NoError(t, err)
	} else {
		publicKeyBytes = publicKey.PublicKey.Encode()
	}

	cadencePublicKey := testutil.BytesToCadenceArray(publicKeyBytes)
	encodedCadencePublicKey, err := jsoncdc.Encode(cadencePublicKey)
	require.NoError(t, err)

	return publicKey, encodedCadencePublicKey
}

func TestCreateAccount(t *testing.T) {

	options := []fvm.Option{
		fvm.WithTransactionProcessors(fvm.NewTransactionInvoker(zerolog.Nop())),
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

				accountCreatedEvents := filterAccountCreatedEvents(tx.Events)
				require.Len(t, accountCreatedEvents, 1)

				data, err := jsoncdc.Decode(accountCreatedEvents[0].Payload)
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

				accountCreatedEventCount := 0
				for i := 0; i < len(tx.Events); i++ {
					if tx.Events[i].Type != flow.EventAccountCreated {
						continue
					}
					accountCreatedEventCount += 1

					data, err := jsoncdc.Decode(tx.Events[i].Payload)
					require.NoError(t, err)
					address := flow.Address(data.(cadence.Event).Fields[0].(cadence.Address))

					account, err := vm.GetAccount(ctx, address, view, programs)
					require.NoError(t, err)
					require.NotNil(t, account)
				}
				require.Equal(t, count, accountCreatedEventCount)
			}),
	)
}

func TestCreateAccount_WithRestrictedAccountCreation(t *testing.T) {

	options := []fvm.Option{
		fvm.WithTransactionProcessors(fvm.NewTransactionInvoker(zerolog.Nop())),
	}

	t.Run("Unauthorized account payer",
		newVMTest().
			withContextOptions(options...).
			withBootstrapProcedureOptions(fvm.WithRestrictedAccountCreationEnabled(true)).
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
			withBootstrapProcedureOptions(fvm.WithRestrictedAccountCreationEnabled(true)).
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
			withBootstrapProcedureOptions(fvm.WithRestrictedAccountCreationEnabled(true)).
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
			withBootstrapProcedureOptions(fvm.WithRestrictedAccountCreationEnabled(true)).
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
		fvm.WithTransactionProcessors(fvm.NewTransactionInvoker(zerolog.Nop())),
	}

	type addKeyTest struct {
		source     string
		apiVersion accountKeyAPIVersion
	}

	// Add a single key

	singleKeyTests := []addKeyTest{
		{
			source:     addAccountKeyTransaction,
			apiVersion: accountKeyAPIVersionV1,
		},
		{
			source:     addAccountKeyTransactionV2,
			apiVersion: accountKeyAPIVersionV2,
		},
	}

	for _, test := range singleKeyTests {

		t.Run(fmt.Sprintf("Add to empty key list %s", test.apiVersion),
			newVMTest().withContextOptions(options...).
				run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
					address := createAccount(t, vm, chain, ctx, view, programs)

					before, err := vm.GetAccount(ctx, address, view, programs)
					require.NoError(t, err)
					assert.Empty(t, before.Keys)

					privateKey, err := unittest.AccountKeyDefaultFixture()
					require.NoError(t, err)

					publicKeyA, cadencePublicKey := newAccountKey(t, privateKey, test.apiVersion)

					txBody := flow.NewTransactionBody().
						SetScript([]byte(test.source)).
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

		t.Run(fmt.Sprintf("Add to non-empty key list %s", test.apiVersion),
			newVMTest().withContextOptions(options...).
				run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
					address := createAccount(t, vm, chain, ctx, view, programs)

					publicKey1 := addAccountKey(t, vm, ctx, view, programs, address, test.apiVersion)

					before, err := vm.GetAccount(ctx, address, view, programs)
					require.NoError(t, err)
					assert.Len(t, before.Keys, 1)

					privateKey, err := unittest.AccountKeyDefaultFixture()
					require.NoError(t, err)

					publicKey2, publicKey2Arg := newAccountKey(t, privateKey, test.apiVersion)

					txBody := flow.NewTransactionBody().
						SetScript([]byte(test.source)).
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

		t.Run(fmt.Sprintf("Invalid key %s", test.apiVersion),
			newVMTest().withContextOptions(options...).
				run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
					address := createAccount(t, vm, chain, ctx, view, programs)

					invalidPublicKey := testutil.BytesToCadenceArray([]byte{1, 2, 3})
					invalidPublicKeyArg, err := jsoncdc.Encode(invalidPublicKey)
					require.NoError(t, err)

					txBody := flow.NewTransactionBody().
						SetScript([]byte(test.source)).
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
	}

	// Add multiple keys

	multipleKeysTests := []addKeyTest{
		{
			source:     addMultipleAccountKeysTransaction,
			apiVersion: accountKeyAPIVersionV1,
		},
		{
			source:     addMultipleAccountKeysTransactionV2,
			apiVersion: accountKeyAPIVersionV2,
		},
	}

	for _, test := range multipleKeysTests {
		t.Run(fmt.Sprintf("Multiple keys %s", test.apiVersion),
			newVMTest().withContextOptions(options...).
				run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
					address := createAccount(t, vm, chain, ctx, view, programs)

					before, err := vm.GetAccount(ctx, address, view, programs)
					require.NoError(t, err)
					assert.Empty(t, before.Keys)

					privateKey1, err := unittest.AccountKeyDefaultFixture()
					require.NoError(t, err)

					privateKey2, err := unittest.AccountKeyDefaultFixture()
					require.NoError(t, err)

					publicKey1, publicKey1Arg := newAccountKey(t, privateKey1, test.apiVersion)
					publicKey2, publicKey2Arg := newAccountKey(t, privateKey2, test.apiVersion)

					txBody := flow.NewTransactionBody().
						SetScript([]byte(test.source)).
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

	t.Run("Invalid hash algorithms", func(t *testing.T) {

		for _, hashAlgo := range []string{"SHA2_384", "SHA3_384"} {

			t.Run(hashAlgo,
				newVMTest().withContextOptions(options...).
					run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
						address := createAccount(t, vm, chain, ctx, view, programs)

						privateKey, err := unittest.AccountKeyDefaultFixture()
						require.NoError(t, err)

						_, publicKeyArg := newAccountKey(t, privateKey, accountKeyAPIVersionV2)

						txBody := flow.NewTransactionBody().
							SetScript([]byte(fmt.Sprintf(
								`
								transaction(key: [UInt8]) {
								  prepare(signer: AuthAccount) {
								    let publicKey = PublicKey(
									  publicKey: key,
									  signatureAlgorithm: SignatureAlgorithm.ECDSA_P256
									)
								    signer.keys.add(
								      publicKey: publicKey,
								      hashAlgorithm: HashAlgorithm.%s,
								      weight: 1000.0
								    )
								  }
								}
								`,
								hashAlgo,
							))).
							AddArgument(publicKeyArg).
							AddAuthorizer(address)

						tx := fvm.Transaction(txBody, 0)

						err = vm.Run(ctx, tx, view, programs)
						require.NoError(t, err)

						require.Error(t, tx.Err)
						assert.Contains(t, tx.Err.Error(), "hashing algorithm type not supported")

						after, err := vm.GetAccount(ctx, address, view, programs)
						require.NoError(t, err)

						assert.Empty(t, after.Keys)
					}),
			)
		}
	})
}

func TestRemoveAccountKey(t *testing.T) {

	options := []fvm.Option{
		fvm.WithTransactionProcessors(fvm.NewTransactionInvoker(zerolog.Nop())),
	}

	type removeKeyTest struct {
		source      string
		apiVersion  accountKeyAPIVersion
		expectError bool
	}

	// Remove a single key

	singleKeyTests := []removeKeyTest{
		{
			source:      removeAccountKeyTransaction,
			apiVersion:  accountKeyAPIVersionV1,
			expectError: true,
		},
		{
			source:      revokeAccountKeyTransaction,
			apiVersion:  accountKeyAPIVersionV2,
			expectError: false,
		},
	}

	for _, test := range singleKeyTests {

		t.Run(fmt.Sprintf("Non-existent key %s", test.apiVersion),
			newVMTest().withContextOptions(options...).
				run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
					address := createAccount(t, vm, chain, ctx, view, programs)

					const keyCount = 2

					for i := 0; i < keyCount; i++ {
						_ = addAccountKey(t, vm, ctx, view, programs, address, test.apiVersion)
					}

					before, err := vm.GetAccount(ctx, address, view, programs)
					require.NoError(t, err)
					assert.Len(t, before.Keys, keyCount)

					for i, keyIndex := range []int{-1, keyCount, keyCount + 1} {
						keyIndexArg, err := jsoncdc.Encode(cadence.NewInt(keyIndex))
						require.NoError(t, err)

						txBody := flow.NewTransactionBody().
							SetScript([]byte(test.source)).
							AddArgument(keyIndexArg).
							AddAuthorizer(address)

						tx := fvm.Transaction(txBody, uint32(i))

						err = vm.Run(ctx, tx, view, programs)
						require.NoError(t, err)

						if test.expectError {
							assert.Error(t, tx.Err)
						} else {
							assert.NoError(t, tx.Err)
						}
					}

					after, err := vm.GetAccount(ctx, address, view, programs)
					require.NoError(t, err)
					assert.Len(t, after.Keys, keyCount)

					for _, publicKey := range after.Keys {
						assert.False(t, publicKey.Revoked)
					}
				}),
		)

		t.Run(fmt.Sprintf("Existing key %s", test.apiVersion),
			newVMTest().withContextOptions(options...).
				run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
					address := createAccount(t, vm, chain, ctx, view, programs)

					const keyCount = 2
					const keyIndex = keyCount - 1

					for i := 0; i < keyCount; i++ {
						_ = addAccountKey(t, vm, ctx, view, programs, address, test.apiVersion)
					}

					before, err := vm.GetAccount(ctx, address, view, programs)
					require.NoError(t, err)
					assert.Len(t, before.Keys, keyCount)

					keyIndexArg, err := jsoncdc.Encode(cadence.NewInt(keyIndex))
					require.NoError(t, err)

					txBody := flow.NewTransactionBody().
						SetScript([]byte(test.source)).
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

		t.Run(fmt.Sprintf("Key added by a different api version %s", test.apiVersion),
			newVMTest().withContextOptions(options...).
				run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
					address := createAccount(t, vm, chain, ctx, view, programs)

					const keyCount = 2
					const keyIndex = keyCount - 1

					// Use one version of API to add the keys, and a different version of the API to revoke the keys.
					var apiVersionForAdding accountKeyAPIVersion
					if test.apiVersion == accountKeyAPIVersionV1 {
						apiVersionForAdding = accountKeyAPIVersionV2
					} else {
						apiVersionForAdding = accountKeyAPIVersionV1
					}

					for i := 0; i < keyCount; i++ {
						_ = addAccountKey(t, vm, ctx, view, programs, address, apiVersionForAdding)
					}

					before, err := vm.GetAccount(ctx, address, view, programs)
					require.NoError(t, err)
					assert.Len(t, before.Keys, keyCount)

					keyIndexArg, err := jsoncdc.Encode(cadence.NewInt(keyIndex))
					require.NoError(t, err)

					txBody := flow.NewTransactionBody().
						SetScript([]byte(test.source)).
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
	}

	// Remove multiple keys

	multipleKeysTests := []removeKeyTest{
		{
			source:     removeMultipleAccountKeysTransaction,
			apiVersion: accountKeyAPIVersionV1,
		},
		{
			source:     revokeMultipleAccountKeysTransaction,
			apiVersion: accountKeyAPIVersionV2,
		},
	}

	for _, test := range multipleKeysTests {
		t.Run(fmt.Sprintf("Multiple keys %s", test.apiVersion),
			newVMTest().withContextOptions(options...).
				run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
					address := createAccount(t, vm, chain, ctx, view, programs)

					const keyCount = 2

					for i := 0; i < keyCount; i++ {
						_ = addAccountKey(t, vm, ctx, view, programs, address, test.apiVersion)
					}

					before, err := vm.GetAccount(ctx, address, view, programs)
					require.NoError(t, err)
					assert.Len(t, before.Keys, keyCount)

					txBody := flow.NewTransactionBody().
						SetScript([]byte(test.source)).
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
}

func TestGetAccountKey(t *testing.T) {

	options := []fvm.Option{
		fvm.WithTransactionProcessors(fvm.NewTransactionInvoker(zerolog.Nop())),
		fvm.WithCadenceLogging(true),
	}

	t.Run("Non-existent key",
		newVMTest().withContextOptions(options...).
			run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				address := createAccount(t, vm, chain, ctx, view, programs)

				const keyCount = 2

				for i := 0; i < keyCount; i++ {
					_ = addAccountKey(t, vm, ctx, view, programs, address, accountKeyAPIVersionV2)
				}

				before, err := vm.GetAccount(ctx, address, view, programs)
				require.NoError(t, err)
				assert.Len(t, before.Keys, keyCount)

				for i, keyIndex := range []int{-1, keyCount, keyCount + 1} {
					keyIndexArg, err := jsoncdc.Encode(cadence.NewInt(keyIndex))
					require.NoError(t, err)

					txBody := flow.NewTransactionBody().
						SetScript([]byte(getAccountKeyTransaction)).
						AddArgument(keyIndexArg).
						AddAuthorizer(address)

					tx := fvm.Transaction(txBody, uint32(i))

					err = vm.Run(ctx, tx, view, programs)
					require.NoError(t, err)
					require.NoError(t, tx.Err)

					require.Len(t, tx.Logs, 1)
					assert.Equal(t, "nil", tx.Logs[0])
				}
			}),
	)

	t.Run("Existing key",
		newVMTest().withContextOptions(options...).
			run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				address := createAccount(t, vm, chain, ctx, view, programs)

				const keyCount = 2
				const keyIndex = keyCount - 1

				keys := make([]flow.AccountPublicKey, keyCount)
				for i := 0; i < keyCount; i++ {
					keys[i] = addAccountKey(t, vm, ctx, view, programs, address, accountKeyAPIVersionV2)
				}

				before, err := vm.GetAccount(ctx, address, view, programs)
				require.NoError(t, err)
				assert.Len(t, before.Keys, keyCount)

				keyIndexArg, err := jsoncdc.Encode(cadence.NewInt(keyIndex))
				require.NoError(t, err)

				txBody := flow.NewTransactionBody().
					SetScript([]byte(getAccountKeyTransaction)).
					AddArgument(keyIndexArg).
					AddAuthorizer(address)

				tx := fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)
				require.NoError(t, tx.Err)

				require.Len(t, tx.Logs, 1)

				key := keys[keyIndex]

				expected := fmt.Sprintf(
					"AccountKey("+
						"keyIndex: %d, "+
						"publicKey: PublicKey(publicKey: %s, signatureAlgorithm: SignatureAlgorithm(rawValue: 1)), "+
						"hashAlgorithm: HashAlgorithm(rawValue: 3), "+
						"weight: 1000.00000000, "+
						"isRevoked: false)",
					keyIndex,
					byteSliceToCadenceArrayLiteral(key.PublicKey.Encode()),
				)

				assert.Equal(t, expected, tx.Logs[0])
			}),
	)

	t.Run("Key added by a different api version",
		newVMTest().withContextOptions(options...).
			run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				address := createAccount(t, vm, chain, ctx, view, programs)

				const keyCount = 2
				const keyIndex = keyCount - 1

				keys := make([]flow.AccountPublicKey, keyCount)
				for i := 0; i < keyCount; i++ {

					// Use the old version of API to add the key
					keys[i] = addAccountKey(t, vm, ctx, view, programs, address, accountKeyAPIVersionV1)
				}

				before, err := vm.GetAccount(ctx, address, view, programs)
				require.NoError(t, err)
				assert.Len(t, before.Keys, keyCount)

				keyIndexArg, err := jsoncdc.Encode(cadence.NewInt(keyIndex))
				require.NoError(t, err)

				txBody := flow.NewTransactionBody().
					SetScript([]byte(getAccountKeyTransaction)).
					AddArgument(keyIndexArg).
					AddAuthorizer(address)

				tx := fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)
				require.NoError(t, tx.Err)

				require.Len(t, tx.Logs, 1)

				key := keys[keyIndex]

				expected := fmt.Sprintf(
					"AccountKey("+
						"keyIndex: %d, "+
						"publicKey: PublicKey(publicKey: %s, signatureAlgorithm: SignatureAlgorithm(rawValue: 1)), "+
						"hashAlgorithm: HashAlgorithm(rawValue: 3), "+
						"weight: 1000.00000000, "+
						"isRevoked: false)",
					keyIndex,
					byteSliceToCadenceArrayLiteral(key.PublicKey.Encode()),
				)

				assert.Equal(t, expected, tx.Logs[0])
			}),
	)

	t.Run("Multiple keys",
		newVMTest().withContextOptions(options...).
			run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				address := createAccount(t, vm, chain, ctx, view, programs)

				const keyCount = 2

				keys := make([]flow.AccountPublicKey, keyCount)
				for i := 0; i < keyCount; i++ {

					keys[i] = addAccountKey(t, vm, ctx, view, programs, address, accountKeyAPIVersionV2)
				}

				before, err := vm.GetAccount(ctx, address, view, programs)
				require.NoError(t, err)
				assert.Len(t, before.Keys, keyCount)

				txBody := flow.NewTransactionBody().
					SetScript([]byte(getMultipleAccountKeysTransaction)).
					AddAuthorizer(address)

				for i := 0; i < keyCount; i++ {
					keyIndexArg, err := jsoncdc.Encode(cadence.NewInt(i))
					require.NoError(t, err)

					txBody.AddArgument(keyIndexArg)
				}

				tx := fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)
				require.NoError(t, tx.Err)

				assert.Len(t, tx.Logs, 2)

				for i := 0; i < keyCount; i++ {
					expected := fmt.Sprintf(
						"AccountKey("+
							"keyIndex: %d, "+
							"publicKey: PublicKey(publicKey: %s, signatureAlgorithm: SignatureAlgorithm(rawValue: 1)), "+
							"hashAlgorithm: HashAlgorithm(rawValue: 3), "+
							"weight: 1000.00000000, "+
							"isRevoked: false)",
						i,
						byteSliceToCadenceArrayLiteral(keys[i].PublicKey.Encode()),
					)

					assert.Equal(t, expected, tx.Logs[i])
				}
			}),
	)
}

func byteSliceToCadenceArrayLiteral(bytes []byte) string {
	elements := make([]string, 0, len(bytes))

	for _, b := range bytes {
		elements = append(elements, strconv.Itoa(int(b)))
	}

	return format.Array(elements)
}

func TestAccountBalanceFields(t *testing.T) {
	t.Run("Get balance works",
		newVMTest().withContextOptions(
			fvm.WithTransactionProcessors(fvm.NewTransactionInvoker(zerolog.Nop())),
			fvm.WithCadenceLogging(true),
		).
			run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				account := createAccount(t, vm, chain, ctx, view, programs)

				txBody := transferTokensTx(chain).
					AddArgument(jsoncdc.MustEncode(cadence.UFix64(1_0000_0000))).
					AddArgument(jsoncdc.MustEncode(cadence.Address(account))).
					AddAuthorizer(chain.ServiceAddress())

				tx := fvm.Transaction(txBody, 0)

				err := vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				script := fvm.Script([]byte(fmt.Sprintf(`
					pub fun main(): UFix64 {
						let acc = getAccount(0x%s)
						return acc.balance
					}
				`, account.Hex())))

				err = vm.Run(ctx, script, view, programs)

				assert.NoError(t, err)

				assert.Equal(t, cadence.UFix64(1_0000_0000), script.Value)
			}),
	)

	t.Run("Get available balance works",
		newVMTest().withContextOptions(
			fvm.WithTransactionProcessors(fvm.NewTransactionInvoker(zerolog.Nop())),
			fvm.WithCadenceLogging(true),
			fvm.WithAccountStorageLimit(false),
		).withBootstrapProcedureOptions(
			fvm.WithStorageMBPerFLOW(10_0000_0000),
		).
			run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				account := createAccount(t, vm, chain, ctx, view, programs)

				txBody := transferTokensTx(chain).
					AddArgument(jsoncdc.MustEncode(cadence.UFix64(1_0000_0000))).
					AddArgument(jsoncdc.MustEncode(cadence.Address(account))).
					AddAuthorizer(chain.ServiceAddress())

				tx := fvm.Transaction(txBody, 0)

				err := vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				script := fvm.Script([]byte(fmt.Sprintf(`
					pub fun main(): UFix64 {
						let acc = getAccount(0x%s)
						return acc.availableBalance
					}
				`, account.Hex())))

				err = vm.Run(ctx, script, view, programs)

				assert.NoError(t, err)
				assert.NoError(t, script.Err)
				assert.Equal(t, cadence.UFix64(9999_2520), script.Value)
			}),
	)

	t.Run("Get available balance works with minimum balance",
		newVMTest().withContextOptions(
			fvm.WithTransactionProcessors(fvm.NewTransactionInvoker(zerolog.Nop())),
			fvm.WithCadenceLogging(true),
			fvm.WithAccountStorageLimit(false),
		).withBootstrapProcedureOptions(
			fvm.WithStorageMBPerFLOW(10_0000_0000),
			fvm.WithAccountCreationFee(10_0000),
			fvm.WithMinimumStorageReservation(10_0000),
		).
			run(func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				account := createAccount(t, vm, chain, ctx, view, programs)

				txBody := transferTokensTx(chain).
					AddArgument(jsoncdc.MustEncode(cadence.UFix64(1_0000_0000))).
					AddArgument(jsoncdc.MustEncode(cadence.Address(account))).
					AddAuthorizer(chain.ServiceAddress())

				tx := fvm.Transaction(txBody, 0)

				err := vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				script := fvm.Script([]byte(fmt.Sprintf(`
					pub fun main(): UFix64 {
						let acc = getAccount(0x%s)
						return acc.availableBalance
					}
				`, account.Hex())))

				err = vm.Run(ctx, script, view, programs)

				assert.NoError(t, err)
				assert.NoError(t, script.Err)

				// Should be 1_0000_0000 because 10_0000 was given to it during account creation and is now locked up
				assert.Equal(t, cadence.UFix64(1_0000_0000), script.Value)
			}),
	)
}
