package fvm_test

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/fvm/blueprints"
	fvmmock "github.com/onflow/flow-go/fvm/mock"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"

	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	errors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func transferTokensTx(chain flow.Chain) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(`
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
							}`, fvm.FungibleTokenAddress(chain), fvm.FlowTokenAddress(chain))),
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

const auditContractForDeploymentTransactionTemplate = `
import FlowContractAudits from 0x%s

transaction(deployAddress: Address, code: String) {
	prepare(serviceAccount: AuthAccount) {

		let auditorAdmin = serviceAccount.borrow<&FlowContractAudits.Administrator>(from: FlowContractAudits.AdminStoragePath)
            ?? panic("Could not borrow a reference to the admin resource")

		let auditor <- auditorAdmin.createNewAuditor()

		auditor.addVoucher(address: deployAddress, recurrent: false, expiryOffset: nil, code: code)

		destroy auditor
	}
}
`

// AuditContractForDeploymentTransaction returns a transaction for generating an audit voucher for contract deploy/update
func AuditContractForDeploymentTransaction(serviceAccount flow.Address, deployAddress flow.Address, code string) (*flow.TransactionBody, error) {
	arg1, err := jsoncdc.Encode(cadence.NewAddress(deployAddress))
	if err != nil {
		return nil, err
	}

	codeCdc, err := cadence.NewString(code)
	if err != nil {
		return nil, err
	}
	arg2, err := jsoncdc.Encode(codeCdc)
	if err != nil {
		return nil, err
	}

	tx := fmt.Sprintf(
		auditContractForDeploymentTransactionTemplate,
		serviceAccount.String(),
	)

	return flow.NewTransactionBody().
		SetScript([]byte(tx)).
		AddAuthorizer(serviceAccount).
		AddArgument(arg1).
		AddArgument(arg2), nil
}

func TestBlockContext_ExecuteTransaction(t *testing.T) {

	t.Parallel()

	chain, vm := createChainAndVm(flow.Testnet)

	ctx := fvm.NewContext(
		zerolog.Nop(),
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

		view := testutil.RootBootstrappedLedger(vm, ctx)
		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, view, programs.NewEmptyPrograms())
		require.NoError(t, err)

		assert.Nil(t, tx.Err)
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

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)

		assert.Error(t, tx.Err)
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

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)

		require.Len(t, tx.Logs, 2)
		assert.Equal(t, "\"foo\"", tx.Logs[0])
		assert.Equal(t, "\"bar\"", tx.Logs[1])
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

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)

		assert.NoError(t, tx.Err)

		accountCreatedEvents := filterAccountCreatedEvents(tx.Events)

		require.Len(t, accountCreatedEvents, 1)
	})
}

func TestBlockContext_DeployContract(t *testing.T) {

	t.Parallel()

	chain, vm := createChainAndVm(flow.Mainnet)

	ctx := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	t.Run("account update with set code succeeds as service account", func(t *testing.T) {
		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := testutil.CreateAccounts(vm, ledger, programs.NewEmptyPrograms(), privateKeys, chain)
		require.NoError(t, err)

		txBody := testutil.DeployCounterContractTransaction(accounts[0], chain)

		txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
		txBody.SetPayer(chain.ServiceAddress())

		err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)

		assert.NoError(t, tx.Err)
	})

	t.Run("account with deployed contract has `contracts.names` filled", func(t *testing.T) {
		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := testutil.CreateAccounts(vm, ledger, programs.NewEmptyPrograms(), privateKeys, chain)
		require.NoError(t, err)

		txBody := testutil.DeployCounterContractTransaction(accounts[0], chain)

		txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
		txBody.SetPayer(chain.ServiceAddress())

		err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)

		assert.NoError(t, tx.Err)

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

		err = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		tx = fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)

		assert.NoError(t, tx.Err)
	})

	t.Run("account update with checker heavy contract", func(t *testing.T) {
		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := testutil.CreateAccounts(vm, ledger, programs.NewEmptyPrograms(), privateKeys, chain)
		require.NoError(t, err)

		txBody := testutil.DeployCheckerHeavyTransaction(accounts[0], chain)

		txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
		txBody.SetPayer(chain.ServiceAddress())

		err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)

		var parsingCheckingError *runtime.ParsingCheckingError
		assert.ErrorAs(t, tx.Err, &parsingCheckingError)
		assert.ErrorContains(t, tx.Err, "program too ambiguous, local replay limit of 64 tokens exceeded")
	})

	t.Run("account update with set code fails if not signed by service account", func(t *testing.T) {
		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := testutil.CreateAccounts(vm, ledger, programs.NewEmptyPrograms(), privateKeys, chain)
		require.NoError(t, err)

		txBody := testutil.DeployUnauthorizedCounterContractTransaction(accounts[0])

		err = testutil.SignTransaction(txBody, accounts[0], privateKeys[0], 0)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)

		assert.Error(t, tx.Err)

		assert.Contains(t, tx.Err.Error(), "deploying contracts requires authorization from specific accounts")
		assert.Equal(t, (&errors.CadenceRuntimeError{}).Code(), tx.Err.Code())
	})

	t.Run("account update with set code fails if not signed by service account if dis-allowed in the state", func(t *testing.T) {
		ctx := fvm.NewContext(
			zerolog.Nop(),
			fvm.WithChain(chain),
			fvm.WithCadenceLogging(true),
			fvm.WithRestrictedDeployment(false),
		)
		restricted := true
		ledger := testutil.RootBootstrappedLedger(
			vm,
			ctx,
			fvm.WithRestrictedContractDeployment(&restricted),
		)

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := testutil.CreateAccounts(vm, ledger, programs.NewEmptyPrograms(), privateKeys, chain)
		require.NoError(t, err)

		txBody := testutil.DeployUnauthorizedCounterContractTransaction(accounts[0])
		txBody.SetProposalKey(accounts[0], 0, 0)
		txBody.SetPayer(accounts[0])

		err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)
		assert.Error(t, tx.Err)

		assert.Contains(t, tx.Err.Error(), "deploying contracts requires authorization from specific accounts")
		assert.Equal(t, (&errors.CadenceRuntimeError{}).Code(), tx.Err.Code())
	})

	t.Run("account update with set succeeds if not signed by service account if allowed in the state", func(t *testing.T) {
		restricted := false
		ledger := testutil.RootBootstrappedLedger(
			vm,
			ctx,
			fvm.WithRestrictedContractDeployment(&restricted),
		)

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := testutil.CreateAccounts(vm, ledger, programs.NewEmptyPrograms(), privateKeys, chain)
		require.NoError(t, err)

		txBody := testutil.DeployUnauthorizedCounterContractTransaction(accounts[0])
		txBody.SetProposalKey(accounts[0], 0, 0)
		txBody.SetPayer(accounts[0])

		err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)
		require.NoError(t, tx.Err)
	})

	t.Run("account update with update code succeeds if not signed by service account", func(t *testing.T) {
		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := testutil.CreateAccounts(vm, ledger, programs.NewEmptyPrograms(), privateKeys, chain)
		require.NoError(t, err)

		txBody := testutil.DeployCounterContractTransaction(accounts[0], chain)
		txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
		txBody.SetPayer(chain.ServiceAddress())

		err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)
		require.NoError(t, tx.Err)

		txBody = testutil.UpdateUnauthorizedCounterContractTransaction(accounts[0])
		txBody.SetProposalKey(accounts[0], 0, 0)
		txBody.SetPayer(accounts[0])

		err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		tx = fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)
		require.NoError(t, tx.Err)
	})

	t.Run("account update with set code succeeds when account is added as authorized account", func(t *testing.T) {
		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := testutil.CreateAccounts(vm, ledger, programs.NewEmptyPrograms(), privateKeys, chain)
		require.NoError(t, err)

		// setup a new authorizer account
		authTxBody, err := blueprints.SetContractDeploymentAuthorizersTransaction(chain.ServiceAddress(), []flow.Address{chain.ServiceAddress(), accounts[0]})
		require.NoError(t, err)

		authTxBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
		authTxBody.SetPayer(chain.ServiceAddress())
		err = testutil.SignEnvelope(authTxBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)
		authTx := fvm.Transaction(authTxBody, 0)

		err = vm.Run(ctx, authTx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)
		assert.NoError(t, authTx.Err)

		// test deploying a new contract (not authorized by service account)
		txBody := testutil.DeployUnauthorizedCounterContractTransaction(accounts[0])
		txBody.SetProposalKey(accounts[0], 0, 0)
		txBody.SetPayer(accounts[0])

		err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		tx := fvm.Transaction(txBody, 0)
		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)
		require.NoError(t, tx.Err)
	})

	t.Run("account update with set code succeeds when there is a matching audit voucher", func(t *testing.T) {
		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := testutil.CreateAccounts(vm, ledger, programs.NewEmptyPrograms(), privateKeys, chain)
		require.NoError(t, err)

		// Deployent without voucher fails
		txBody := testutil.DeployUnauthorizedCounterContractTransaction(accounts[0])
		err = testutil.SignTransaction(txBody, accounts[0], privateKeys[0], 0)
		require.NoError(t, err)
		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)
		assert.Error(t, tx.Err)
		assert.Contains(t, tx.Err.Error(), "deploying contracts requires authorization from specific accounts")
		assert.Equal(t, (&errors.CadenceRuntimeError{}).Code(), tx.Err.Code())

		// Generate an audit voucher
		authTxBody, err := AuditContractForDeploymentTransaction(
			chain.ServiceAddress(),
			accounts[0],
			testutil.CounterContract)
		require.NoError(t, err)

		authTxBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
		authTxBody.SetPayer(chain.ServiceAddress())
		err = testutil.SignEnvelope(authTxBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)
		authTx := fvm.Transaction(authTxBody, 0)

		err = vm.Run(ctx, authTx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)
		assert.NoError(t, authTx.Err)

		// Deploying with voucher succeeds
		txBody = testutil.DeployUnauthorizedCounterContractTransaction(accounts[0])
		txBody.SetProposalKey(accounts[0], 0, 1)
		txBody.SetPayer(accounts[0])
		err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)
		tx = fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)
		assert.NoError(t, tx.Err)
	})

}

func TestBlockContext_ExecuteTransaction_WithArguments(t *testing.T) {

	t.Parallel()

	chain, vm := createChainAndVm(flow.Mainnet)

	ctx := fvm.NewContext(
		zerolog.Nop(),
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
		check       func(t *testing.T, tx *fvm.TransactionProcedure)
	}{
		{
			label:  "No parameters",
			script: `transaction { execute { log("Hello, World!") } }`,
			args:   [][]byte{arg1},
			check: func(t *testing.T, tx *fvm.TransactionProcedure) {
				assert.Error(t, tx.Err)
			},
		},
		{
			label:  "Single parameter",
			script: `transaction(x: Int) { execute { log(x) } }`,
			args:   [][]byte{arg1},
			check: func(t *testing.T, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Len(t, tx.Logs, 1)
				assert.Equal(t, "42", tx.Logs[0])
			},
		},
		{
			label:  "Multiple parameters",
			script: `transaction(x: Int, y: String) { execute { log(x); log(y) } }`,
			args:   [][]byte{arg1, arg2},
			check: func(t *testing.T, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Len(t, tx.Logs, 2)
				assert.Equal(t, "42", tx.Logs[0])
				assert.Equal(t, `"foo"`, tx.Logs[1])
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
			check: func(t *testing.T, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				assert.ElementsMatch(t, []string{"0x" + chain.ServiceAddress().Hex(), "42", `"foo"`}, tx.Logs)
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

			ledger := testutil.RootBootstrappedLedger(vm, ctx)

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			tx := fvm.Transaction(txBody, 0)

			err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
			require.NoError(t, err)

			tt.check(t, tx)
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
		zerolog.Nop(),
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	var tests = []struct {
		label    string
		script   string
		gasLimit uint64
		check    func(t *testing.T, tx *fvm.TransactionProcedure)
	}{
		{
			label:    "Zero",
			script:   gasLimitScript(100),
			gasLimit: 0,
			check: func(t *testing.T, tx *fvm.TransactionProcedure) {
				// gas limit of zero is ignored by runtime
				require.NoError(t, tx.Err)
			},
		},
		{
			label:    "Insufficient",
			script:   gasLimitScript(100),
			gasLimit: 5,
			check: func(t *testing.T, tx *fvm.TransactionProcedure) {
				assert.Error(t, tx.Err)
			},
		},
		{
			label:    "Sufficient",
			script:   gasLimitScript(100),
			gasLimit: 1000,
			check: func(t *testing.T, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Len(t, tx.Logs, 100)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			txBody := flow.NewTransactionBody().
				SetScript([]byte(tt.script)).
				SetGasLimit(tt.gasLimit)

			ledger := testutil.RootBootstrappedLedger(vm, ctx)

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			tx := fvm.Transaction(txBody, 0)

			err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
			require.NoError(t, err)

			tt.check(t, tx)
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
	}

	t.Run("Storing too much data fails", newVMTest().withBootstrapProcedureOptions(bootstrapOptions...).
		run(
			func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				ctx.LimitAccountStorage = true // this test requires storage limits to be enforced

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
				accounts, err := testutil.CreateAccounts(vm, view, programs, privateKeys, chain)
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

				err = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				assert.Equal(t, (&errors.StorageCapacityExceededError{}).Code(), tx.Err.Code())
			}))
	t.Run("Increasing storage capacity works", newVMTest().withBootstrapProcedureOptions(bootstrapOptions...).
		run(
			func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				ctx.LimitAccountStorage = true // this test requires storage limits to be enforced

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
				accounts, err := testutil.CreateAccounts(vm, view, programs, privateKeys, chain)
				require.NoError(t, err)

				// deposit more flow to increase capacity
				txBody := flow.NewTransactionBody().
					SetScript([]byte(fmt.Sprintf(`
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
					}`, fvm.FungibleTokenAddress(chain).HexWithPrefix(),
						fvm.FlowTokenAddress(chain).HexWithPrefix(),
						"Container",
						hex.EncodeToString([]byte(script))))).
					AddAuthorizer(accounts[0]).
					AddAuthorizer(chain.ServiceAddress()).
					SetProposalKey(chain.ServiceAddress(), 0, 0).
					SetPayer(chain.ServiceAddress())

				err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
				require.NoError(t, err)

				err = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				require.NoError(t, tx.Err)
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
			func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				ctx.MaxStateInteractionSize = 500_000

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
				accounts, err := testutil.CreateAccounts(vm, view, programs, privateKeys, chain)
				require.NoError(t, err)

				txBody := testutil.CreateContractDeploymentTransaction(
					"Container",
					script,
					accounts[0],
					chain)

				txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
				txBody.SetPayer(accounts[0])

				err = testutil.SignPayload(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
				require.NoError(t, err)

				err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				assert.Equal(t, (&errors.LedgerIntractionLimitExceededError{}).Code(), tx.Err.Code())
			}))

	t.Run("Using to much interaction but not failing because of service account", newVMTest().withBootstrapProcedureOptions(bootstrapOptions...).
		withContextOptions(fvm.WithTransactionFeesEnabled(true)).
		run(
			func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				ctx.MaxStateInteractionSize = 500_000
				//ctx.MaxStateInteractionSize = 100_000 // this is not enough to load the FlowServiceAccount for fee deduction

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
				accounts, err := testutil.CreateAccounts(vm, view, programs, privateKeys, chain)
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

				err = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)
				require.NoError(t, tx.Err)
			}))

	t.Run("Using to much interaction fails but does not panic", newVMTest().withBootstrapProcedureOptions(bootstrapOptions...).
		withContextOptions(
			fvm.WithTransactionProcessors(
				fvm.NewTransactionAccountFrozenChecker(),
				fvm.NewTransactionAccountFrozenEnabler(),
				fvm.NewTransactionInvoker(zerolog.Nop()),
			),
		).
		run(
			func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				ctx.MaxStateInteractionSize = 500_000
				//ctx.MaxStateInteractionSize = 100_000 // this is not enough to load the FlowServiceAccount for fee deduction

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
				accounts, err := testutil.CreateAccounts(vm, view, programs, privateKeys, chain)
				require.NoError(t, err)

				_, txBody := testutil.CreateMultiAccountCreationTransaction(t, chain, 40)

				txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
				txBody.SetPayer(accounts[0])

				err = testutil.SignPayload(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
				require.NoError(t, err)

				err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)
				require.Error(t, tx.Err)
				assert.Equal(t, (&errors.LedgerIntractionLimitExceededError{}).Code(), tx.Err.Code())
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
		zerolog.Nop(),
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	t.Run("script success", func(t *testing.T) {
		code := []byte(`
            pub fun main(): Int {
                return 42
            }
        `)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		script := fvm.Script(code)

		err := vm.Run(ctx, script, ledger, programs.NewEmptyPrograms())
		assert.NoError(t, err)

		assert.NoError(t, script.Err)
	})

	t.Run("script failure", func(t *testing.T) {
		code := []byte(`
            pub fun main(): Int {
                assert(1 == 2)
                return 42
            }
        `)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		script := fvm.Script(code)

		err := vm.Run(ctx, script, ledger, programs.NewEmptyPrograms())
		assert.NoError(t, err)

		assert.Error(t, script.Err)
	})

	t.Run("script logs", func(t *testing.T) {
		code := []byte(`
            pub fun main(): Int {
                log("foo")
                log("bar")
                return 42
            }
        `)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		script := fvm.Script(code)

		err := vm.Run(ctx, script, ledger, programs.NewEmptyPrograms())
		assert.NoError(t, err)

		assert.NoError(t, script.Err)
		require.Len(t, script.Logs, 2)
		assert.Equal(t, "\"foo\"", script.Logs[0])
		assert.Equal(t, "\"bar\"", script.Logs[1])
	})

	t.Run("storage ID allocation", func(t *testing.T) {

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := testutil.CreateAccounts(vm, ledger, programs.NewEmptyPrograms(), privateKeys, chain)
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

		txBody := testutil.CreateContractDeploymentTransaction("Test", contract, address, chain)

		txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
		txBody.SetPayer(chain.ServiceAddress())

		err = testutil.SignPayload(txBody, address, privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		require.NoError(t, err)

		assert.NoError(t, tx.Err)

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

		script := fvm.Script(code)

		err = vm.Run(ctx, script, ledger, programs.NewEmptyPrograms())
		assert.NoError(t, err)

		assert.NoError(t, script.Err)
	})
}

func TestBlockContext_GetBlockInfo(t *testing.T) {

	t.Parallel()

	chain, vm := createChainAndVm(flow.Mainnet)

	ctx := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	blocks := new(fvmmock.Blocks)

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

		ledger := testutil.RootBootstrappedLedger(vm, ctx)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(blockCtx, tx, ledger, programs.NewEmptyPrograms())
		assert.NoError(t, err)

		assert.NoError(t, tx.Err)

		require.Len(t, tx.Logs, 2)
		assert.Equal(
			t,
			fmt.Sprintf(
				"Block(height: %v, view: %v, id: 0x%x, timestamp: %.8f)",
				block1.Header.Height,
				block1.Header.View,
				block1.ID(),
				float64(block1.Header.Timestamp.Unix()),
			),
			tx.Logs[0],
		)
		assert.Equal(
			t,
			fmt.Sprintf(
				"Block(height: %v, view: %v, id: 0x%x, timestamp: %.8f)",
				block2.Header.Height,
				block2.Header.View,
				block2.ID(),
				float64(block2.Header.Timestamp.Unix()),
			),
			tx.Logs[1],
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

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		script := fvm.Script(code)

		err := vm.Run(blockCtx, script, ledger, programs.NewEmptyPrograms())
		assert.NoError(t, err)

		assert.NoError(t, script.Err)

		require.Len(t, script.Logs, 2)
		assert.Equal(t,
			fmt.Sprintf(
				"Block(height: %v, view: %v, id: 0x%x, timestamp: %.8f)",
				block1.Header.Height,
				block1.Header.View,
				block1.ID(),
				float64(block1.Header.Timestamp.Unix()),
			),
			script.Logs[0],
		)
		assert.Equal(
			t,
			fmt.Sprintf(
				"Block(height: %v, view: %v, id: 0x%x, timestamp: %.8f)",
				block2.Header.Height,
				block2.Header.View,
				block2.ID(),
				float64(block2.Header.Timestamp.Unix()),
			),
			script.Logs[1],
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

		ledger := testutil.RootBootstrappedLedger(vm, ctx)
		require.NoError(t, err)

		err = vm.Run(blockCtx, fvm.Transaction(tx, 0), ledger, programs.NewEmptyPrograms())
		require.Error(t, err)
	})

	t.Run("panics if external function panics in script", func(t *testing.T) {
		script := []byte(`
            pub fun main() {
                let block = getCurrentBlock()
                let nextBlock = getBlock(at: block.height + UInt64(2))
            }
        `)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)
		err := vm.Run(blockCtx, fvm.Script(script), ledger, programs.NewEmptyPrograms())
		require.Error(t, err)
	})
}

func TestBlockContext_GetAccount(t *testing.T) {

	t.Parallel()

	const count = 100

	chain, vm := createChainAndVm(flow.Mainnet)

	ctx := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(chain),
		fvm.WithCadenceLogging(true),
	)

	sequenceNumber := uint64(0)

	ledger := testutil.RootBootstrappedLedger(vm, ctx)

	programs := programs.NewEmptyPrograms()

	createAccount := func() (flow.Address, crypto.PublicKey) {
		privateKey, txBody := testutil.CreateAccountCreationTransaction(t, chain)

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
		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs)
		require.NoError(t, err)

		assert.NoError(t, tx.Err)

		accountCreatedEvents := filterAccountCreatedEvents(tx.Events)

		require.Len(t, accountCreatedEvents, 1)

		// read the address of the account created (e.g. "0x01" and convert it to flow.address)
		data, err := jsoncdc.Decode(nil, accountCreatedEvents[0].Payload)
		require.NoError(t, err)
		address := flow.Address(data.(cadence.Event).Fields[0].(cadence.Address))

		return address, privateKey.PublicKey(fvm.AccountKeyWeightThreshold).PublicKey
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

		assert.Equal(t, expectedAddress, address)
		accounts[address] = key
	}

	// happy path - get each of the created account and check if it is the right one
	t.Run("get accounts", func(t *testing.T) {
		for address, expectedKey := range accounts {

			account, err := vm.GetAccount(ctx, address, ledger, programs)
			require.NoError(t, err)

			assert.Len(t, account.Keys, 1)
			actualKey := account.Keys[0].PublicKey
			assert.Equal(t, expectedKey, actualKey)
		}
	})

	// non-happy path - get an account that was never created
	t.Run("get a non-existing account", func(t *testing.T) {
		address, err := addressGen.NextAddress()
		require.NoError(t, err)

		var account *flow.Account
		account, err = vm.GetAccount(ctx, address, ledger, programs)
		assert.True(t, errors.IsAccountNotFoundError(err))
		assert.Nil(t, account)
	})
}

func TestBlockContext_UnsafeRandom(t *testing.T) {

	t.Parallel()

	chain, vm := createChainAndVm(flow.Mainnet)

	header := flow.Header{Height: 42}

	ctx := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(chain),
		fvm.WithBlockHeader(&header),
		fvm.WithCadenceLogging(true),
	)

	t.Run("works as transaction", func(t *testing.T) {
		txBody := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                    execute {
                        let rand = unsafeRandom()
                        log(rand)
                    }
                }
            `))

		err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		require.NoError(t, err)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody, 0)

		err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
		assert.NoError(t, err)

		assert.NoError(t, tx.Err)

		require.Len(t, tx.Logs, 1)

		num, err := strconv.ParseUint(tx.Logs[0], 10, 64)
		require.NoError(t, err)
		require.Equal(t, uint64(0x8872445cb397f6d2), num)
	})
}

func TestBlockContext_ExecuteTransaction_CreateAccount_WithMonotonicAddresses(t *testing.T) {

	t.Parallel()

	chain, vm := createChainAndVm(flow.MonotonicEmulator)

	ctx := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(chain),
	)

	ledger := testutil.RootBootstrappedLedger(vm, ctx)

	txBody := flow.NewTransactionBody().
		SetScript(createAccountScript).
		AddAuthorizer(chain.ServiceAddress())

	err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
	require.NoError(t, err)

	tx := fvm.Transaction(txBody, 0)

	err = vm.Run(ctx, tx, ledger, programs.NewEmptyPrograms())
	assert.NoError(t, err)

	assert.NoError(t, tx.Err)

	accountCreatedEvents := filterAccountCreatedEvents(tx.Events)

	require.Len(t, accountCreatedEvents, 1)

	data, err := jsoncdc.Decode(nil, accountCreatedEvents[0].Payload)
	require.NoError(t, err)
	address := flow.Address(data.(cadence.Event).Fields[0].(cadence.Address))

	assert.Equal(t, flow.HexToAddress("05"), address)
}

func TestBlockContext_ExecuteTransaction_FailingTransactions(t *testing.T) {
	getBalance := func(vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, address flow.Address) uint64 {

		code := []byte(fmt.Sprintf(`
					import FungibleToken from 0x%s
					import FlowToken from 0x%s

					pub fun main(account: Address): UFix64 {
						let acct = getAccount(account)
						let vaultRef = acct.getCapability(/public/flowTokenBalance)
							.borrow<&FlowToken.Vault{FungibleToken.Balance}>()
							?? panic("Could not borrow Balance reference to the Vault")

						return vaultRef.balance
					}
				`, fvm.FungibleTokenAddress(chain), fvm.FlowTokenAddress(chain)))
		script := fvm.Script(code).WithArguments(
			jsoncdc.MustEncode(cadence.NewAddress(address)),
		)

		err := vm.Run(ctx, script, view, programs.NewEmptyPrograms())
		require.NoError(t, err)
		require.NoError(t, script.Err)
		return script.Value.ToGoValue().(uint64)
	}

	t.Run("Transaction fails because of storage", newVMTest().withBootstrapProcedureOptions(
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		fvm.WithExecutionMemoryLimit(math.MaxUint64),
	).run(
		func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
			ctx.LimitAccountStorage = true // this test requires storage limits to be enforced

			// Create an account private key.
			privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
			require.NoError(t, err)

			// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
			accounts, err := testutil.CreateAccounts(vm, view, programs, privateKeys, chain)
			require.NoError(t, err)

			balanceBefore := getBalance(vm, chain, ctx, view, accounts[0])

			txBody := transferTokensTx(chain).
				AddAuthorizer(accounts[0]).
				AddArgument(jsoncdc.MustEncode(cadence.UFix64(1))).
				AddArgument(jsoncdc.MustEncode(cadence.NewAddress(chain.ServiceAddress())))

			txBody.SetProposalKey(accounts[0], 0, 0)
			txBody.SetPayer(accounts[0])

			err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
			require.NoError(t, err)

			tx := fvm.Transaction(txBody, 0)

			err = vm.Run(ctx, tx, view, programs)
			require.NoError(t, err)

			require.Equal(t, (&errors.StorageCapacityExceededError{}).Code(), tx.Err.Code())

			balanceAfter := getBalance(vm, chain, ctx, view, accounts[0])

			require.Equal(t, balanceAfter, balanceBefore)
		}),
	)

	t.Run("Transaction fails because of recipient account not existing", newVMTest().withBootstrapProcedureOptions(
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		fvm.WithExecutionMemoryLimit(math.MaxUint64),
	).run(
		func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
			ctx.LimitAccountStorage = true // this test requires storage limits to be enforced

			// Create an account private key.
			privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
			require.NoError(t, err)

			// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
			accounts, err := testutil.CreateAccounts(vm, view, programs, privateKeys, chain)
			require.NoError(t, err)

			// non-existent account
			lastAddress, err := chain.AddressAtIndex((1 << 45) - 1)
			require.NoError(t, err)

			balanceBefore := getBalance(vm, chain, ctx, view, accounts[0])

			// transfer tokens to non-existent account
			txBody := transferTokensTx(chain).
				AddAuthorizer(accounts[0]).
				AddArgument(jsoncdc.MustEncode(cadence.UFix64(1))).
				AddArgument(jsoncdc.MustEncode(cadence.NewAddress(lastAddress)))

			txBody.SetProposalKey(accounts[0], 0, 0)
			txBody.SetPayer(accounts[0])

			err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
			require.NoError(t, err)

			tx := fvm.Transaction(txBody, 0)

			err = vm.Run(ctx, tx, view, programs)
			require.NoError(t, err)

			require.Equal(t, (&errors.CadenceRuntimeError{}).Code(), tx.Err.Code())

			balanceAfter := getBalance(vm, chain, ctx, view, accounts[0])

			require.Equal(t, balanceAfter, balanceBefore)
		}),
	)

	t.Run("Transaction sequence number check fails and sequence number is not incremented", newVMTest().withBootstrapProcedureOptions(
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
	).
		run(
			func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				ctx.LimitAccountStorage = true // this test requires storage limits to be enforced

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
				accounts, err := testutil.CreateAccounts(vm, view, programs, privateKeys, chain)
				require.NoError(t, err)

				txBody := transferTokensTx(chain).
					AddAuthorizer(accounts[0]).
					AddArgument(jsoncdc.MustEncode(cadence.UFix64(1_0000_0000_0000))).
					AddArgument(jsoncdc.MustEncode(cadence.NewAddress(chain.ServiceAddress())))

				// set wrong sequence number
				txBody.SetProposalKey(accounts[0], 0, 10)
				txBody.SetPayer(accounts[0])

				err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)
				require.Equal(t, (&errors.InvalidProposalSeqNumberError{}).Code(), tx.Err.Code())
				require.Equal(t, uint64(0), tx.Err.(*errors.InvalidProposalSeqNumberError).CurrentSeqNumber())
			}),
	)

	t.Run("Transaction invocation fails but sequence number is incremented", newVMTest().withBootstrapProcedureOptions(
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
	).
		run(
			func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, view state.View, programs *programs.Programs) {
				ctx.LimitAccountStorage = true // this test requires storage limits to be enforced

				// Create an account private key.
				privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
				require.NoError(t, err)

				// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
				accounts, err := testutil.CreateAccounts(vm, view, programs, privateKeys, chain)
				require.NoError(t, err)

				txBody := transferTokensTx(chain).
					AddAuthorizer(accounts[0]).
					AddArgument(jsoncdc.MustEncode(cadence.UFix64(1_0000_0000_0000))).
					AddArgument(jsoncdc.MustEncode(cadence.NewAddress(chain.ServiceAddress())))

				txBody.SetProposalKey(accounts[0], 0, 0)
				txBody.SetPayer(accounts[0])

				err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
				require.NoError(t, err)

				tx := fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				require.IsType(t, &errors.CadenceRuntimeError{}, tx.Err)

				// send it again
				tx = fvm.Transaction(txBody, 0)

				err = vm.Run(ctx, tx, view, programs)
				require.NoError(t, err)

				require.Equal(t, (&errors.InvalidProposalSeqNumberError{}).Code(), tx.Err.Code())
				require.Equal(t, uint64(1), tx.Err.(*errors.InvalidProposalSeqNumberError).CurrentSeqNumber())
			}),
	)
}
