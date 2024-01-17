package fvm_test

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/blueprints"
	envMock "github.com/onflow/flow-go/fvm/environment/mock"
	errors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func transferTokensTx(chain flow.Chain) *flow.TransactionBody {
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(
			`
							// This transaction is a template for a transaction that
							// could be used by anyone to send tokens to another account
							// that has been set up to receive tokens.
							//
							// The withdraw amount and the account from getAccount
							// would be the parameters to the transaction

							import FungibleToken from 0x%s
							import FlowToken from 0x%s

							transaction(amount: UFix64, to: Address) {

								// The Vault resource that holds the tokens that are being transferred
								let sentVault: @FungibleToken.Vault

								prepare(signer: AuthAccount) {

									// Get a reference to the signer's stored vault
									let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
										?? panic("Could not borrow reference to the owner's Vault!")

									// Withdraw tokens from the signer's stored vault
									self.sentVault <- vaultRef.withdraw(amount: amount)
								}

								execute {

									// Get the recipient's public account object
									let recipient = getAccount(to)

									// Get a reference to the recipient's Receiver
									let receiverRef = recipient.getCapability(/public/flowTokenReceiver)
										.borrow<&{FungibleToken.Receiver}>()
										?? panic("Could not borrow receiver reference to the recipient's Vault")

									// Deposit the withdrawn tokens in the recipient's receiver
									receiverRef.deposit(from: <-self.sentVault)
								}
							}`,
			sc.FungibleToken.Address.Hex(),
			sc.FlowToken.Address.Hex(),
		)),
		)
}

func filterAccountCreatedEvents(events []flow.Event) []flow.Event {
	var accountCreatedEvents []flow.Event
	for _, event := range events {
		if event.Type != flow.EventAccountCreated {
			continue
		}
		accountCreatedEvents = append(accountCreatedEvents, event)
		break
	}
	return accountCreatedEvents
}

func TestBlockContext_ExecuteTransaction(t *testing.T) {

	t.Parallel()

	chain, vm := createChainAndVm(flow.Testnet)

	ctx := fvm.NewContext(
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	t.Run("Success", func(t *testing.T) {
		txBody := flow.NewTransactionBody().
			SetScript([]byte(`
	            transaction {
	              prepare(signer: AuthAccount) {}
	            }
	        `)).
			AddAuthorizer(unittest.AddressFixture())

		err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		require.NoError(t, err)

		_, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			testutil.RootBootstrappedLedger(vm, ctx))
		require.NoError(t, err)
		require.NoError(t, output.Err)
	})

	t.Run("Failure", func(t *testing.T) {
		txBody := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                  var x: Int

                  prepare(signer: AuthAccount) {
                    self.x = 0
                  }

                  execute {
                    self.x = 1
                  }

                  post {
                    self.x == 2
                  }
                }
            `))

		err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		require.NoError(t, err)

		_, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			testutil.RootBootstrappedLedger(vm, ctx))
		require.NoError(t, err)
		require.Error(t, output.Err)
	})

	t.Run("Logs", func(t *testing.T) {
		txBody := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                  execute {
                    log("foo")
                    log("bar")
                  }
                }
            `))

		err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		require.NoError(t, err)

		_, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			testutil.RootBootstrappedLedger(vm, ctx))
		require.NoError(t, err)
		require.NoError(t, output.Err)

		require.Equal(t, []string{"\"foo\"", "\"bar\""}, output.Logs)
	})

	t.Run("Events", func(t *testing.T) {
		txBody := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                  prepare(signer: AuthAccount) {
                    AuthAccount(payer: signer)
                  }
                }
            `)).
			AddAuthorizer(chain.ServiceAddress())

		err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		require.NoError(t, err)

		_, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			testutil.RootBootstrappedLedger(vm, ctx))
		require.NoError(t, err)
		require.NoError(t, output.Err)

		require.Len(t, filterAccountCreatedEvents(output.Events), 1)
	})
}

func TestBlockContext_DeployContract(t *testing.T) {

	t.Parallel()

	chain, vm := createChainAndVm(flow.Mainnet)

	ctx := fvm.NewContext(
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	t.Run("account update with set code succeeds as service account", func(t *testing.T) {

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys
		// and the root account.
		snapshotTree, accounts, err := testutil.CreateAccounts(
			vm,
			testutil.RootBootstrappedLedger(vm, ctx),
			privateKeys,
			chain)
		require.NoError(t, err)

		txBody := testutil.DeployCounterContractTransaction(accounts[0], chain)

		txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
		txBody.SetPayer(chain.ServiceAddress())

		err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(
			txBody,
			chain.ServiceAddress(),
			unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		_, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)
	})

	t.Run("account with deployed contract has `contracts.names` filled", func(t *testing.T) {

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys
		// and the root account.
		snapshotTree, accounts, err := testutil.CreateAccounts(
			vm,
			testutil.RootBootstrappedLedger(vm, ctx),
			privateKeys,
			chain)
		require.NoError(t, err)

		txBody := testutil.DeployCounterContractTransaction(accounts[0], chain)

		txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
		txBody.SetPayer(chain.ServiceAddress())

		err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(
			txBody,
			chain.ServiceAddress(),
			unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		executionSnapshot, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		// transaction will panic if `contracts.names` is incorrect
		txBody = flow.NewTransactionBody().
			SetScript([]byte(`
				transaction {
					prepare(signer: AuthAccount) {
						var s : String = ""
						for name in signer.contracts.names {
							s = s.concat(name).concat(",")
						}
						if s != "Container," {
							panic(s)
						}
					}
				}
			`)).
			AddAuthorizer(accounts[0])

		txBody.SetProposalKey(chain.ServiceAddress(), 0, 1)
		txBody.SetPayer(chain.ServiceAddress())

		err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(
			txBody,
			chain.ServiceAddress(),
			unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		_, output, err = vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)
	})

	t.Run("account update with checker heavy contract (local replay limit)", func(t *testing.T) {
		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys
		// and the root account.
		snapshotTree, accounts, err := testutil.CreateAccounts(
			vm,
			testutil.RootBootstrappedLedger(vm, ctx),
			privateKeys,
			chain)
		require.NoError(t, err)

		txBody := testutil.DeployLocalReplayLimitedTransaction(
			accounts[0],
			chain)

		txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
		txBody.SetPayer(chain.ServiceAddress())

		err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(
			txBody,
			chain.ServiceAddress(),
			unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		_, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)

		var parsingCheckingError *runtime.ParsingCheckingError
		require.ErrorAs(t, output.Err, &parsingCheckingError)
		require.ErrorContains(
			t,
			output.Err,
			"program too ambiguous, local replay limit of 64 tokens exceeded")
	})

	t.Run("account update with checker heavy contract (global replay limit)", func(t *testing.T) {
		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys
		// and the root account.
		snapshotTree, accounts, err := testutil.CreateAccounts(
			vm,
			testutil.RootBootstrappedLedger(vm, ctx),
			privateKeys,
			chain)
		require.NoError(t, err)

		txBody := testutil.DeployGlobalReplayLimitedTransaction(
			accounts[0],
			chain)

		txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
		txBody.SetPayer(chain.ServiceAddress())

		err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(
			txBody,
			chain.ServiceAddress(),
			unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		_, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)

		var parsingCheckingError *runtime.ParsingCheckingError
		require.ErrorAs(t, output.Err, &parsingCheckingError)
		require.ErrorContains(
			t,
			output.Err,
			"program too ambiguous, global replay limit of 1024 tokens exceeded")
	})

	t.Run("account update with set code fails if not signed by service account", func(t *testing.T) {
		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys
		// and the root account.
		snapshotTree, accounts, err := testutil.CreateAccounts(
			vm,
			testutil.RootBootstrappedLedger(vm, ctx),
			privateKeys,
			chain)
		require.NoError(t, err)

		txBody := testutil.DeployUnauthorizedCounterContractTransaction(
			accounts[0])

		err = testutil.SignTransaction(txBody, accounts[0], privateKeys[0], 0)
		require.NoError(t, err)

		_, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)

		require.Error(t, output.Err)

		require.ErrorContains(
			t,
			output.Err,
			"deploying contracts requires authorization from specific accounts")
		require.True(t, errors.IsCadenceRuntimeError(output.Err))
	})

	t.Run("account update with set code fails if not signed by service account if dis-allowed in the state", func(t *testing.T) {
		ctx := fvm.NewContext(
			fvm.WithChain(chain),
			fvm.WithCadenceLogging(true),
			fvm.WithContractDeploymentRestricted(false),
		)
		restricted := true

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys
		// and the root account.
		snapshotTree, accounts, err := testutil.CreateAccounts(
			vm,
			testutil.RootBootstrappedLedger(
				vm,
				ctx,
				fvm.WithRestrictedContractDeployment(&restricted)),
			privateKeys,
			chain)
		require.NoError(t, err)

		txBody := testutil.DeployUnauthorizedCounterContractTransaction(
			accounts[0])
		txBody.SetProposalKey(accounts[0], 0, 0)
		txBody.SetPayer(accounts[0])

		err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		_, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),

			snapshotTree)
		require.NoError(t, err)
		require.Error(t, output.Err)

		require.ErrorContains(
			t,
			output.Err,
			"deploying contracts requires authorization from specific accounts")
		require.True(t, errors.IsCadenceRuntimeError(output.Err))
	})

	t.Run("account update with set succeeds if not signed by service account if allowed in the state", func(t *testing.T) {

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys
		// and the root account.
		restricted := false
		snapshotTree, accounts, err := testutil.CreateAccounts(
			vm,
			testutil.RootBootstrappedLedger(
				vm,
				ctx,
				fvm.WithRestrictedContractDeployment(&restricted)),
			privateKeys,
			chain)
		require.NoError(t, err)

		txBody := testutil.DeployUnauthorizedCounterContractTransaction(
			accounts[0])
		txBody.SetProposalKey(accounts[0], 0, 0)
		txBody.SetPayer(accounts[0])

		err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		_, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)
	})

	t.Run("account update with update code succeeds if not signed by service account", func(t *testing.T) {
		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys
		// and the root account.
		snapshotTree, accounts, err := testutil.CreateAccounts(
			vm,
			testutil.RootBootstrappedLedger(vm, ctx),
			privateKeys,
			chain)
		require.NoError(t, err)

		txBody := testutil.DeployCounterContractTransaction(accounts[0], chain)
		txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
		txBody.SetPayer(chain.ServiceAddress())

		err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(
			txBody,
			chain.ServiceAddress(),
			unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		executionSnapshot, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		txBody = testutil.UpdateUnauthorizedCounterContractTransaction(
			accounts[0])
		txBody.SetProposalKey(accounts[0], 0, 0)
		txBody.SetPayer(accounts[0])

		err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		_, output, err = vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)
	})

	t.Run("account update with code removal fails if not signed by service account", func(t *testing.T) {

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys
		// and the root account.
		snapshotTree, accounts, err := testutil.CreateAccounts(
			vm,
			testutil.RootBootstrappedLedger(vm, ctx),
			privateKeys,
			chain)
		require.NoError(t, err)

		txBody := testutil.DeployCounterContractTransaction(accounts[0], chain)
		txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
		txBody.SetPayer(chain.ServiceAddress())

		err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(
			txBody,
			chain.ServiceAddress(),
			unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		executionSnapshot, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		txBody = testutil.RemoveUnauthorizedCounterContractTransaction(
			accounts[0])
		txBody.SetProposalKey(accounts[0], 0, 0)
		txBody.SetPayer(accounts[0])

		err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		_, output, err = vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.Error(t, output.Err)

		require.ErrorContains(
			t,
			output.Err,
			"removing contracts requires authorization from specific accounts")
		require.True(t, errors.IsCadenceRuntimeError(output.Err))
	})

	t.Run("account update with code removal succeeds if signed by service account", func(t *testing.T) {
		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys
		// and the root account.
		snapshotTree, accounts, err := testutil.CreateAccounts(
			vm,
			testutil.RootBootstrappedLedger(vm, ctx),
			privateKeys,
			chain)
		require.NoError(t, err)

		txBody := testutil.DeployCounterContractTransaction(accounts[0], chain)
		txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
		txBody.SetPayer(chain.ServiceAddress())

		err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(
			txBody,
			chain.ServiceAddress(),
			unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		executionSnapshot, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		txBody = testutil.RemoveCounterContractTransaction(accounts[0], chain)
		txBody.SetProposalKey(accounts[0], 0, 0)
		txBody.SetPayer(chain.ServiceAddress())

		err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(
			txBody,
			chain.ServiceAddress(),
			unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		_, output, err = vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)
	})

	t.Run("account update with set code succeeds when account is added as authorized account", func(t *testing.T) {
		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys
		// and the root account.
		snapshotTree, accounts, err := testutil.CreateAccounts(
			vm,
			testutil.RootBootstrappedLedger(vm, ctx),
			privateKeys,
			chain)
		require.NoError(t, err)

		// setup a new authorizer account
		authTxBody, err := blueprints.SetContractDeploymentAuthorizersTransaction(
			chain.ServiceAddress(),
			[]flow.Address{chain.ServiceAddress(), accounts[0]})
		require.NoError(t, err)

		authTxBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
		authTxBody.SetPayer(chain.ServiceAddress())
		err = testutil.SignEnvelope(
			authTxBody,
			chain.ServiceAddress(),
			unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		executionSnapshot, output, err := vm.Run(
			ctx,
			fvm.Transaction(authTxBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		// test deploying a new contract (not authorized by service account)
		txBody := testutil.DeployUnauthorizedCounterContractTransaction(accounts[0])
		txBody.SetProposalKey(accounts[0], 0, 0)
		txBody.SetPayer(accounts[0])

		err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		_, output, err = vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)
	})

}

func TestBlockContext_ExecuteTransaction_WithArguments(t *testing.T) {

	t.Parallel()

	chain, vm := createChainAndVm(flow.Mainnet)

	ctx := fvm.NewContext(
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	arg1, _ := jsoncdc.Encode(cadence.NewInt(42))
	fooString, _ := cadence.NewString("foo")
	arg2, _ := jsoncdc.Encode(fooString)

	var tests = []struct {
		label       string
		script      string
		args        [][]byte
		authorizers []flow.Address
		check       func(t *testing.T, output fvm.ProcedureOutput)
	}{
		{
			label:  "No parameters",
			script: `transaction { execute { log("Hello, World!") } }`,
			args:   [][]byte{arg1},
			check: func(t *testing.T, output fvm.ProcedureOutput) {
				require.Error(t, output.Err)
			},
		},
		{
			label:  "Single parameter",
			script: `transaction(x: Int) { execute { log(x) } }`,
			args:   [][]byte{arg1},
			check: func(t *testing.T, output fvm.ProcedureOutput) {
				require.NoError(t, output.Err)
				require.Len(t, output.Logs, 1)
				require.Equal(t, "42", output.Logs[0])
			},
		},
		{
			label:  "Multiple parameters",
			script: `transaction(x: Int, y: String) { execute { log(x); log(y) } }`,
			args:   [][]byte{arg1, arg2},
			check: func(t *testing.T, output fvm.ProcedureOutput) {
				require.NoError(t, output.Err)
				require.Len(t, output.Logs, 2)
				require.Equal(t, "42", output.Logs[0])
				require.Equal(t, `"foo"`, output.Logs[1])
			},
		},
		{
			label: "Parameters and authorizer",
			script: `
                transaction(x: Int, y: String) {
                    prepare(acct: AuthAccount) { log(acct.address) }
                    execute { log(x); log(y) }
                }`,
			args:        [][]byte{arg1, arg2},
			authorizers: []flow.Address{chain.ServiceAddress()},
			check: func(t *testing.T, output fvm.ProcedureOutput) {
				require.NoError(t, output.Err)
				require.ElementsMatch(
					t,
					[]string{"0x" + chain.ServiceAddress().Hex(), "42", `"foo"`},
					output.Logs)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			txBody := flow.NewTransactionBody().
				SetScript([]byte(tt.script)).
				SetArguments(tt.args)

			for _, authorizer := range tt.authorizers {
				txBody.AddAuthorizer(authorizer)
			}

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			_, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				testutil.RootBootstrappedLedger(vm, ctx))
			require.NoError(t, err)

			tt.check(t, output)
		})
	}
}
func gasLimitScript(depth int) string {
	return fmt.Sprintf(`
        pub fun foo(_ i: Int) {
            if i <= 0 {
                return
            }
            log("foo")
            foo(i-1)
        }

        transaction { execute { foo(%d) } }
    `, depth)
}

func TestBlockContext_ExecuteTransaction_GasLimit(t *testing.T) {

	t.Parallel()

	chain, vm := createChainAndVm(flow.Mainnet)

	ctx := fvm.NewContext(
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	var tests = []struct {
		label    string
		script   string
		gasLimit uint64
		check    func(t *testing.T, output fvm.ProcedureOutput)
	}{
		{
			label:    "Zero",
			script:   gasLimitScript(100),
			gasLimit: 0,
			check: func(t *testing.T, output fvm.ProcedureOutput) {
				// gas limit of zero is ignored by runtime
				require.NoError(t, output.Err)
			},
		},
		{
			label:    "Insufficient",
			script:   gasLimitScript(100),
			gasLimit: 5,
			check: func(t *testing.T, output fvm.ProcedureOutput) {
				require.Error(t, output.Err)
			},
		},
		{
			label:    "Sufficient",
			script:   gasLimitScript(100),
			gasLimit: 1000,
			check: func(t *testing.T, output fvm.ProcedureOutput) {
				require.NoError(t, output.Err)
				require.Len(t, output.Logs, 100)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			txBody := flow.NewTransactionBody().
				SetScript([]byte(tt.script)).
				SetGasLimit(tt.gasLimit)

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			_, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				testutil.RootBootstrappedLedger(vm, ctx))
			require.NoError(t, err)

			tt.check(t, output)
		})
	}
}

func TestBlockContext_ExecuteTransaction_StorageLimit(t *testing.T) {

	t.Parallel()

	b := make([]byte, 1000000) // 1MB
	_, err := rand.Read(b)
	require.NoError(t, err)
	longString := base64.StdEncoding.EncodeToString(b) // 1.3 times 1MB

	script := fmt.Sprintf(`
			access(all) contract Container {
				access(all) resource Counter {
					pub var longString: String
					init() {
						self.longString = "%s"
					}
				}
			}`, longString)

	bootstrapOptions := []fvm.BootstrapProcedureOption{
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		// The evm account has a storage exception, and if we don't bootstrap with evm,
		// the first created account will have that address.
		fvm.WithSetupEVMEnabled(true),
	}

	t.Run("Storing too much data fails", newVMTest().withBootstrapProcedureOptions(bootstrapOptions...).
		run(
			func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
				// this test requires storage limits to be enforced
				ctx.LimitAccountStorage = true

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided
				// private keys and the root account.
				snapshotTree, accounts, err := testutil.CreateAccounts(
					vm,
					snapshotTree,
					privateKeys,
					chain)
				require.NoError(t, err)

				txBody := testutil.CreateContractDeploymentTransaction(
					"Container",
					script,
					accounts[0],
					chain)

				txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
				txBody.SetPayer(chain.ServiceAddress())

				err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
				require.NoError(t, err)

				err = testutil.SignEnvelope(
					txBody,
					chain.ServiceAddress(),
					unittest.ServiceAccountPrivateKey)
				require.NoError(t, err)

				_, output, err := vm.Run(
					ctx,
					fvm.Transaction(txBody, 0),
					snapshotTree)
				require.NoError(t, err)

				require.True(
					t,
					errors.IsStorageCapacityExceededError(output.Err))
			}))
	t.Run("Increasing storage capacity works", newVMTest().withBootstrapProcedureOptions(bootstrapOptions...).
		run(
			func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
				ctx.LimitAccountStorage = true // this test requires storage limits to be enforced

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided
				// private keys and the root account.
				snapshotTree, accounts, err := testutil.CreateAccounts(
					vm,
					snapshotTree,
					privateKeys,
					chain)
				require.NoError(t, err)

				sc := systemcontracts.SystemContractsForChain(chain.ChainID())
				// deposit more flow to increase capacity
				txBody := flow.NewTransactionBody().
					SetScript([]byte(fmt.Sprintf(
						`
					import FungibleToken from %s
					import FlowToken from %s

					transaction {
						prepare(signer: AuthAccount, service: AuthAccount) {
							signer.contracts.add(name: "%s", code: "%s".decodeHex())

							let vaultRef = service.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)!
							// deposit additional flow
							let payment <- vaultRef.withdraw(amount: 10.0) as! @FlowToken.Vault

							let receiver = signer.getCapability(/public/flowTokenReceiver)!.borrow<&{FungibleToken.Receiver}>()
								?? panic("Could not borrow receiver reference to the recipient's Vault")
							receiver.deposit(from: <-payment)
						}
					}`,
						sc.FungibleToken.Address.HexWithPrefix(),
						sc.FlowToken.Address.HexWithPrefix(),
						"Container",
						hex.EncodeToString([]byte(script)),
					))).
					AddAuthorizer(accounts[0]).
					AddAuthorizer(chain.ServiceAddress()).
					SetProposalKey(chain.ServiceAddress(), 0, 0).
					SetPayer(chain.ServiceAddress())

				err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
				require.NoError(t, err)

				err = testutil.SignEnvelope(
					txBody,
					chain.ServiceAddress(),
					unittest.ServiceAccountPrivateKey)
				require.NoError(t, err)

				_, output, err := vm.Run(
					ctx,
					fvm.Transaction(txBody, 0),
					snapshotTree)
				require.NoError(t, err)
				require.NoError(t, output.Err)
			}))
}

func TestBlockContext_ExecuteTransaction_InteractionLimitReached(t *testing.T) {
	t.Parallel()

	b := make([]byte, 1000000) // 1MB
	_, err := rand.Read(b)
	require.NoError(t, err)
	longString := base64.StdEncoding.EncodeToString(b) // ~1.3 times 1MB

	// save a really large contract to an account should fail because of interaction limit reached
	script := fmt.Sprintf(`
			access(all) contract Container {
				access(all) resource Counter {
					pub var longString: String
					init() {
						self.longString = "%s"
					}
				}
			}`, longString)

	bootstrapOptions := []fvm.BootstrapProcedureOption{
		fvm.WithTransactionFee(fvm.DefaultTransactionFees),
	}

	t.Run("Using to much interaction fails", newVMTest().withBootstrapProcedureOptions(bootstrapOptions...).
		withContextOptions(fvm.WithTransactionFeesEnabled(true)).
		run(
			func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
				ctx.MaxStateInteractionSize = 500_000

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided
				// private keys and the root account.
				snapshotTree, accounts, err := testutil.CreateAccounts(
					vm,
					snapshotTree,
					privateKeys,
					chain)
				require.NoError(t, err)

				n := 0
				seqNum := func() uint64 {
					sn := n
					n++
					return uint64(sn)
				}
				// fund account so the payer can pay for the next transaction.
				txBody := transferTokensTx(chain).
					SetProposalKey(chain.ServiceAddress(), 0, seqNum()).
					AddAuthorizer(chain.ServiceAddress()).
					AddArgument(
						jsoncdc.MustEncode(cadence.UFix64(100_000_000))).
					AddArgument(
						jsoncdc.MustEncode(cadence.NewAddress(accounts[0]))).
					SetPayer(chain.ServiceAddress())

				err = testutil.SignEnvelope(
					txBody,
					chain.ServiceAddress(),
					unittest.ServiceAccountPrivateKey)
				require.NoError(t, err)

				executionSnapshot, output, err := vm.Run(
					ctx,
					fvm.Transaction(txBody, 0),
					snapshotTree)
				require.NoError(t, err)
				require.NoError(t, output.Err)

				snapshotTree = snapshotTree.Append(executionSnapshot)

				ctx.MaxStateInteractionSize = 500_000

				txBody = testutil.CreateContractDeploymentTransaction(
					"Container",
					script,
					accounts[0],
					chain)

				txBody.SetProposalKey(chain.ServiceAddress(), 0, seqNum())
				txBody.SetPayer(accounts[0])

				err = testutil.SignPayload(
					txBody,
					chain.ServiceAddress(),
					unittest.ServiceAccountPrivateKey)
				require.NoError(t, err)

				err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
				require.NoError(t, err)

				_, output, err = vm.Run(
					ctx,
					fvm.Transaction(txBody, 0),
					snapshotTree)
				require.NoError(t, err)

				require.True(
					t,
					errors.IsLedgerInteractionLimitExceededError(output.Err))
			}))

	t.Run("Using to much interaction but not failing because of service account", newVMTest().withBootstrapProcedureOptions(bootstrapOptions...).
		withContextOptions(fvm.WithTransactionFeesEnabled(true)).
		run(
			func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
				ctx.MaxStateInteractionSize = 500_000

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided
				// private keys and the root account.
				snapshotTree, accounts, err := testutil.CreateAccounts(
					vm,
					snapshotTree,
					privateKeys,
					chain)
				require.NoError(t, err)

				txBody := testutil.CreateContractDeploymentTransaction(
					"Container",
					script,
					accounts[0],
					chain)

				txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
				txBody.SetPayer(chain.ServiceAddress())

				err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
				require.NoError(t, err)

				err = testutil.SignEnvelope(
					txBody,
					chain.ServiceAddress(),
					unittest.ServiceAccountPrivateKey)
				require.NoError(t, err)

				_, output, err := vm.Run(
					ctx,
					fvm.Transaction(txBody, 0),
					snapshotTree)
				require.NoError(t, err)
				require.NoError(t, output.Err)
			}))

	t.Run("Using to much interaction fails but does not panic", newVMTest().withBootstrapProcedureOptions(bootstrapOptions...).
		withContextOptions(
			fvm.WithAuthorizationChecksEnabled(false),
			fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
		).
		run(
			func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
				ctx.MaxStateInteractionSize = 50_000

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided
				// private keys and the root account.
				snapshotTree, accounts, err := testutil.CreateAccounts(
					vm,
					snapshotTree,
					privateKeys,
					chain)
				require.NoError(t, err)

				_, txBody := testutil.CreateMultiAccountCreationTransaction(
					t,
					chain,
					40)

				txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
				txBody.SetPayer(accounts[0])

				err = testutil.SignPayload(
					txBody,
					chain.ServiceAddress(),
					unittest.ServiceAccountPrivateKey)
				require.NoError(t, err)

				err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
				require.NoError(t, err)

				_, output, err := vm.Run(
					ctx,
					fvm.Transaction(txBody, 0),
					snapshotTree)
				require.NoError(t, err)
				require.Error(t, output.Err)

				require.True(t, errors.IsCadenceRuntimeError(output.Err))
			}))
}

var createAccountScript = []byte(`
    transaction {
        prepare(signer: AuthAccount) {
            let acct = AuthAccount(payer: signer)
        }
    }
`)

func TestBlockContext_ExecuteScript(t *testing.T) {

	t.Parallel()

	chain, vm := createChainAndVm(flow.Mainnet)

	ctx := fvm.NewContext(
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	t.Run("script success", func(t *testing.T) {
		code := []byte(`
            pub fun main(): Int {
                return 42
            }
        `)

		_, output, err := vm.Run(
			ctx,
			fvm.Script(code),
			testutil.RootBootstrappedLedger(vm, ctx))
		require.NoError(t, err)

		require.NoError(t, output.Err)
	})

	t.Run("script failure", func(t *testing.T) {
		code := []byte(`
            pub fun main(): Int {
                assert(1 == 2)
                return 42
            }
        `)

		_, output, err := vm.Run(
			ctx,
			fvm.Script(code),
			testutil.RootBootstrappedLedger(vm, ctx))
		require.NoError(t, err)

		require.Error(t, output.Err)
	})

	t.Run("script logs", func(t *testing.T) {
		code := []byte(`
            pub fun main(): Int {
                log("foo")
                log("bar")
                return 42
            }
        `)

		_, output, err := vm.Run(
			ctx,
			fvm.Script(code),
			testutil.RootBootstrappedLedger(vm, ctx))
		require.NoError(t, err)

		require.NoError(t, output.Err)
		require.Len(t, output.Logs, 2)
		require.Equal(t, "\"foo\"", output.Logs[0])
		require.Equal(t, "\"bar\"", output.Logs[1])
	})

	t.Run("storage ID allocation", func(t *testing.T) {
		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private
		// keys and the root account.
		snapshotTree, accounts, err := testutil.CreateAccounts(
			vm,
			testutil.RootBootstrappedLedger(vm, ctx),
			privateKeys,
			chain)
		require.NoError(t, err)

		// Deploy the test contract

		const contract = `
			pub contract Test {

				pub struct Foo {}

                pub let foos: [Foo]

				init() {
					self.foos = []
				}

				pub fun add() {
					self.foos.append(Foo())
				}
			}
		`

		address := accounts[0]

		txBody := testutil.CreateContractDeploymentTransaction(
			"Test",
			contract,
			address,
			chain)

		txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
		txBody.SetPayer(chain.ServiceAddress())

		err = testutil.SignPayload(txBody, address, privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(
			txBody,
			chain.ServiceAddress(),
			unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		executionSnapshot, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		// Run test script

		code := []byte(fmt.Sprintf(
			`
			  import Test from 0x%s

			  pub fun main() {
			      Test.add()
			  }
			`,
			address.String(),
		))

		_, output, err = vm.Run(ctx, fvm.Script(code), snapshotTree)
		require.NoError(t, err)

		require.NoError(t, output.Err)
	})
}

func TestBlockContext_GetBlockInfo(t *testing.T) {

	t.Parallel()

	chain, vm := createChainAndVm(flow.Mainnet)

	ctx := fvm.NewContext(
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	blocks := new(envMock.Blocks)

	block1 := unittest.BlockFixture()
	block2 := unittest.BlockWithParentFixture(block1.Header)
	block3 := unittest.BlockWithParentFixture(block2.Header)

	blocks.On("ByHeightFrom", block1.Header.Height, block1.Header).Return(block1.Header, nil)
	blocks.On("ByHeightFrom", block2.Header.Height, block1.Header).Return(block2.Header, nil)

	type logPanic struct{}
	blocks.On("ByHeightFrom", block3.Header.Height, block1.Header).Run(func(args mock.Arguments) { panic(logPanic{}) })

	blockCtx := fvm.NewContextFromParent(ctx, fvm.WithBlocks(blocks), fvm.WithBlockHeader(block1.Header))

	t.Run("works as transaction", func(t *testing.T) {
		txBody := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                    execute {
                        let block = getCurrentBlock()
                        log(block)

                        let nextBlock = getBlock(at: block.height + UInt64(1))
                        log(nextBlock)
                    }
                }
            `))

		err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		require.NoError(t, err)

		_, output, err := vm.Run(
			blockCtx,
			fvm.Transaction(txBody, 0),
			testutil.RootBootstrappedLedger(vm, ctx))
		require.NoError(t, err)
		require.NoError(t, output.Err)

		require.Len(t, output.Logs, 2)
		require.Equal(
			t,
			fmt.Sprintf(
				"Block(height: %v, view: %v, id: 0x%x, timestamp: %.8f)",
				block1.Header.Height,
				block1.Header.View,
				block1.ID(),
				float64(block1.Header.Timestamp.Unix()),
			),
			output.Logs[0],
		)
		require.Equal(
			t,
			fmt.Sprintf(
				"Block(height: %v, view: %v, id: 0x%x, timestamp: %.8f)",
				block2.Header.Height,
				block2.Header.View,
				block2.ID(),
				float64(block2.Header.Timestamp.Unix()),
			),
			output.Logs[1],
		)
	})

	t.Run("works as script", func(t *testing.T) {
		code := []byte(`
            pub fun main() {
                let block = getCurrentBlock()
                log(block)

                let nextBlock = getBlock(at: block.height + UInt64(1))
                log(nextBlock)
            }
        `)

		_, output, err := vm.Run(
			blockCtx,
			fvm.Script(code),
			testutil.RootBootstrappedLedger(vm, ctx))
		require.NoError(t, err)
		require.NoError(t, output.Err)

		require.Len(t, output.Logs, 2)
		require.Equal(t,
			fmt.Sprintf(
				"Block(height: %v, view: %v, id: 0x%x, timestamp: %.8f)",
				block1.Header.Height,
				block1.Header.View,
				block1.ID(),
				float64(block1.Header.Timestamp.Unix()),
			),
			output.Logs[0],
		)
		require.Equal(
			t,
			fmt.Sprintf(
				"Block(height: %v, view: %v, id: 0x%x, timestamp: %.8f)",
				block2.Header.Height,
				block2.Header.View,
				block2.ID(),
				float64(block2.Header.Timestamp.Unix()),
			),
			output.Logs[1],
		)
	})

	t.Run("panics if external function panics in transaction", func(t *testing.T) {
		tx := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                    execute {
                        let block = getCurrentBlock()
                        let nextBlock = getBlock(at: block.height + UInt64(2))
                    }
                }
            `))

		err := testutil.SignTransactionAsServiceAccount(tx, 0, chain)
		require.NoError(t, err)

		_, output, err := vm.Run(
			blockCtx,
			fvm.Transaction(tx, 0),
			testutil.RootBootstrappedLedger(vm, ctx))
		require.NoError(t, err)
		require.Error(t, output.Err)
	})

	t.Run("panics if external function panics in script", func(t *testing.T) {
		script := []byte(`
            pub fun main() {
                let block = getCurrentBlock()
                let nextBlock = getBlock(at: block.height + UInt64(2))
            }
        `)

		_, output, err := vm.Run(
			blockCtx,
			fvm.Script(script),
			testutil.RootBootstrappedLedger(vm, ctx))
		require.NoError(t, err)
		require.Error(t, output.Err)
	})
}

func TestBlockContext_GetAccount(t *testing.T) {

	t.Parallel()

	const count = 100

	chain, vm := createChainAndVm(flow.Mainnet)

	ctx := fvm.NewContext(
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	snapshotTree := testutil.RootBootstrappedLedger(vm, ctx)
	sequenceNumber := uint64(0)

	createAccount := func() (flow.Address, crypto.PublicKey) {
		privateKey, txBody := testutil.CreateAccountCreationTransaction(
			t,
			chain)

		txBody.SetProposalKey(chain.ServiceAddress(), 0, sequenceNumber)
		txBody.SetPayer(chain.ServiceAddress())
		sequenceNumber++

		rootHasher := hash.NewSHA2_256()

		err := txBody.SignEnvelope(
			chain.ServiceAddress(),
			0,
			unittest.ServiceAccountPrivateKey.PrivateKey,
			rootHasher,
		)
		require.NoError(t, err)

		// execute the transaction
		executionSnapshot, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			snapshotTree)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		snapshotTree = snapshotTree.Append(executionSnapshot)

		accountCreatedEvents := filterAccountCreatedEvents(output.Events)

		require.Len(t, accountCreatedEvents, 1)

		// read the address of the account created (e.g. "0x01" and convert it
		// to flow.address)
		data, err := ccf.Decode(nil, accountCreatedEvents[0].Payload)
		require.NoError(t, err)
		address := flow.ConvertAddress(
			data.(cadence.Event).Fields[0].(cadence.Address))

		return address, privateKey.PublicKey(
			fvm.AccountKeyWeightThreshold).PublicKey
	}

	addressGen := chain.NewAddressGenerator()
	// skip the addresses of 4 reserved accounts
	for i := 0; i < 4; i++ {
		_, err := addressGen.NextAddress()
		require.NoError(t, err)
	}

	// create a bunch of accounts
	accounts := make(map[flow.Address]crypto.PublicKey, count)
	for i := 0; i < count; i++ {
		address, key := createAccount()
		expectedAddress, err := addressGen.NextAddress()
		require.NoError(t, err)

		require.Equal(t, expectedAddress, address)
		accounts[address] = key
	}

	// happy path - get each of the created account and check if it is the right one
	t.Run("get accounts", func(t *testing.T) {
		for address, expectedKey := range accounts {
			account, err := vm.GetAccount(ctx, address, snapshotTree)
			require.NoError(t, err)

			require.Len(t, account.Keys, 1)
			actualKey := account.Keys[0].PublicKey
			require.Equal(t, expectedKey, actualKey)
		}
	})

	// non-happy path - get an account that was never created
	t.Run("get a non-existing account", func(t *testing.T) {
		address, err := addressGen.NextAddress()
		require.NoError(t, err)

		account, err := vm.GetAccount(ctx, address, snapshotTree)
		require.True(t, errors.IsAccountNotFoundError(err))
		require.Nil(t, account)
	})
}

func TestBlockContext_Random(t *testing.T) {
	chain, vm := createChainAndVm(flow.Mainnet)
	header := &flow.Header{Height: 42}
	source := testutil.EntropyProviderFixture(nil)
	ctx := fvm.NewContext(
		fvm.WithChain(chain),
		fvm.WithBlockHeader(header),
		fvm.WithEntropyProvider(source),
		fvm.WithCadenceLogging(true),
	)

	tx_code := []byte(`
	transaction {
		execute {
			let rand1 = unsafeRandom()
			log(rand1)
			let rand2 = unsafeRandom()
			log(rand2)
		}
	}
	`)

	getTxRandoms := func(t *testing.T) [2]uint64 {
		txBody := flow.NewTransactionBody().SetScript(tx_code)
		err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		require.NoError(t, err)

		_, output, err := vm.Run(
			ctx,
			fvm.Transaction(txBody, 0),
			testutil.RootBootstrappedLedger(vm, ctx))
		require.NoError(t, err)
		require.NoError(t, output.Err)
		require.Len(t, output.Logs, 2)

		r1, err := strconv.ParseUint(output.Logs[0], 10, 64)
		require.NoError(t, err)
		r2, err := strconv.ParseUint(output.Logs[1], 10, 64)
		require.NoError(t, err)
		return [2]uint64{r1, r2}
	}

	// - checks that unsafeRandom works on transactions
	// - (sanity) checks that two successive randoms aren't equal
	t.Run("single transaction", func(t *testing.T) {
		randoms := getTxRandoms(t)
		require.NotEqual(t, randoms[1], randoms[0], "extremely unlikely to be equal")
	})

	// checks that two transactions with different IDs do not generate the same randoms
	t.Run("two transactions", func(t *testing.T) {
		// getLoggedRandoms generates different tx IDs because envelope signature is randomized
		randoms1 := getTxRandoms(t)
		randoms2 := getTxRandoms(t)
		require.NotEqual(t, randoms1[0], randoms2[0], "extremely unlikely to be equal")
	})

	script_string := `
	pub fun main(a: Int8) {
		let rand = unsafeRandom()
		log(rand)
		let rand%d = unsafeRandom()
		log(rand%d)
	}
	`

	getScriptRandoms := func(t *testing.T, codeSalt int, arg int) [2]uint64 {
		script_code := []byte(fmt.Sprintf(script_string, codeSalt, codeSalt))
		script := fvm.Script(script_code).WithArguments(
			jsoncdc.MustEncode(cadence.Int8(arg)))

		_, output, err := vm.Run(ctx, script, testutil.RootBootstrappedLedger(vm, ctx))
		require.NoError(t, err)
		require.NoError(t, output.Err)

		r1, err := strconv.ParseUint(output.Logs[0], 10, 64)
		require.NoError(t, err)
		r2, err := strconv.ParseUint(output.Logs[1], 10, 64)
		require.NoError(t, err)
		return [2]uint64{r1, r2}
	}

	// - checks that unsafeRandom works on scripts
	// - (sanity) checks that two successive randoms aren't equal
	t.Run("single script", func(t *testing.T) {
		randoms := getScriptRandoms(t, 1, 0)
		require.NotEqual(t, randoms[1], randoms[0], "extremely unlikely to be equal")
	})

	// checks that two scripts with different codes do not generate the same randoms
	t.Run("two script codes", func(t *testing.T) {
		// getScriptRandoms generates different scripts IDs using different codes
		randoms1 := getScriptRandoms(t, 1, 0)
		randoms2 := getScriptRandoms(t, 2, 0)
		require.NotEqual(t, randoms1[0], randoms2[0], "extremely unlikely to be equal")
	})

	// checks that two scripts with same codes but different arguments do not generate the same randoms
	t.Run("same script codes different arguments", func(t *testing.T) {
		// getScriptRandoms generates different scripts IDs using different arguments
		randoms1 := getScriptRandoms(t, 1, 0)
		randoms2 := getScriptRandoms(t, 1, 1)
		require.NotEqual(t, randoms1[0], randoms2[0], "extremely unlikely to be equal")
	})

}

func TestBlockContext_ExecuteTransaction_CreateAccount_WithMonotonicAddresses(t *testing.T) {

	t.Parallel()

	chain, vm := createChainAndVm(flow.MonotonicEmulator)

	ctx := fvm.NewContext(
		fvm.WithChain(chain),
	)

	txBody := flow.NewTransactionBody().
		SetScript(createAccountScript).
		AddAuthorizer(chain.ServiceAddress())

	err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
	require.NoError(t, err)

	_, output, err := vm.Run(
		ctx,
		fvm.Transaction(txBody, 0),
		testutil.RootBootstrappedLedger(vm, ctx))
	require.NoError(t, err)
	require.NoError(t, output.Err)

	accountCreatedEvents := filterAccountCreatedEvents(output.Events)

	require.Len(t, accountCreatedEvents, 1)

	data, err := ccf.Decode(nil, accountCreatedEvents[0].Payload)
	require.NoError(t, err)
	address := flow.ConvertAddress(
		data.(cadence.Event).Fields[0].(cadence.Address))

	require.Equal(t, flow.HexToAddress("05"), address)
}

func TestBlockContext_ExecuteTransaction_FailingTransactions(t *testing.T) {
	getBalance := func(
		vm fvm.VM,
		chain flow.Chain,
		ctx fvm.Context,
		storageSnapshot snapshot.StorageSnapshot,
		address flow.Address,
	) uint64 {

		sc := systemcontracts.SystemContractsForChain(chain.ChainID())
		code := []byte(fmt.Sprintf(
			`
					import FungibleToken from 0x%s
					import FlowToken from 0x%s

					pub fun main(account: Address): UFix64 {
						let acct = getAccount(account)
						let vaultRef = acct.getCapability(/public/flowTokenBalance)
							.borrow<&FlowToken.Vault{FungibleToken.Balance}>()
							?? panic("Could not borrow Balance reference to the Vault")

						return vaultRef.balance
					}
				`,
			sc.FungibleToken.Address.Hex(),
			sc.FlowToken.Address.Hex(),
		))
		script := fvm.Script(code).WithArguments(
			jsoncdc.MustEncode(cadence.NewAddress(address)),
		)

		_, output, err := vm.Run(ctx, script, storageSnapshot)
		require.NoError(t, err)
		require.NoError(t, output.Err)
		return output.Value.ToGoValue().(uint64)
	}

	t.Run("Transaction fails because of storage", newVMTest().withBootstrapProcedureOptions(
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		fvm.WithExecutionMemoryLimit(math.MaxUint64),
		// The evm account has a storage exception, and if we don't bootstrap with evm,
		// the first created account will have that address.
		fvm.WithSetupEVMEnabled(true),
	).run(
		func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
			ctx.LimitAccountStorage = true // this test requires storage limits to be enforced

			// Create an account private key.
			privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
			require.NoError(t, err)

			// Bootstrap a ledger, creating accounts with the provided private
			// keys and the root account.
			snapshotTree, accounts, err := testutil.CreateAccounts(
				vm,
				snapshotTree,
				privateKeys,
				chain)
			require.NoError(t, err)

			balanceBefore := getBalance(
				vm,
				chain,
				ctx,
				snapshotTree,
				accounts[0])

			txBody := transferTokensTx(chain).
				AddAuthorizer(accounts[0]).
				AddArgument(jsoncdc.MustEncode(cadence.UFix64(1))).
				AddArgument(jsoncdc.MustEncode(
					cadence.NewAddress(chain.ServiceAddress())))

			txBody.SetProposalKey(accounts[0], 0, 0)
			txBody.SetPayer(accounts[0])

			err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
			require.NoError(t, err)

			executionSnapshot, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)
			require.NoError(t, err)

			snapshotTree = snapshotTree.Append(executionSnapshot)

			require.True(t, errors.IsStorageCapacityExceededError(output.Err))

			balanceAfter := getBalance(
				vm,
				chain,
				ctx,
				snapshotTree.Append(executionSnapshot),
				accounts[0])

			require.Equal(t, balanceAfter, balanceBefore)
		}),
	)

	t.Run("Transaction fails because of recipient account not existing", newVMTest().withBootstrapProcedureOptions(
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		fvm.WithExecutionMemoryLimit(math.MaxUint64),
	).run(
		func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
			ctx.LimitAccountStorage = true // this test requires storage limits to be enforced

			// Create an account private key.
			privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
			require.NoError(t, err)

			// Bootstrap a ledger, creating accounts with the provided private
			// keys and the root account.
			snapshotTree, accounts, err := testutil.CreateAccounts(
				vm,
				snapshotTree,
				privateKeys,
				chain)
			require.NoError(t, err)

			// non-existent account
			lastAddress, err := chain.AddressAtIndex((1 << 45) - 1)
			require.NoError(t, err)

			balanceBefore := getBalance(
				vm,
				chain,
				ctx,
				snapshotTree,
				accounts[0])

			// transfer tokens to non-existent account
			txBody := transferTokensTx(chain).
				AddAuthorizer(accounts[0]).
				AddArgument(jsoncdc.MustEncode(cadence.UFix64(1))).
				AddArgument(jsoncdc.MustEncode(cadence.NewAddress(lastAddress)))

			txBody.SetProposalKey(accounts[0], 0, 0)
			txBody.SetPayer(accounts[0])

			err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
			require.NoError(t, err)

			executionSnapshot, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				snapshotTree)
			require.NoError(t, err)

			snapshotTree = snapshotTree.Append(executionSnapshot)

			require.True(t, errors.IsCadenceRuntimeError(output.Err))

			balanceAfter := getBalance(
				vm,
				chain,
				ctx,
				snapshotTree.Append(executionSnapshot),
				accounts[0])

			require.Equal(t, balanceAfter, balanceBefore)
		}),
	)

	t.Run("Transaction sequence number check fails and sequence number is not incremented", newVMTest().withBootstrapProcedureOptions(
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
	).
		run(
			func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
				// this test requires storage limits to be enforced
				ctx.LimitAccountStorage = true

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided
				// private keys and the root account.
				snapshotTree, accounts, err := testutil.CreateAccounts(
					vm,
					snapshotTree,
					privateKeys,
					chain)
				require.NoError(t, err)

				txBody := transferTokensTx(chain).
					AddAuthorizer(accounts[0]).
					AddArgument(jsoncdc.MustEncode(cadence.UFix64(1_0000_0000_0000))).
					AddArgument(jsoncdc.MustEncode(
						cadence.NewAddress(chain.ServiceAddress())))

				// set wrong sequence number
				txBody.SetProposalKey(accounts[0], 0, 10)
				txBody.SetPayer(accounts[0])

				err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
				require.NoError(t, err)

				_, output, err := vm.Run(
					ctx,
					fvm.Transaction(txBody, 0),
					snapshotTree)
				require.NoError(t, err)

				require.Equal(
					t,
					errors.ErrCodeInvalidProposalSeqNumberError,
					output.Err.Code())

				// The outer most coded error is a wrapper, not the actual
				// InvalidProposalSeqNumberError itself.
				_, ok := output.Err.(errors.InvalidProposalSeqNumberError)
				require.False(t, ok)

				var seqNumErr errors.InvalidProposalSeqNumberError
				ok = errors.As(output.Err, &seqNumErr)
				require.True(t, ok)
				require.Equal(t, uint64(0), seqNumErr.CurrentSeqNumber())
			}),
	)

	t.Run("Transaction invocation fails but sequence number is incremented", newVMTest().withBootstrapProcedureOptions(
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
	).
		run(
			func(t *testing.T, vm fvm.VM, chain flow.Chain, ctx fvm.Context, snapshotTree snapshot.SnapshotTree) {
				// this test requires storage limits to be enforced
				ctx.LimitAccountStorage = true

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided
				// private keys and the root account.
				snapshotTree, accounts, err := testutil.CreateAccounts(
					vm,
					snapshotTree,
					privateKeys,
					chain)
				require.NoError(t, err)

				txBody := transferTokensTx(chain).
					AddAuthorizer(accounts[0]).
					AddArgument(jsoncdc.MustEncode(
						cadence.UFix64(1_0000_0000_0000))).
					AddArgument(jsoncdc.MustEncode(
						cadence.NewAddress(chain.ServiceAddress())))

				txBody.SetProposalKey(accounts[0], 0, 0)
				txBody.SetPayer(accounts[0])

				err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
				require.NoError(t, err)

				executionSnapshot, output, err := vm.Run(
					ctx,
					fvm.Transaction(txBody, 0),
					snapshotTree)
				require.NoError(t, err)

				snapshotTree = snapshotTree.Append(executionSnapshot)

				require.True(t, errors.IsCadenceRuntimeError(output.Err))

				// send it again
				_, output, err = vm.Run(
					ctx,
					fvm.Transaction(txBody, 0),
					snapshotTree)
				require.NoError(t, err)

				require.Equal(
					t,
					errors.ErrCodeInvalidProposalSeqNumberError,
					output.Err.Code())

				// The outer most coded error is a wrapper, not the actual
				// InvalidProposalSeqNumberError itself.
				_, ok := output.Err.(errors.InvalidProposalSeqNumberError)
				require.False(t, ok)

				var seqNumErr errors.InvalidProposalSeqNumberError
				ok = errors.As(output.Err, &seqNumErr)
				require.True(t, ok)
				require.Equal(t, uint64(1), seqNumErr.CurrentSeqNumber())
			}),
	)
}
