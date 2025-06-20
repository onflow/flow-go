package fvm_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"strings"
	"testing"

	"github.com/ipfs/boxo/blockstore"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/onflow/atree"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/stdlib"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/templates"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/computation/computer"
	exeState "github.com/onflow/flow-go/engine/execution/state"
	bootstrapexec "github.com/onflow/flow-go/engine/execution/state/bootstrap"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	reusableRuntime "github.com/onflow/flow-go/fvm/runtime"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/fvm/tracing"
	completeLedger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/provider"
	mocktracker "github.com/onflow/flow-go/module/executiondatasync/tracker/mock"
	"github.com/onflow/flow-go/module/metrics"
	moduleMock "github.com/onflow/flow-go/module/mock"
	requesterunit "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

// TODO (ramtin)
// - move block computer logic to its own location inside exec node, so we embed a real
// block computer, simplify this one to use an in-mem ledger
// - time track different part of execution (execution, ledger update, ...)
// - add metrics like number of registers touched, proof size
//

type TestBenchBlockExecutor interface {
	ExecuteCollections(tb testing.TB, collections [][]*flow.TransactionBody) *execution.ComputationResult
	Chain(tb testing.TB) flow.Chain
	ServiceAccount(tb testing.TB) *TestBenchAccount
}

type TestBenchAccount struct {
	SeqNumber  uint64
	PrivateKey flow.AccountPrivateKey
	Address    flow.Address
}

func (account *TestBenchAccount) RetAndIncSeqNumber() uint64 {
	account.SeqNumber++
	return account.SeqNumber - 1
}

func (account *TestBenchAccount) DeployContract(b *testing.B, blockExec TestBenchBlockExecutor, contractName string, contract string) {
	serviceAccount := blockExec.ServiceAccount(b)
	txBody := testutil.CreateContractDeploymentTransaction(
		contractName,
		contract,
		account.Address,
		blockExec.Chain(b))

	txBody.SetProposalKey(serviceAccount.Address, 0, serviceAccount.RetAndIncSeqNumber())
	txBody.SetPayer(serviceAccount.Address)

	err := testutil.SignPayload(txBody, account.Address, account.PrivateKey)
	require.NoError(b, err)

	err = testutil.SignEnvelope(txBody, serviceAccount.Address, serviceAccount.PrivateKey)
	require.NoError(b, err)

	computationResult := blockExec.ExecuteCollections(b, [][]*flow.TransactionBody{{txBody}})
	require.Empty(b, computationResult.AllTransactionResults()[0].ErrorMessage)
}

func (account *TestBenchAccount) AddArrayToStorage(b *testing.B, blockExec TestBenchBlockExecutor, list []string) {
	serviceAccount := blockExec.ServiceAccount(b)
	txBody := flow.NewTransactionBody().
		SetScript([]byte(`
		transaction(list: [String]) {
		  prepare(acct: auth(Storage) &Account) {
			acct.storage.load<[String]>(from: /storage/test)
			acct.storage.save(list, to: /storage/test)
		  }
		  execute {}
		}
		`)).
		AddAuthorizer(account.Address)

	cadenceArrayValues := make([]cadence.Value, len(list))
	for i, item := range list {
		cadenceArrayValues[i] = cadence.String(item)
	}
	cadenceArray, err := jsoncdc.Encode(cadence.NewArray(cadenceArrayValues))
	require.NoError(b, err)
	txBody.AddArgument(cadenceArray)

	txBody.SetProposalKey(serviceAccount.Address, 0, serviceAccount.RetAndIncSeqNumber())
	txBody.SetPayer(serviceAccount.Address)

	if account.Address != serviceAccount.Address {
		err = testutil.SignPayload(txBody, account.Address, account.PrivateKey)
		require.NoError(b, err)
	}

	err = testutil.SignEnvelope(txBody, serviceAccount.Address, serviceAccount.PrivateKey)
	require.NoError(b, err)

	computationResult := blockExec.ExecuteCollections(b, [][]*flow.TransactionBody{{txBody}})
	require.Empty(b, computationResult.AllTransactionResults()[0].ErrorMessage)
}

// BasicBlockExecutor executes blocks in sequence and applies all changes (not fork aware)
type BasicBlockExecutor struct {
	blockComputer         computer.BlockComputer
	derivedChainData      *derived.DerivedChainData
	activeSnapshot        snapshot.SnapshotTree
	activeStateCommitment flow.StateCommitment
	chain                 flow.Chain
	serviceAccount        *TestBenchAccount
	onStopFunc            func()
}

func NewBasicBlockExecutor(tb testing.TB, chain flow.Chain, logger zerolog.Logger) *BasicBlockExecutor {
	vm := fvm.NewVirtualMachine()

	// a big interaction limit since that is not what's being tested.
	interactionLimit := fvm.DefaultMaxInteractionSize * uint64(1000)

	opts := []fvm.Option{
		fvm.WithTransactionFeesEnabled(true),
		fvm.WithAccountStorageLimit(true),
		fvm.WithChain(chain),
		fvm.WithLogger(logger),
		fvm.WithMaxStateInteractionSize(interactionLimit),
		fvm.WithReusableCadenceRuntimePool(
			reusableRuntime.NewReusableCadenceRuntimePool(
				computation.ReusableCadenceRuntimePoolSize,
				runtime.Config{},
			),
		),
		fvm.WithEVMEnabled(true),
	}
	fvmContext := fvm.NewContext(opts...)

	collector := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()

	wal := &fixtures.NoopWAL{}

	ledger, err := completeLedger.NewLedger(wal, 100, collector, logger, completeLedger.DefaultPathFinderVersion)
	require.NoError(tb, err)

	compactor := fixtures.NewNoopCompactor(ledger)
	<-compactor.Ready()

	onStopFunc := func() {
		<-ledger.Done()
		<-compactor.Done()
	}

	bootstrapper := bootstrapexec.NewBootstrapper(logger)

	serviceAccount := &TestBenchAccount{
		SeqNumber:  0,
		PrivateKey: unittest.ServiceAccountPrivateKey,
		Address:    chain.ServiceAddress(),
	}

	initialCommit, err := bootstrapper.BootstrapLedger(
		ledger,
		unittest.ServiceAccountPublicKey,
		chain,
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithTransactionFee(fvm.DefaultTransactionFees),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
	)
	require.NoError(tb, err)

	bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	trackerStorage := mocktracker.NewMockStorage()

	prov := provider.NewProvider(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		execution_data.DefaultSerializer,
		bservice,
		trackerStorage,
	)

	me := new(moduleMock.Local)
	me.On("NodeID").Return(unittest.IdentifierFixture())
	me.On("Sign", mock.Anything, mock.Anything).Return(nil, nil)
	me.On("SignFunc", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)

	ledgerCommitter := committer.NewLedgerViewCommitter(ledger, tracer)
	blockComputer, err := computer.NewBlockComputer(
		vm,
		fvmContext,
		collector,
		tracer,
		logger,
		ledgerCommitter,
		me,
		prov,
		nil,
		testutil.ProtocolStateWithSourceFixture(nil),
		1) // We're interested in fvm's serial execution time
	require.NoError(tb, err)

	activeSnapshot := snapshot.NewSnapshotTree(
		exeState.NewLedgerStorageSnapshot(ledger, initialCommit))

	derivedChainData, err := derived.NewDerivedChainData(
		derived.DefaultDerivedDataCacheSize)
	require.NoError(tb, err)

	return &BasicBlockExecutor{
		blockComputer:         blockComputer,
		derivedChainData:      derivedChainData,
		activeStateCommitment: initialCommit,
		activeSnapshot:        activeSnapshot,
		chain:                 chain,
		serviceAccount:        serviceAccount,
		onStopFunc:            onStopFunc,
	}
}

func (b *BasicBlockExecutor) Chain(_ testing.TB) flow.Chain {
	return b.chain
}

func (b *BasicBlockExecutor) ServiceAccount(_ testing.TB) *TestBenchAccount {
	return b.serviceAccount
}

func (b *BasicBlockExecutor) ExecuteCollections(tb testing.TB, collections [][]*flow.TransactionBody) *execution.ComputationResult {
	executableBlock := unittest.ExecutableBlockFromTransactions(b.chain.ChainID(), collections)
	executableBlock.StartState = &b.activeStateCommitment

	derivedBlockData := b.derivedChainData.GetOrCreateDerivedBlockData(
		executableBlock.ID(),
		executableBlock.ParentID())

	computationResult, err := b.blockComputer.ExecuteBlock(
		context.Background(),
		unittest.IdentifierFixture(),
		executableBlock,
		b.activeSnapshot,
		derivedBlockData)
	require.NoError(tb, err)

	b.activeStateCommitment = computationResult.CurrentEndState()

	for _, snapshot := range computationResult.AllExecutionSnapshots() {
		b.activeSnapshot = b.activeSnapshot.Append(snapshot)
	}

	return computationResult
}

func (b *BasicBlockExecutor) RunWithLedger(tb testing.TB, f func(ledger atree.Ledger)) {
	ts := state.NewTransactionState(b.activeSnapshot, state.DefaultParameters())

	accounts := environment.NewAccounts(ts)
	meter := environment.NewMeter(ts)

	valueStore := environment.NewValueStore(
		tracing.NewMockTracerSpan(),
		meter,
		accounts,
	)

	f(valueStore)

	newSnapshot, err := ts.FinalizeMainTransaction()
	require.NoError(tb, err)

	b.activeSnapshot = b.activeSnapshot.Append(newSnapshot)
}

func (b *BasicBlockExecutor) SetupAccounts(tb testing.TB, privateKeys []flow.AccountPrivateKey) []TestBenchAccount {
	accounts := make([]TestBenchAccount, 0)
	serviceAddress := b.Chain(tb).ServiceAddress()

	for _, privateKey := range privateKeys {
		accountKey := flowsdk.NewAccountKey().
			FromPrivateKey(privateKey.PrivateKey).
			SetWeight(fvm.AccountKeyWeightThreshold).
			SetHashAlgo(privateKey.HashAlgo).
			SetSigAlgo(privateKey.SignAlgo)

		sdkTX, err := templates.CreateAccount([]*flowsdk.AccountKey{accountKey}, []templates.Contract{}, flowsdk.BytesToAddress(serviceAddress.Bytes()))
		require.NoError(tb, err)

		txBody := flow.NewTransactionBody().
			SetScript(sdkTX.Script).
			SetArguments(sdkTX.Arguments).
			AddAuthorizer(serviceAddress).
			SetProposalKey(serviceAddress, 0, b.ServiceAccount(tb).RetAndIncSeqNumber()).
			SetPayer(serviceAddress)

		err = testutil.SignEnvelope(txBody, b.Chain(tb).ServiceAddress(), unittest.ServiceAccountPrivateKey)
		require.NoError(tb, err)

		computationResult := b.ExecuteCollections(tb, [][]*flow.TransactionBody{{txBody}})
		require.Empty(tb, computationResult.AllTransactionResults()[0].ErrorMessage)

		var addr flow.Address

		for _, event := range computationResult.AllEvents() {
			if event.Type == flow.EventAccountCreated {
				data, err := ccf.Decode(nil, event.Payload)
				if err != nil {
					tb.Fatal("setup account failed, error decoding events")
				}

				address := cadence.SearchFieldByName(
					data.(cadence.Event),
					stdlib.AccountEventAddressParameter.Identifier,
				).(cadence.Address)

				addr = flow.ConvertAddress(address)
				break
			}
		}
		if addr == flow.EmptyAddress {
			tb.Fatal("setup account failed, no account creation event emitted")
		}
		accounts = append(accounts, TestBenchAccount{Address: addr, PrivateKey: privateKey, SeqNumber: 0})
	}

	return accounts
}

type logExtractor struct {
	TimeSpent       map[string]uint64
	InteractionUsed map[string]uint64
}

type txWeights struct {
	TXHash                string `json:"tx_id"`
	LedgerInteractionUsed uint64 `json:"ledgerInteractionUsed"`
	ComputationUsed       uint   `json:"computationUsed"`
	MemoryEstimate        uint   `json:"memoryEstimate"`
}

type txSuccessfulLog struct {
	TxID          string `json:"tx_id"`
	TimeSpentInMS uint64 `json:"timeSpentInMS"`
}

func (l *logExtractor) Write(p []byte) (n int, err error) {
	if strings.Contains(string(p), "transaction execution data") {
		w := txWeights{}
		err := json.Unmarshal(p, &w)

		if err != nil {
			fmt.Println(err)
			return len(p), nil
		}

		l.InteractionUsed[w.TXHash] = w.LedgerInteractionUsed
	}
	if strings.Contains(string(p), "transaction executed successfully") {
		w := txSuccessfulLog{}
		err := json.Unmarshal(p, &w)

		if err != nil {
			fmt.Println(err)
			return len(p), nil
		}
		l.TimeSpent[w.TxID] = w.TimeSpentInMS
	}
	return len(p), nil

}

var _ io.Writer = &logExtractor{}

type benchTransactionContext struct {
	EvmTestContract *testutils.TestContract
	EvmTestAccount  *testutils.EOATestAccount
}

// BenchmarkRuntimeEmptyTransaction simulates executing blocks with `transactionsPerBlock`
// where each transaction is an empty transaction
func BenchmarkRuntimeTransaction(b *testing.B) {

	transactionsPerBlock := 10

	longString := strings.Repeat("0", 1000)

	chain := flow.Testnet.Chain()

	logE := &logExtractor{
		TimeSpent:       map[string]uint64{},
		InteractionUsed: map[string]uint64{},
	}
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())

	testContractAddress, err := chain.AddressAtIndex(systemcontracts.LastSystemAccountIndex + 1)
	require.NoError(b, err)

	benchTransaction := func(
		b *testing.B,
		txStringFunc func(b *testing.B, context benchTransactionContext) string,
	) {

		logger := zerolog.New(logE).Level(zerolog.DebugLevel)

		blockExecutor := NewBasicBlockExecutor(b, chain, logger)
		defer func() {
			blockExecutor.onStopFunc()
		}()

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(b, err)

		accounts := blockExecutor.SetupAccounts(b, privateKeys)

		addrs := []flow.Address{}
		for _, account := range accounts {
			addrs = append(addrs, account.Address)
		}
		evmAddress, err := chain.AddressAtIndex(systemcontracts.EVMStorageAccountIndex)
		require.NoError(b, err)
		addrs = append(addrs, evmAddress)

		// fund all accounts so not to run into storage problems
		fundAccounts(b, blockExecutor, cadence.UFix64(1_000_000_000_000), addrs...)

		accounts[0].DeployContract(b, blockExecutor, "TestContract", `
			access(all) contract TestContract {
				access(all) event SomeEvent()

				access(all) fun empty() {
				}

				access(all) fun emitEvent() {
					emit SomeEvent()
				}
			}
			`)
		require.Equal(b, testContractAddress, accounts[0].Address,
			"test contract should be deployed to first available account index")

		accounts[0].AddArrayToStorage(b, blockExecutor, []string{longString, longString, longString, longString, longString})

		tc := testutils.GetStorageTestContract(b)
		var evmTestAccount *testutils.EOATestAccount
		blockExecutor.RunWithLedger(b, func(ledger atree.Ledger) {
			testutils.DeployContract(b, types.EmptyAddress, tc, ledger, chain.ServiceAddress())
			evmTestAccount = testutils.FundAndGetEOATestAccount(b, ledger, chain.ServiceAddress())
		})

		benchTransactionContext := benchTransactionContext{
			EvmTestContract: tc,
			EvmTestAccount:  evmTestAccount,
		}

		benchmarkAccount := &accounts[0]

		b.ResetTimer() // setup done, lets start measuring
		b.StopTimer()
		for i := 0; i < b.N; i++ {
			transactions := make([]*flow.TransactionBody, transactionsPerBlock)
			for j := 0; j < transactionsPerBlock; j++ {
				tx := txStringFunc(b, benchTransactionContext)

				btx := []byte(tx)
				txBody := flow.NewTransactionBody().
					SetScript(btx).
					AddAuthorizer(benchmarkAccount.Address).
					SetProposalKey(benchmarkAccount.Address, 0, benchmarkAccount.RetAndIncSeqNumber()).
					SetPayer(benchmarkAccount.Address)

				err = testutil.SignEnvelope(txBody, benchmarkAccount.Address, benchmarkAccount.PrivateKey)
				require.NoError(b, err)

				transactions[j] = txBody
			}
			b.StartTimer()
			computationResult := blockExecutor.ExecuteCollections(b, [][]*flow.TransactionBody{transactions})
			b.StopTimer()
			totalInteractionUsed := uint64(0)
			totalComputationUsed := uint64(0)
			results := computationResult.AllTransactionResults()
			// not interested in the system transaction
			for _, txRes := range results[0 : len(results)-1] {
				require.Empty(b, txRes.ErrorMessage)
				totalInteractionUsed += logE.InteractionUsed[txRes.ID().String()]
				totalComputationUsed += txRes.ComputationUsed
			}
			b.ReportMetric(float64(totalInteractionUsed/uint64(transactionsPerBlock)), "interactions")
			b.ReportMetric(float64(totalComputationUsed/uint64(transactionsPerBlock)), "computation")
		}
	}

	templateTx := func(rep int, prepare string) string {
		return fmt.Sprintf(
			`
			import FungibleToken from 0x%s
			import FlowToken from 0x%s
			import TestContract from 0x%s
			import EVM from 0x%s

			transaction(){
				prepare(signer: auth(Storage, Capabilities) &Account){
					var i = 0
					while i < %d {
						i = i + 	1
			%s
					}
				}
			}`,
			sc.FungibleToken.Address.Hex(),
			sc.FlowToken.Address.Hex(),
			testContractAddress,
			sc.EVMContract.Address.Hex(),
			rep,
			prepare,
		)
	}

	b.Run("reference tx", func(b *testing.B) {
		benchTransaction(b,
			func(b *testing.B, context benchTransactionContext) string {
				return templateTx(100, "")
			},
		)
	})
	b.Run("convert int to string", func(b *testing.B) {
		benchTransaction(b,
			func(b *testing.B, context benchTransactionContext) string {
				return templateTx(100, `i.toString()`)
			},
		)
	})
	b.Run("convert int to string and concatenate it", func(b *testing.B) {
		benchTransaction(b,
			func(b *testing.B, context benchTransactionContext) string {
				return templateTx(100, `"x".concat(i.toString())`)
			},
		)
	})
	b.Run("get signer address", func(b *testing.B) {
		benchTransaction(b,
			func(b *testing.B, context benchTransactionContext) string {
				return templateTx(100, `signer.address`)
			},
		)
	})
	b.Run("get public account", func(b *testing.B) {
		benchTransaction(b,
			func(b *testing.B, context benchTransactionContext) string {
				return templateTx(100, `getAccount(signer.address)`)
			},
		)
	})
	b.Run("get account and get balance", func(b *testing.B) {
		benchTransaction(b,
			func(b *testing.B, context benchTransactionContext) string {
				return templateTx(100, `getAccount(signer.address).balance`)
			},
		)
	})
	b.Run("get account and get available balance", func(b *testing.B) {
		benchTransaction(b,
			func(b *testing.B, context benchTransactionContext) string {
				return templateTx(100, `getAccount(signer.address).availableBalance`)
			},
		)
	})
	b.Run("get account and get storage used", func(b *testing.B) {
		benchTransaction(b,
			func(b *testing.B, context benchTransactionContext) string {
				return templateTx(100, `getAccount(signer.address).storage.used`)
			},
		)
	})
	b.Run("get account and get storage capacity", func(b *testing.B) {
		benchTransaction(b,
			func(b *testing.B, context benchTransactionContext) string {
				return templateTx(100, `getAccount(signer.address).storage.capacity`)
			},
		)
	})
	b.Run("get signer vault", func(b *testing.B) {
		benchTransaction(
			b,
			func(b *testing.B, context benchTransactionContext) string {
				return templateTx(100,
					`let vaultRef = signer.storage.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)!`)
			},
		)
	})
	b.Run("get signer receiver", func(b *testing.B) {
		benchTransaction(
			b,
			func(b *testing.B, context benchTransactionContext) string {
				return templateTx(100,
					`let receiverRef =  getAccount(signer.address)
						.capabilities.borrow<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)!`)
			},
		)
	})
	b.Run("transfer tokens", func(b *testing.B) {
		benchTransaction(
			b,
			func(b *testing.B, context benchTransactionContext) string {
				return templateTx(100, `
					let receiverRef =  getAccount(signer.address)
						.capabilities.borrow<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)!

					let vaultRef = signer.storage.borrow<auth(FungibleToken.Withdraw) &FlowToken.Vault>(from: /storage/flowTokenVault)!

					receiverRef.deposit(from: <-vaultRef.withdraw(amount: 0.00001))
				`)
			},
		)
	})
	b.Run("load and save empty string on signers address", func(b *testing.B) {
		benchTransaction(
			b,
			func(b *testing.B, context benchTransactionContext) string {
				return templateTx(100, `
					signer.storage.load<String>(from: /storage/testpath)
					signer.storage.save("", to: /storage/testpath)
				`)
			},
		)
	})
	b.Run("load and save long string on signers address", func(b *testing.B) {
		benchTransaction(
			b,
			func(b *testing.B, context benchTransactionContext) string {
				return templateTx(100, fmt.Sprintf(`
					signer.storage.load<String>(from: /storage/testpath)
					signer.storage.save("%s", to: /storage/testpath)
				`, longString))
			},
		)
	})
	b.Run("create new account", func(b *testing.B) {
		benchTransaction(b,
			func(b *testing.B, context benchTransactionContext) string {
				return templateTx(50, `let acct = Account(payer: signer)`)
			},
		)
	})
	b.Run("call empty contract function", func(b *testing.B) {
		benchTransaction(b,
			func(b *testing.B, context benchTransactionContext) string {
				return templateTx(100, `TestContract.empty()`)
			},
		)
	})
	b.Run("emit event", func(b *testing.B) {
		benchTransaction(b,
			func(b *testing.B, context benchTransactionContext) string {
				return templateTx(100, `TestContract.emitEvent()`)
			},
		)
	})
	b.Run("borrow array from storage", func(b *testing.B) {
		benchTransaction(
			b,
			func(b *testing.B, context benchTransactionContext) string {
				return templateTx(100, `
					let strings = signer.storage.borrow<&[String]>(from: /storage/test)!
					var i = 0
					while (i < strings.length) {
					  log(strings[i])
					  i = i +1
					}
				`)
			},
		)
	})
	b.Run("copy array from storage", func(b *testing.B) {
		benchTransaction(
			b,
			func(b *testing.B, context benchTransactionContext) string {
				return templateTx(100, `
					let strings = signer.storage.copy<[String]>(from: /storage/test)!
					var i = 0
					while (i < strings.length) {
					  log(strings[i])
					  i = i +1
					}
				`)
			},
		)
	})

	benchEvm := func(b *testing.B, control bool) {
		// This is the same as the evm benchmark but without the EVM.run call
		// This way we can observe the cost of just the EVM.run call
		benchTransaction(
			b,
			func(b *testing.B, context benchTransactionContext) string {
				coinbaseBytes := context.EvmTestAccount.Address().Bytes()
				transactionBody := fmt.Sprintf(`
				                    let coinbaseBytesRaw = "%s".decodeHex()
					let coinbaseBytes: [UInt8; 20] = [0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0]
					for j, v in coinbaseBytesRaw {
						coinbaseBytes[j] = v
					}
					let coinbase = EVM.EVMAddress(bytes: coinbaseBytes)
				`, hex.EncodeToString(coinbaseBytes))

				num := int64(12)
				gasLimit := uint64(100_000)

				// add 10 EVM transactions to the Flow transaction body
				for i := 0; i < 100; i++ {
					txBytes := context.EvmTestAccount.PrepareSignAndEncodeTx(b,
						context.EvmTestContract.DeployedAt.ToCommon(),
						context.EvmTestContract.MakeCallData(b, "store", big.NewInt(num)),
						big.NewInt(0),
						gasLimit,
						big.NewInt(0),
					)
					if control {
						transactionBody += fmt.Sprintf(`
						let txBytes%[1]d = "%[2]s".decodeHex()
						EVM.run(tx: txBytes%[1]d, coinbase: coinbase)
						`,
							i,
							hex.EncodeToString(txBytes),
						)
					} else {
						// don't run the EVM transaction but do the hex conversion
						transactionBody += fmt.Sprintf(`
						let txBytes%[1]d = "%[2]s".decodeHex()
						//EVM.run(tx: txBytes%[1]d, coinbase: coinbase)
						`,
							i,
							hex.EncodeToString(txBytes),
						)
					}

				}

				return templateTx(1, transactionBody)
			},
		)
	}

	b.Run("evm", func(b *testing.B) {
		benchEvm(b, false)
	})

	b.Run("evm control", func(b *testing.B) {
		benchEvm(b, true)
	})

}

const TransferTxTemplate = `
		import NonFungibleToken from 0x%s
		import BatchNFT from 0x%s

		transaction(testTokenIDs: [UInt64], recipientAddress: Address) {
			let transferTokens: @NonFungibleToken.Collection

			prepare(acct: auth(BorrowValue) &Account) {
				let ref = acct.storage.borrow<&BatchNFT.Collection>(from: /storage/TestTokenCollection)!
				self.transferTokens <- ref.batchWithdraw(ids: testTokenIDs)
			}

			execute {
				// get the recipient's public account object
				let recipient = getAccount(recipientAddress)
				// get the Collection reference for the receiver
				let receiverRef = recipient.capabilities.borrow<&{BatchNFT.TestTokenCollectionPublic}>(/public/TestTokenCollection)!
				// deposit the NFT in the receivers collection
				receiverRef.batchDeposit(tokens: <-self.transferTokens)
			}
		}`

// BenchmarkRuntimeNFTBatchTransfer runs BenchRunNFTBatchTransfer with BasicBlockExecutor
func BenchmarkRuntimeNFTBatchTransfer(b *testing.B) {
	blockExecutor := NewBasicBlockExecutor(b, flow.Testnet.Chain(), zerolog.Nop())
	defer func() {
		blockExecutor.onStopFunc()
	}()

	// Create an account private key.
	privateKeys, err := testutil.GenerateAccountPrivateKeys(3)
	require.NoError(b, err)

	// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
	accounts := blockExecutor.SetupAccounts(b, privateKeys)

	BenchRunNFTBatchTransfer(b, blockExecutor, accounts)
}

// BenchRunNFTBatchTransfer simulates executing blocks with `transactionsPerBlock`
// where each transaction transfers `testTokensPerTransaction` testTokens (NFTs)
func BenchRunNFTBatchTransfer(b *testing.B,
	blockExecutor TestBenchBlockExecutor,
	accounts []TestBenchAccount) {

	transactionsPerBlock := 10
	testTokensPerTransaction := 10

	serviceAccount := blockExecutor.ServiceAccount(b)
	// deploy NFT
	nftAccount := accounts[0]
	deployNFT(b, blockExecutor, &nftAccount)

	// deploy NFT
	batchNFTAccount := accounts[1]
	deployBatchNFT(b, blockExecutor, &batchNFTAccount, nftAccount.Address)

	// fund all accounts so not to run into storage problems
	fundAccounts(b, blockExecutor, cadence.UFix64(10_0000_0000), nftAccount.Address, batchNFTAccount.Address, accounts[2].Address)

	// mint nfts into the batchNFT account
	mintNFTs(b, blockExecutor, &accounts[1], transactionsPerBlock*testTokensPerTransaction*b.N)

	// set up receiver
	setupReceiver(b, blockExecutor, &nftAccount, &batchNFTAccount, &accounts[2])

	// Transfer NFTs
	transferTx := []byte(fmt.Sprintf(TransferTxTemplate, accounts[0].Address.Hex(), accounts[1].Address.Hex()))

	encodedAddress, err := jsoncdc.Encode(cadence.Address(accounts[2].Address))
	require.NoError(b, err)

	var computationResult *execution.ComputationResult

	b.ResetTimer() // setup done, lets start measuring
	for i := 0; i < b.N; i++ {
		transactions := make([]*flow.TransactionBody, transactionsPerBlock)
		for j := 0; j < transactionsPerBlock; j++ {
			cadenceValues := make([]cadence.Value, testTokensPerTransaction)
			startTestToken := (i*transactionsPerBlock+j)*testTokensPerTransaction + 1
			for m := 0; m < testTokensPerTransaction; m++ {
				cadenceValues[m] = cadence.NewUInt64(uint64(startTestToken + m))
			}

			encodedArg, err := jsoncdc.Encode(
				cadence.NewArray(cadenceValues),
			)
			require.NoError(b, err)

			txBody := flow.NewTransactionBody().
				SetScript(transferTx).
				SetProposalKey(serviceAccount.Address, 0, serviceAccount.RetAndIncSeqNumber()).
				AddAuthorizer(accounts[1].Address).
				AddArgument(encodedArg).
				AddArgument(encodedAddress).
				SetPayer(serviceAccount.Address)

			err = testutil.SignPayload(txBody, accounts[1].Address, accounts[1].PrivateKey)
			require.NoError(b, err)

			err = testutil.SignEnvelope(txBody, serviceAccount.Address, serviceAccount.PrivateKey)
			require.NoError(b, err)

			transactions[j] = txBody
		}

		computationResult = blockExecutor.ExecuteCollections(b, [][]*flow.TransactionBody{transactions})
		results := computationResult.AllTransactionResults()
		// not interested in the system transaction
		for _, txRes := range results[0 : len(results)-1] {
			require.Empty(b, txRes.ErrorMessage)
		}
	}
}

func setupReceiver(b *testing.B, be TestBenchBlockExecutor, nftAccount, batchNFTAccount, targetAccount *TestBenchAccount) {
	serviceAccount := be.ServiceAccount(b)

	setUpReceiverTemplate := `
	import NonFungibleToken from 0x%s
	import BatchNFT from 0x%s
	
	transaction {
		prepare(signer: auth(SaveValue) &Account) {
			signer.save(
				<-BatchNFT.createEmptyCollection(),
				to: /storage/TestTokenCollection
			)
		}
	}`

	setupTx := []byte(fmt.Sprintf(setUpReceiverTemplate, nftAccount.Address.Hex(), batchNFTAccount.Address.Hex()))

	txBody := flow.NewTransactionBody().
		SetScript(setupTx).
		SetProposalKey(serviceAccount.Address, 0, serviceAccount.RetAndIncSeqNumber()).
		AddAuthorizer(targetAccount.Address).
		SetPayer(serviceAccount.Address)

	err := testutil.SignPayload(txBody, targetAccount.Address, targetAccount.PrivateKey)
	require.NoError(b, err)

	err = testutil.SignEnvelope(txBody, serviceAccount.Address, serviceAccount.PrivateKey)
	require.NoError(b, err)

	computationResult := be.ExecuteCollections(b, [][]*flow.TransactionBody{{txBody}})
	require.Empty(b, computationResult.AllTransactionResults()[0].ErrorMessage)
}

func mintNFTs(b *testing.B, be TestBenchBlockExecutor, batchNFTAccount *TestBenchAccount, size int) {
	serviceAccount := be.ServiceAccount(b)
	mintScriptTemplate := `
	import BatchNFT from 0x%s
	transaction {
		prepare(signer: auth(BorrowValue) &Account) {
			let adminRef = signer.storage.borrow<&BatchNFT.Admin>(from: /storage/BatchNFTAdmin)!
			let playID = adminRef.createPlay(metadata: {"name": "Test"})
			let setID = BatchNFT.nextSetID
			adminRef.createSet(name: "Test")
			let setRef = adminRef.borrowSet(setID: setID)
			setRef.addPlay(playID: playID)
			let testTokens <- setRef.batchMintTestToken(playID: playID, quantity: %d)
			signer.storage.borrow<&BatchNFT.Collection>(from: /storage/TestTokenCollection)!
				.batchDeposit(tokens: <-testTokens)
		}
	}`
	mintScript := []byte(fmt.Sprintf(mintScriptTemplate, batchNFTAccount.Address.Hex(), size))

	txBody := flow.NewTransactionBody().
		SetComputeLimit(999999).
		SetScript(mintScript).
		SetProposalKey(serviceAccount.Address, 0, serviceAccount.RetAndIncSeqNumber()).
		AddAuthorizer(batchNFTAccount.Address).
		SetPayer(serviceAccount.Address)

	err := testutil.SignPayload(txBody, batchNFTAccount.Address, batchNFTAccount.PrivateKey)
	require.NoError(b, err)

	err = testutil.SignEnvelope(txBody, serviceAccount.Address, serviceAccount.PrivateKey)
	require.NoError(b, err)

	computationResult := be.ExecuteCollections(b, [][]*flow.TransactionBody{{txBody}})
	require.Empty(b, computationResult.AllTransactionResults()[0].ErrorMessage)
}

func fundAccounts(b *testing.B, be TestBenchBlockExecutor, value cadence.UFix64, accounts ...flow.Address) {
	serviceAccount := be.ServiceAccount(b)
	for _, a := range accounts {
		txBody := transferTokensTx(be.Chain(b))
		txBody.SetProposalKey(serviceAccount.Address, 0, serviceAccount.RetAndIncSeqNumber())
		txBody.AddArgument(jsoncdc.MustEncode(value))
		txBody.AddArgument(jsoncdc.MustEncode(cadence.Address(a)))
		txBody.AddAuthorizer(serviceAccount.Address)
		txBody.SetPayer(serviceAccount.Address)

		err := testutil.SignEnvelope(txBody, serviceAccount.Address, serviceAccount.PrivateKey)
		require.NoError(b, err)

		computationResult := be.ExecuteCollections(b, [][]*flow.TransactionBody{{txBody}})
		require.Empty(b, computationResult.AllTransactionResults()[0].ErrorMessage)
	}
}

func deployBatchNFT(b *testing.B, be TestBenchBlockExecutor, owner *TestBenchAccount, nftAddress flow.Address) {
	batchNFTContract := func(nftAddress flow.Address) string {
		return fmt.Sprintf(`
			import NonFungibleToken from 0x%s

			access(all) contract BatchNFT: NonFungibleToken {
				access(all) event ContractInitialized()
				access(all) event PlayCreated(id: UInt32, metadata: {String:String})
				access(all) event NewSeriesStarted(newCurrentSeries: UInt32)
				access(all) event SetCreated(setID: UInt32, series: UInt32)
				access(all) event PlayAddedToSet(setID: UInt32, playID: UInt32)
				access(all) event PlayRetiredFromSet(setID: UInt32, playID: UInt32, numTestTokens: UInt32)
				access(all) event SetLocked(setID: UInt32)
				access(all) event TestTokenMinted(testTokenID: UInt64, playID: UInt32, setID: UInt32, serialNumber: UInt32)
				access(all) event Withdraw(id: UInt64, from: Address?)
				access(all) event Deposit(id: UInt64, to: Address?)
				access(all) var currentSeries: UInt32
				access(self) var playDatas: {UInt32: Play}
				access(self) var setDatas: {UInt32: SetData}
				access(self) var sets: @{UInt32: Set}
				access(all) var nextPlayID: UInt32
				access(all) var nextSetID: UInt32
				access(all) var totalSupply: UInt64

				access(all) struct Play {
					access(all) let playID: UInt32
					access(all) let metadata: {String: String}

					init(metadata: {String: String}) {
						pre {
							metadata.length != 0: "New Play Metadata cannot be empty"
						}
						self.playID = BatchNFT.nextPlayID
						self.metadata = metadata

						BatchNFT.nextPlayID = BatchNFT.nextPlayID + UInt32(1)
						emit PlayCreated(id: self.playID, metadata: metadata)
					}
				}

				access(all) struct SetData {
					access(all) let setID: UInt32
					access(all) let name: String
					access(all) let series: UInt32
					init(name: String) {
						pre {
							name.length > 0: "New Set name cannot be empty"
						}
						self.setID = BatchNFT.nextSetID
						self.name = name
						self.series = BatchNFT.currentSeries
						BatchNFT.nextSetID = BatchNFT.nextSetID + UInt32(1)
						emit SetCreated(setID: self.setID, series: self.series)
					}
				}

				access(all) resource Set {
					access(all) let setID: UInt32
					access(all) var plays: [UInt32]
					access(all) var retired: {UInt32: Bool}
					access(all) var locked: Bool
					access(all) var numberMintedPerPlay: {UInt32: UInt32}

					init(name: String) {
						self.setID = BatchNFT.nextSetID
						self.plays = []
						self.retired = {}
						self.locked = false
						self.numberMintedPerPlay = {}

						BatchNFT.setDatas[self.setID] = SetData(name: name)
					}

					access(all) fun addPlay(playID: UInt32) {
						pre {
							BatchNFT.playDatas[playID] != nil: "Cannot add the Play to Set: Play doesn't exist"
							!self.locked: "Cannot add the play to the Set after the set has been locked"
							self.numberMintedPerPlay[playID] == nil: "The play has already beed added to the set"
						}

						self.plays.append(playID)
						self.retired[playID] = false
						self.numberMintedPerPlay[playID] = 0
						emit PlayAddedToSet(setID: self.setID, playID: playID)
					}

					access(all) fun addPlays(playIDs: [UInt32]) {
						for play in playIDs {
							self.addPlay(playID: play)
						}
					}

					access(all) fun retirePlay(playID: UInt32) {
						pre {
							self.retired[playID] != nil: "Cannot retire the Play: Play doesn't exist in this set!"
						}

						if !self.retired[playID]! {
							self.retired[playID] = true

							emit PlayRetiredFromSet(setID: self.setID, playID: playID, numTestTokens: self.numberMintedPerPlay[playID]!)
						}
					}

					access(all) fun retireAll() {
						for play in self.plays {
							self.retirePlay(playID: play)
						}
					}

					access(all) fun lock() {
						if !self.locked {
							self.locked = true
							emit SetLocked(setID: self.setID)
						}
					}

					access(all) fun mintTestToken(playID: UInt32): @NFT {
						pre {
							self.retired[playID] != nil: "Cannot mint the testToken: This play doesn't exist"
							!self.retired[playID]!: "Cannot mint the testToken from this play: This play has been retired"
						}
						let numInPlay = self.numberMintedPerPlay[playID]!
						let newTestToken: @NFT <- create NFT(serialNumber: numInPlay + UInt32(1),
														playID: playID,
														setID: self.setID)

						self.numberMintedPerPlay[playID] = numInPlay + UInt32(1)

						return <-newTestToken
					}

					access(all) fun batchMintTestToken(playID: UInt32, quantity: UInt64): @Collection {
						let newCollection <- create Collection()

						var i: UInt64 = 0
						while i < quantity {
							newCollection.deposit(token: <-self.mintTestToken(playID: playID))
							i = i + UInt64(1)
						}

						return <-newCollection
					}
				}

				access(all) struct TestTokenData {
					access(all) let setID: UInt32
					access(all) let playID: UInt32
					access(all) let serialNumber: UInt32

					init(setID: UInt32, playID: UInt32, serialNumber: UInt32) {
						self.setID = setID
						self.playID = playID
						self.serialNumber = serialNumber
					}

				}

				access(all) resource NFT: NonFungibleToken.INFT {
					access(all) let id: UInt64
					access(all) let data: TestTokenData

					init(serialNumber: UInt32, playID: UInt32, setID: UInt32) {
						BatchNFT.totalSupply = BatchNFT.totalSupply + UInt64(1)

						self.id = BatchNFT.totalSupply

						self.data = TestTokenData(setID: setID, playID: playID, serialNumber: serialNumber)

						emit TestTokenMinted(testTokenID: self.id, playID: playID, setID: self.data.setID, serialNumber: self.data.serialNumber)
					}
				}

				access(all) resource Admin {
					access(all) fun createPlay(metadata: {String: String}): UInt32 {
						var newPlay = Play(metadata: metadata)
						let newID = newPlay.playID

						BatchNFT.playDatas[newID] = newPlay

						return newID
					}

					access(all) fun createSet(name: String) {
						var newSet <- create Set(name: name)

						BatchNFT.sets[newSet.setID] <-! newSet
					}

					access(all) fun borrowSet(setID: UInt32): &Set {
						pre {
							BatchNFT.sets[setID] != nil: "Cannot borrow Set: The Set doesn't exist"
						}
						return (&BatchNFT.sets[setID] as &Set?)!
					}

					access(all) fun startNewSeries(): UInt32 {
						BatchNFT.currentSeries = BatchNFT.currentSeries + UInt32(1)

						emit NewSeriesStarted(newCurrentSeries: BatchNFT.currentSeries)

						return BatchNFT.currentSeries
					}

					access(all) fun createNewAdmin(): @Admin {
						return <-create Admin()
					}
				}

				access(all) resource interface TestTokenCollectionPublic {
					access(all) fun deposit(token: @NonFungibleToken.NFT)
					access(all) fun batchDeposit(tokens: @NonFungibleToken.Collection)
					access(all) fun getIDs(): [UInt64]
					access(all) fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
					access(all) fun borrowTestToken(id: UInt64): &BatchNFT.NFT? {
						post {
							(result == nil) || (result?.id == id):
								"Cannot borrow TestToken reference: The ID of the returned reference is incorrect"
						}
					}
				}

				access(all) resource Collection: TestTokenCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
					access(all) var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

					init() {
						self.ownedNFTs <- {}
					}

					access(all) fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
						let token <- self.ownedNFTs.remove(key: withdrawID)
							?? panic("Cannot withdraw: TestToken does not exist in the collection")

						emit Withdraw(id: token.id, from: self.owner?.address)

						return <-token
					}

					access(all) fun batchWithdraw(ids: [UInt64]): @NonFungibleToken.Collection {
						var batchCollection <- create Collection()

						for id in ids {
							batchCollection.deposit(token: <-self.withdraw(withdrawID: id))
						}
						return <-batchCollection
					}

					access(all) fun deposit(token: @NonFungibleToken.NFT) {
						let token <- token as! @BatchNFT.NFT

						let id = token.id
						let oldToken <- self.ownedNFTs[id] <- token

						if self.owner?.address != nil {
							emit Deposit(id: id, to: self.owner?.address)
						}

						destroy oldToken
					}

					access(all) fun batchDeposit(tokens: @NonFungibleToken.Collection) {
						let keys = tokens.getIDs()

						for key in keys {
							self.deposit(token: <-tokens.withdraw(withdrawID: key))
						}
						destroy tokens
					}

					access(all) fun getIDs(): [UInt64] {
						return self.ownedNFTs.keys
					}

					access(all) fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
						return (&self.ownedNFTs[id] as &NonFungibleToken.NFT?)!
					}

					access(all) fun borrowTestToken(id: UInt64): &BatchNFT.NFT? {
						if self.ownedNFTs[id] != nil {
							let ref = (&self.ownedNFTs[id] as auth &NonFungibleToken.NFT?)!
							return ref as! &BatchNFT.NFT
						} else {
							return nil
						}
					}
				}

				access(all) fun createEmptyCollection(): @NonFungibleToken.Collection {
					return <-create BatchNFT.Collection()
				}

				access(all) fun getAllPlays(): [BatchNFT.Play] {
					return BatchNFT.playDatas.values
				}

				access(all) fun getPlayMetaData(playID: UInt32): {String: String}? {
					return self.playDatas[playID]?.metadata
				}

				access(all) fun getPlayMetaDataByField(playID: UInt32, field: String): String? {
					if let play = BatchNFT.playDatas[playID] {
						return play.metadata[field]
					} else {
						return nil
					}
				}

				access(all) fun getSetName(setID: UInt32): String? {
					return BatchNFT.setDatas[setID]?.name
				}

				access(all) fun getSetSeries(setID: UInt32): UInt32? {
					return BatchNFT.setDatas[setID]?.series
				}

				access(all) fun getSetIDsByName(setName: String): [UInt32]? {
					var setIDs: [UInt32] = []

					for setData in BatchNFT.setDatas.values {
						if setName == setData.name {
							setIDs.append(setData.setID)
						}
					}

					if setIDs.length == 0 {
						return nil
					} else {
						return setIDs
					}
				}

				access(all) fun getPlaysInSet(setID: UInt32): [UInt32]? {
					return BatchNFT.sets[setID]?.plays
				}

				access(all) fun isEditionRetired(setID: UInt32, playID: UInt32): Bool? {
					if let setToRead <- BatchNFT.sets.remove(key: setID) {
						let retired = setToRead.retired[playID]
						BatchNFT.sets[setID] <-! setToRead
						return retired
					} else {
						return nil
					}
				}

				access(all) fun isSetLocked(setID: UInt32): Bool? {
					return BatchNFT.sets[setID]?.locked
				}

				access(all) fun getNumTestTokensInEdition(setID: UInt32, playID: UInt32): UInt32? {
					if let setToRead <- BatchNFT.sets.remove(key: setID) {
						let amount = setToRead.numberMintedPerPlay[playID]
						BatchNFT.sets[setID] <-! setToRead
						return amount
					} else {
						return nil
					}
				}

				init() {
					self.currentSeries = 0
					self.playDatas = {}
					self.setDatas = {}
					self.sets <- {}
					self.nextPlayID = 1
					self.nextSetID = 1
					self.totalSupply = 0

					self.account.storage.save<@Collection>(<- create Collection(), to: /storage/TestTokenCollection)

					let collectionCap = self.account.capabilities.storage.issue<&{TestTokenCollectionPublic}>(/storage/TestTokenCollection)
					self.account.capabilities.publish(collectionCap, at: /public/TestTokenCollection)

					self.account.storage.save<@Admin>(<- create Admin(), to: /storage/BatchNFTAdmin)
					emit ContractInitialized()
				}
			}
		`, nftAddress.Hex())
	}
	owner.DeployContract(b, be, "BatchNFT", batchNFTContract(nftAddress))
}

func deployNFT(b *testing.B, be TestBenchBlockExecutor, owner *TestBenchAccount) {
	const nftContract = `
		access(all) contract interface NonFungibleToken {
			access(all) var totalSupply: UInt64
			access(all) event ContractInitialized()
			access(all) event Withdraw(id: UInt64, from: Address?)
			access(all) event Deposit(id: UInt64, to: Address?)
			access(all) resource interface INFT {
				access(all) let id: UInt64
			}
			access(all) resource NFT: INFT {
				access(all) let id: UInt64
			}
			access(all) resource interface Provider {
				access(all) fun withdraw(withdrawID: UInt64): @NFT {
					post {
						result.id == withdrawID: "The ID of the withdrawn token must be the same as the requested ID"
					}
				}
			}
			access(all) resource interface Receiver {
				access(all) fun deposit(token: @NFT)
			}
			access(all) resource interface CollectionPublic {
				access(all) fun deposit(token: @NFT)
				access(all) fun getIDs(): [UInt64]
				access(all) fun borrowNFT(id: UInt64): &NFT
			}
			access(all) resource Collection: Provider, Receiver, CollectionPublic {
				access(all) var ownedNFTs: @{UInt64: NFT}
				access(all) fun withdraw(withdrawID: UInt64): @NFT
				access(all) fun deposit(token: @NFT)
				access(all) fun getIDs(): [UInt64]
				access(all) fun borrowNFT(id: UInt64): &NFT {
					pre {
						self.ownedNFTs[id] != nil: "NFT does not exist in the collection!"
					}
				}
			}
			access(all) fun createEmptyCollection(): @Collection {
				post {
					result.getIDs().length == 0: "The created collection must be empty!"
				}
			}
		}`

	owner.DeployContract(b, be, "NonFungibleToken", nftContract)
}
