package computation

import (
	"context"
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/computation/computer"
	"github.com/onflow/flow-go/engine/execution/state"
	bootstrapexec "github.com/onflow/flow-go/engine/execution/state/bootstrap"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/engine/execution/utils"
	"github.com/onflow/flow-go/engine/testutil/mocklocal"
	"github.com/onflow/flow-go/engine/verification/fetcher"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	completeLedger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/module/chunks"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	exedataprovider "github.com/onflow/flow-go/module/executiondatasync/provider"
	mocktracker "github.com/onflow/flow-go/module/executiondatasync/tracker/mock"
	"github.com/onflow/flow-go/module/metrics"
	requesterunit "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	testVerifyMaxConcurrency = 2
)

var chain = flow.Emulator.Chain()

// In the following tests the system transaction is expected to fail, as the epoch related things are not set up properly.
// This is not relevant to the test, as only the non-system transactions are tested.

func Test_ExecutionMatchesVerification(t *testing.T) {
	t.Run("empty block", func(t *testing.T) {
		executeBlockAndVerify(t,
			[][]*flow.TransactionBody{},
			fvm.DefaultTransactionFees,
			fvm.DefaultMinimumStorageReservation)
	})

	t.Run("single transaction event", func(t *testing.T) {

		deployTx := blueprints.DeployContractTransaction(chain.ServiceAddress(), []byte(""+
			`pub contract Foo {
				pub event FooEvent(x: Int, y: Int)

				pub fun event() { 
					emit FooEvent(x: 2, y: 1)
				}
			}`), "Foo")

		emitTx := &flow.TransactionBody{
			Script: []byte(fmt.Sprintf(`
			import Foo from 0x%s
			transaction {
				prepare() {}
				execute {
					Foo.event()
				}
			}`, chain.ServiceAddress())),
		}

		err := testutil.SignTransactionAsServiceAccount(deployTx, 0, chain)
		require.NoError(t, err)

		err = testutil.SignTransactionAsServiceAccount(emitTx, 1, chain)
		require.NoError(t, err)

		cr := executeBlockAndVerify(t, [][]*flow.TransactionBody{
			{
				deployTx, emitTx,
			},
		}, fvm.BootstrapProcedureFeeParameters{}, fvm.DefaultMinimumStorageReservation)

		colResult := cr.CollectionExecutionResultAt(0)
		txResults := colResult.TransactionResults()
		events := colResult.Events()
		// ensure event is emitted
		require.Empty(t, txResults[0].ErrorMessage)
		require.Empty(t, txResults[1].ErrorMessage)
		require.Len(t, events, 2)
		require.Equal(t, flow.EventType(fmt.Sprintf("A.%s.Foo.FooEvent", chain.ServiceAddress())), events[1].Type)
	})

	t.Run("multiple collections events", func(t *testing.T) {

		deployTx := blueprints.DeployContractTransaction(chain.ServiceAddress(), []byte(""+
			`pub contract Foo {
				pub event FooEvent(x: Int, y: Int)

				pub fun event() { 
					emit FooEvent(x: 2, y: 1)
				}
			}`), "Foo")

		emitTx1 := flow.TransactionBody{
			Script: []byte(fmt.Sprintf(`
			import Foo from 0x%s
			transaction {
				prepare() {}
				execute {
					Foo.event()
				}
			}`, chain.ServiceAddress())),
		}

		// copy txs
		emitTx2 := emitTx1
		emitTx3 := emitTx1

		err := testutil.SignTransactionAsServiceAccount(deployTx, 0, chain)
		require.NoError(t, err)

		err = testutil.SignTransactionAsServiceAccount(&emitTx1, 1, chain)
		require.NoError(t, err)
		err = testutil.SignTransactionAsServiceAccount(&emitTx2, 2, chain)
		require.NoError(t, err)
		err = testutil.SignTransactionAsServiceAccount(&emitTx3, 3, chain)
		require.NoError(t, err)

		cr := executeBlockAndVerify(t, [][]*flow.TransactionBody{
			{
				deployTx, &emitTx1,
			},
			{
				&emitTx2,
			},
			{
				&emitTx3,
			},
		}, fvm.BootstrapProcedureFeeParameters{}, fvm.DefaultMinimumStorageReservation)

		verifyTxResults := func(t *testing.T, colIndex, expResCount int) {
			colResult := cr.CollectionExecutionResultAt(colIndex)
			txResults := colResult.TransactionResults()
			require.Len(t, txResults, expResCount)
			for i := 0; i < expResCount; i++ {
				require.Empty(t, txResults[i].ErrorMessage)
			}
		}

		verifyEvents := func(t *testing.T, colIndex int, eventTypes []flow.EventType) {
			colResult := cr.CollectionExecutionResultAt(colIndex)
			events := colResult.Events()
			require.Len(t, events, len(eventTypes))
			for i, event := range events {
				require.Equal(t, event.Type, eventTypes[i])
			}
		}

		expEventType1 := flow.EventType("flow.AccountContractAdded")
		expEventType2 := flow.EventType(fmt.Sprintf("A.%s.Foo.FooEvent", chain.ServiceAddress()))

		// first collection
		verifyTxResults(t, 0, 2)
		verifyEvents(t, 0, []flow.EventType{expEventType1, expEventType2})

		// second collection
		verifyTxResults(t, 1, 1)
		verifyEvents(t, 1, []flow.EventType{expEventType2})

		// 3rd collection
		verifyTxResults(t, 2, 1)
		verifyEvents(t, 2, []flow.EventType{expEventType2})
	})

	t.Run("with failed storage limit", func(t *testing.T) {

		accountPrivKey, createAccountTx := testutil.CreateAccountCreationTransaction(t, chain)

		// this should return the address of newly created account
		accountAddress, err := chain.AddressAtIndex(5)
		require.NoError(t, err)

		err = testutil.SignTransactionAsServiceAccount(createAccountTx, 0, chain)
		require.NoError(t, err)

		addKeyTx := testutil.CreateAddAnAccountKeyMultipleTimesTransaction(t, &accountPrivKey, 100).AddAuthorizer(accountAddress)
		err = testutil.SignTransaction(addKeyTx, accountAddress, accountPrivKey, 0)
		require.NoError(t, err)

		minimumStorage, err := cadence.NewUFix64("0.00010807")
		require.NoError(t, err)

		cr := executeBlockAndVerify(t, [][]*flow.TransactionBody{
			{
				createAccountTx,
			},
			{
				addKeyTx,
			},
		}, fvm.DefaultTransactionFees, minimumStorage)

		colResult := cr.CollectionExecutionResultAt(0)
		txResults := colResult.TransactionResults()
		// storage limit error
		assert.Len(t, txResults, 1)
		assert.Equal(t, txResults[0].ErrorMessage, "")
		// ensure events from the first transaction is emitted
		require.Len(t, colResult.Events(), 10)

		colResult = cr.CollectionExecutionResultAt(1)
		txResults = colResult.TransactionResults()
		assert.Len(t, txResults, 1)
		// storage limit error
		assert.Contains(t, txResults[0].ErrorMessage, errors.ErrCodeStorageCapacityExceeded.String())
		// ensure fee deduction events are emitted even though tx fails
		require.Len(t, colResult.Events(), 3)
	})

	t.Run("with failed transaction fee deduction", func(t *testing.T) {
		accountPrivKey, createAccountTx := testutil.CreateAccountCreationTransaction(t, chain)
		// this should return the address of newly created account
		accountAddress, err := chain.AddressAtIndex(5)
		require.NoError(t, err)

		err = testutil.SignTransactionAsServiceAccount(createAccountTx, 0, chain)
		require.NoError(t, err)

		spamTx := &flow.TransactionBody{
			Script: []byte(`
			transaction {
				prepare() {}
				execute {
					var s: Int256 = 1024102410241024
					var i = 0
					var a = Int256(7)
					var b = Int256(5)
					var c = Int256(2)
					while i < 150000 {
						s = s * a
						s = s / b
						s = s / c
						i = i + 1
					}
					log(i)
				}
			}`),
		}

		spamTx.SetComputeLimit(800000)
		err = testutil.SignTransaction(spamTx, accountAddress, accountPrivKey, 0)
		require.NoError(t, err)

		require.NoError(t, err)

		cr := executeBlockAndVerifyWithParameters(t, [][]*flow.TransactionBody{
			{
				createAccountTx,
				spamTx,
			},
		},
			[]fvm.Option{
				fvm.WithTransactionFeesEnabled(true),
				fvm.WithAccountStorageLimit(true),
				// make sure we don't run out of memory first.
				fvm.WithMemoryLimit(20_000_000_000),
			}, []fvm.BootstrapProcedureOption{
				fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
				fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
				fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
				fvm.WithTransactionFee(fvm.DefaultTransactionFees),
				fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
			})

		colResult := cr.CollectionExecutionResultAt(0)
		txResults := colResult.TransactionResults()
		events := colResult.Events()

		// no error
		assert.Equal(t, txResults[0].ErrorMessage, "")

		// ensure events from the first transaction is emitted. Since transactions are in the same block, get all events from Events[0]
		transactionEvents := 0
		for _, event := range events {
			if event.TransactionID == txResults[0].TransactionID {
				transactionEvents += 1
			}
		}
		require.Equal(t, 10, transactionEvents)

		assert.Contains(t, txResults[1].ErrorMessage, errors.ErrCodeStorageCapacityExceeded.String())

		// ensure tx fee deduction events are emitted even though tx failed
		transactionEvents = 0
		for _, event := range events {
			if event.TransactionID == txResults[1].TransactionID {
				transactionEvents += 1
			}
		}
		require.Equal(t, 3, transactionEvents)
	})

}

func TestTransactionFeeDeduction(t *testing.T) {

	type testCase struct {
		name          string
		fundWith      uint64
		tryToTransfer uint64
		checkResult   func(t *testing.T, cr *execution.ComputationResult)
	}

	txFees := uint64(1_000)
	fundingAmount := uint64(100_000_000)
	transferAmount := uint64(123_456)

	sc := systemcontracts.SystemContractsForChain(chain.ChainID())

	depositedEvent := fmt.Sprintf("A.%s.FlowToken.TokensDeposited", sc.FlowToken.Address)
	withdrawnEvent := fmt.Sprintf("A.%s.FlowToken.TokensWithdrawn", sc.FlowToken.Address)

	testCases := []testCase{
		{
			name:          "Transaction fee deduction emits events",
			fundWith:      fundingAmount,
			tryToTransfer: 0,
			checkResult: func(t *testing.T, cr *execution.ComputationResult) {
				txResults := cr.AllTransactionResults()

				require.Empty(t, txResults[0].ErrorMessage)
				require.Empty(t, txResults[1].ErrorMessage)
				require.Empty(t, txResults[2].ErrorMessage)

				var deposits []flow.Event
				var withdraws []flow.Event

				// events of the first collection
				events := cr.CollectionExecutionResultAt(2).Events()
				for _, e := range events {
					if string(e.Type) == depositedEvent {
						deposits = append(deposits, e)
					}
					if string(e.Type) == withdrawnEvent {
						withdraws = append(withdraws, e)
					}
				}

				require.Len(t, deposits, 2)
				require.Len(t, withdraws, 2)
			},
		},
		{
			name:          "If just enough balance, fees are still deducted",
			fundWith:      txFees + transferAmount,
			tryToTransfer: transferAmount,
			checkResult: func(t *testing.T, cr *execution.ComputationResult) {
				txResults := cr.AllTransactionResults()

				require.Empty(t, txResults[0].ErrorMessage)
				require.Empty(t, txResults[1].ErrorMessage)
				require.Empty(t, txResults[2].ErrorMessage)

				var deposits []flow.Event
				var withdraws []flow.Event

				// events of the last collection
				events := cr.CollectionExecutionResultAt(2).Events()
				for _, e := range events {
					if string(e.Type) == depositedEvent {
						deposits = append(deposits, e)
					}
					if string(e.Type) == withdrawnEvent {
						withdraws = append(withdraws, e)
					}
				}

				require.Len(t, deposits, 2)
				require.Len(t, withdraws, 2)
			},
		},
		{
			// this is an edge case that is not applicable to any network.
			// If storage limits were on this would fail due to storage limits
			name:          "If not enough balance, transaction succeeds and fees are deducted to 0",
			fundWith:      txFees,
			tryToTransfer: 1,
			checkResult: func(t *testing.T, cr *execution.ComputationResult) {
				txResults := cr.AllTransactionResults()

				require.Empty(t, txResults[0].ErrorMessage)
				require.Empty(t, txResults[1].ErrorMessage)
				require.Empty(t, txResults[2].ErrorMessage)

				var deposits []flow.Event
				var withdraws []flow.Event

				// events of the last collection
				events := cr.CollectionExecutionResultAt(2).Events()
				for _, e := range events {
					if string(e.Type) == depositedEvent {
						deposits = append(deposits, e)
					}
					if string(e.Type) == withdrawnEvent {
						withdraws = append(withdraws, e)
					}
				}

				require.Len(t, deposits, 2)
				require.Len(t, withdraws, 2)
			},
		},
		{
			name:          "If tx fails, fees are deducted",
			fundWith:      fundingAmount,
			tryToTransfer: 2 * fundingAmount,
			checkResult: func(t *testing.T, cr *execution.ComputationResult) {
				txResults := cr.AllTransactionResults()

				require.Empty(t, txResults[0].ErrorMessage)
				require.Empty(t, txResults[1].ErrorMessage)
				require.Contains(t, txResults[2].ErrorMessage, "Error Code: 1101")

				var deposits []flow.Event
				var withdraws []flow.Event

				// events of the last collection
				events := cr.CollectionExecutionResultAt(2).Events()
				for _, e := range events {
					if string(e.Type) == depositedEvent {
						deposits = append(deposits, e)
					}
					if string(e.Type) == withdrawnEvent {
						withdraws = append(withdraws, e)
					}
				}

				require.Len(t, deposits, 1)
				require.Len(t, withdraws, 1)
			},
		},
	}

	testCasesWithStorageEnabled := []testCase{
		{
			name:          "Transaction fee deduction emits events",
			fundWith:      fundingAmount,
			tryToTransfer: 0,
			checkResult: func(t *testing.T, cr *execution.ComputationResult) {
				txResults := cr.AllTransactionResults()

				require.Empty(t, txResults[0].ErrorMessage)
				require.Empty(t, txResults[1].ErrorMessage)
				require.Empty(t, txResults[2].ErrorMessage)

				var deposits []flow.Event
				var withdraws []flow.Event

				// events of the last collection
				events := cr.CollectionExecutionResultAt(2).Events()
				for _, e := range events {
					if string(e.Type) == depositedEvent {
						deposits = append(deposits, e)
					}
					if string(e.Type) == withdrawnEvent {
						withdraws = append(withdraws, e)
					}
				}

				require.Len(t, deposits, 2)
				require.Len(t, withdraws, 2)
			},
		},
		{
			name:          "If just enough balance, fees are deducted",
			fundWith:      txFees + transferAmount,
			tryToTransfer: transferAmount,
			checkResult: func(t *testing.T, cr *execution.ComputationResult) {
				txResults := cr.AllTransactionResults()

				require.Empty(t, txResults[0].ErrorMessage)
				require.Empty(t, txResults[1].ErrorMessage)
				require.Empty(t, txResults[2].ErrorMessage)

				var deposits []flow.Event
				var withdraws []flow.Event

				// events of the last collection
				events := cr.CollectionExecutionResultAt(2).Events()
				for _, e := range events {
					if string(e.Type) == depositedEvent {
						deposits = append(deposits, e)
					}
					if string(e.Type) == withdrawnEvent {
						withdraws = append(withdraws, e)
					}
				}

				require.Len(t, deposits, 2)
				require.Len(t, withdraws, 2)
			},
		},
		{
			name:          "If tx fails, fees are still deducted and fee deduction events are emitted",
			fundWith:      fundingAmount,
			tryToTransfer: 2 * fundingAmount,
			checkResult: func(t *testing.T, cr *execution.ComputationResult) {
				txResults := cr.AllTransactionResults()

				require.Empty(t, txResults[0].ErrorMessage)
				require.Empty(t, txResults[1].ErrorMessage)
				require.Contains(t, txResults[2].ErrorMessage, "Error Code: 1101")

				var deposits []flow.Event
				var withdraws []flow.Event

				// events of the last collection
				events := cr.CollectionExecutionResultAt(2).Events()
				for _, e := range events {
					if string(e.Type) == depositedEvent {
						deposits = append(deposits, e)
					}
					if string(e.Type) == withdrawnEvent {
						withdraws = append(withdraws, e)
					}
				}

				require.Len(t, deposits, 1)
				require.Len(t, withdraws, 1)
			},
		},
		{
			name:          "If balance at minimum, transaction fails, fees are deducted and fee deduction events are emitted",
			fundWith:      0,
			tryToTransfer: 0,
			checkResult: func(t *testing.T, cr *execution.ComputationResult) {
				txResults := cr.AllTransactionResults()

				require.Empty(t, txResults[0].ErrorMessage)
				require.Empty(t, txResults[1].ErrorMessage)
				require.Contains(t, txResults[2].ErrorMessage, errors.ErrCodeStorageCapacityExceeded.String())

				var deposits []flow.Event
				var withdraws []flow.Event

				// events of the last collection
				events := cr.CollectionExecutionResultAt(2).Events()
				for _, e := range events {
					if string(e.Type) == depositedEvent {
						deposits = append(deposits, e)
					}
					if string(e.Type) == withdrawnEvent {
						withdraws = append(withdraws, e)
					}
				}

				require.Len(t, deposits, 1)
				require.Len(t, withdraws, 1)
			},
		},
	}

	transferTokensTx := func(chain flow.Chain) *flow.TransactionBody {
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
							}`, sc.FungibleToken.Address, sc.FlowToken.Address)),
			)
	}

	runTx := func(tc testCase,
		opts []fvm.Option,
		bootstrapOpts []fvm.BootstrapProcedureOption) func(t *testing.T) {
		return func(t *testing.T) {
			// ==== Create an account ====
			privateKey, createAccountTx := testutil.CreateAccountCreationTransaction(t, chain)

			// this should return the address of newly created account
			address, err := chain.AddressAtIndex(5)
			require.NoError(t, err)

			err = testutil.SignTransactionAsServiceAccount(createAccountTx, 0, chain)
			require.NoError(t, err)

			// ==== Transfer tokens to new account ====
			transferTx := transferTokensTx(chain).
				AddAuthorizer(chain.ServiceAddress()).
				AddArgument(jsoncdc.MustEncode(cadence.UFix64(tc.fundWith))).
				AddArgument(jsoncdc.MustEncode(cadence.NewAddress(address)))

			transferTx.SetProposalKey(chain.ServiceAddress(), 0, 1)
			transferTx.SetPayer(chain.ServiceAddress())

			err = testutil.SignEnvelope(
				transferTx,
				chain.ServiceAddress(),
				unittest.ServiceAccountPrivateKey,
			)
			require.NoError(t, err)

			// ==== Transfer tokens from new account ====

			transferTx2 := transferTokensTx(chain).
				AddAuthorizer(address).
				AddArgument(jsoncdc.MustEncode(cadence.UFix64(tc.tryToTransfer))).
				AddArgument(jsoncdc.MustEncode(cadence.NewAddress(chain.ServiceAddress())))

			transferTx2.SetProposalKey(address, 0, 0)
			transferTx2.SetPayer(address)

			err = testutil.SignEnvelope(
				transferTx2,
				address,
				privateKey,
			)
			require.NoError(t, err)

			cr := executeBlockAndVerifyWithParameters(t, [][]*flow.TransactionBody{
				{
					createAccountTx,
				},
				{
					transferTx,
				},
				{
					transferTx2,
				},
			}, opts, bootstrapOpts)

			tc.checkResult(t, cr)
		}
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Transaction Fees without storage %d: %s", i, tc.name), runTx(tc, []fvm.Option{
			fvm.WithTransactionFeesEnabled(true),
			fvm.WithAccountStorageLimit(false),
		}, []fvm.BootstrapProcedureOption{
			fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
			fvm.WithTransactionFee(fvm.DefaultTransactionFees),
		}))
	}

	for i, tc := range testCasesWithStorageEnabled {
		t.Run(fmt.Sprintf("Transaction Fees with storage %d: %s", i, tc.name), runTx(tc, []fvm.Option{
			fvm.WithTransactionFeesEnabled(true),
			fvm.WithAccountStorageLimit(true),
		}, []fvm.BootstrapProcedureOption{
			fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
			fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
			fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
			fvm.WithTransactionFee(fvm.DefaultTransactionFees),
			fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		}))
	}
}

func executeBlockAndVerifyWithParameters(t *testing.T,
	txs [][]*flow.TransactionBody,
	opts []fvm.Option,
	bootstrapOpts []fvm.BootstrapProcedureOption) *execution.ComputationResult {
	vm := fvm.NewVirtualMachine()

	logger := zerolog.Nop()

	opts = append(opts, fvm.WithChain(chain))
	opts = append(opts, fvm.WithLogger(logger))
	opts = append(opts, fvm.WithBlocks(&environment.NoopBlockFinder{}))

	fvmContext :=
		fvm.NewContext(
			opts...,
		)

	collector := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()

	wal := &fixtures.NoopWAL{}

	ledger, err := completeLedger.NewLedger(wal, 100, collector, logger, completeLedger.DefaultPathFinderVersion)
	require.NoError(t, err)

	compactor := fixtures.NewNoopCompactor(ledger)
	<-compactor.Ready()
	defer func() {
		<-ledger.Done()
		<-compactor.Done()
	}()

	bootstrapper := bootstrapexec.NewBootstrapper(logger)

	initialCommit, err := bootstrapper.BootstrapLedger(
		ledger,
		unittest.ServiceAccountPublicKey,
		chain,
		bootstrapOpts...,
	)

	require.NoError(t, err)

	ledgerCommiter := committer.NewLedgerViewCommitter(ledger, tracer)

	bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	trackerStorage := mocktracker.NewMockStorage()

	prov := exedataprovider.NewProvider(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		execution_data.DefaultSerializer,
		bservice,
		trackerStorage,
	)

	// generates signing identity including staking key for signing
	myIdentity := unittest.IdentityFixture()
	seed := make([]byte, crypto.KeyGenSeedMinLen)
	n, err := rand.Read(seed)
	require.Equal(t, n, crypto.KeyGenSeedMinLen)
	require.NoError(t, err)
	sk, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
	require.NoError(t, err)
	myIdentity.StakingPubKey = sk.PublicKey()
	me := mocklocal.NewMockLocal(sk, myIdentity.ID(), t)

	// used by computer to generate the prng used in the service tx
	stateForRandomSource := testutil.ProtocolStateWithSourceFixture(nil)

	blockComputer, err := computer.NewBlockComputer(
		vm,
		fvmContext,
		collector,
		tracer,
		logger,
		ledgerCommiter,
		me,
		prov,
		nil,
		stateForRandomSource,
		testVerifyMaxConcurrency)
	require.NoError(t, err)

	executableBlock := unittest.ExecutableBlockFromTransactions(chain.ChainID(), txs)
	executableBlock.StartState = &initialCommit

	prevResultId := unittest.IdentifierFixture()
	computationResult, err := blockComputer.ExecuteBlock(
		context.Background(),
		prevResultId,
		executableBlock,
		state.NewLedgerStorageSnapshot(
			ledger,
			initialCommit),
		derived.NewEmptyDerivedBlockData(0))
	require.NoError(t, err)

	spockHasher := utils.NewSPOCKHasher()

	for i := 0; i < computationResult.BlockExecutionResult.Size(); i++ {
		res := computationResult.CollectionExecutionResultAt(i)
		snapshot := res.ExecutionSnapshot()
		valid, err := crypto.SPOCKVerifyAgainstData(
			myIdentity.StakingPubKey,
			computationResult.Spocks[i],
			snapshot.SpockSecret,
			spockHasher)
		require.NoError(t, err)
		require.True(t, valid)
	}

	receipt := computationResult.ExecutionReceipt
	receiptID := receipt.ID()
	valid, err := myIdentity.StakingPubKey.Verify(
		receipt.ExecutorSignature,
		receiptID[:],
		utils.NewExecutionReceiptHasher())

	require.NoError(t, err)
	require.True(t, valid)

	chdps := computationResult.AllChunkDataPacks()
	require.Equal(t, len(chdps), len(receipt.Spocks))

	er := &computationResult.ExecutionResult

	verifier := chunks.NewChunkVerifier(vm, fvmContext, logger)

	vcds := make([]*verification.VerifiableChunkData, er.Chunks.Len())

	for i, chunk := range er.Chunks {
		isSystemChunk := i == er.Chunks.Len()-1
		offsetForChunk, err := fetcher.TransactionOffsetForChunk(er.Chunks, chunk.Index)
		require.NoError(t, err)

		vcds[i] = &verification.VerifiableChunkData{
			IsSystemChunk:     isSystemChunk,
			Chunk:             chunk,
			Header:            executableBlock.Block.Header,
			Result:            er,
			ChunkDataPack:     chdps[i],
			EndState:          chunk.EndState,
			TransactionOffset: offsetForChunk,
			// returns the same RandomSource used by the computer
			Snapshot: stateForRandomSource.AtBlockID(chunk.BlockID),
		}
	}

	require.Len(t, vcds, len(txs)+1) // +1 for system chunk

	for _, vcd := range vcds {
		spockSecret, err := verifier.Verify(vcd)
		assert.NoError(t, err)
		assert.NotNil(t, spockSecret)
	}

	return computationResult
}

func executeBlockAndVerify(t *testing.T,
	txs [][]*flow.TransactionBody,
	txFees fvm.BootstrapProcedureFeeParameters,
	minStorageBalance cadence.UFix64) *execution.ComputationResult {
	return executeBlockAndVerifyWithParameters(t,
		txs,
		[]fvm.Option{
			fvm.WithTransactionFeesEnabled(true),
			fvm.WithAccountStorageLimit(true),
		}, []fvm.BootstrapProcedureOption{
			fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
			fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
			fvm.WithMinimumStorageReservation(minStorageBalance),
			fvm.WithTransactionFee(txFees),
			fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		})
}
