package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/computation/computer"
	exeState "github.com/onflow/flow-go/engine/execution/state"
	bootstrapexec "github.com/onflow/flow-go/engine/execution/state/bootstrap"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	completeLedger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

var filenameFlag string
var blocksFlag int
var workersFlag int
var desiredMaxTimeFlag float64

func init() {
	flag.StringVar(&filenameFlag, "file", "data", "Output data file prefix")
	flag.IntVar(&blocksFlag, "blocks", 200, "Total blocks to go through")
	flag.IntVar(&workersFlag, "workers", 2, "Number of concurrent threads")
	flag.Float64Var(&desiredMaxTimeFlag, "max_time", 500., "Desired max time per tx")
}

// run with something like `go run --tags relic ./fvm/parameter_calibration --file ./fvm/parameter_calibration/data1 --blocks 100`
func main() {
	flag.Parse()

	desiredMaxTime = desiredMaxTimeFlag

	numWorkers := workersFlag
	workersWG := &sync.WaitGroup{}
	collectorsWG := &sync.WaitGroup{}
	dataChannel := make(chan *transactionDataCollector, numWorkers)
	rootDataCollector := newTransactionDataCollector()

	runsPerWorker := blocksFlag / (10 * numWorkers)
	run := 0
	blocksPerRun := 10
	collectorsWG.Add(1)
	go func() {
		for dc := range dataChannel {
			rootDataCollector.Merge(dc)

			run++
			fmt.Println("progress: ", run, "/", runsPerWorker*numWorkers)
			if run%numWorkers == 0 {
				for _, tt := range Pool.Pool {
					if stt, ok := tt.(*SimpleTxType); ok {
						fmt.Println(stt.name, stt.slope)
					}
				}
			}
		}
		collectorsWG.Done()
	}()

	for i := 0; i < numWorkers; i++ {
		workersWG.Add(1)
		go func() {
			for j := 0; j < runsPerWorker; j++ {
				dataChannel <- runTransactionsAndGetData(blocksPerRun)
			}
			workersWG.Done()
		}()
	}

	workersWG.Wait()
	close(dataChannel)
	collectorsWG.Wait()

	// output collected data to file
	outputDataToFile(filenameFlag+"_c.csv",
		rootDataCollector.ComputationColumns,
		rootDataCollector.ComputationUsed,
		rootDataCollector.TimeSpent,
		rootDataCollector.TransactionNames,
		rootDataCollector.ComputationIntensities)

	outputDataToFile(filenameFlag+"_m.csv",
		rootDataCollector.MemoryColumns,
		rootDataCollector.MemoryUsed,
		rootDataCollector.MemAlloc,
		rootDataCollector.TransactionNames,
		rootDataCollector.MemoryIntensities)
}

func outputDataToFile(filename string, columns map[string]struct{}, estimated map[string]uint, actual map[string]uint64, transactionNames map[string]string, intensities map[string]map[string]uint64) {
	// output collected data to file
	var data [][]string
	allColumns := []string{"tx"}
	for s := range columns {
		allColumns = append(allColumns, s)
	}
	allColumns = append(allColumns, "estimated", "actual")

	data = append(data, allColumns)

	for s, u := range actual {
		cdata := make([]string, len(allColumns))
		cdata[0] = transactionNames[s]
		for i := 1; i < len(allColumns)-2; i++ {
			cdata[i] = strconv.FormatUint(intensities[s][allColumns[i]], 10)
		}
		cdata[len(allColumns)-2] = strconv.FormatUint(uint64(estimated[s]), 10)
		cdata[len(allColumns)-1] = strconv.FormatUint(uint64(u), 10)
		data = append(data, cdata)
	}

	f, err := os.Create(filename)
	defer func() {
		_ = f.Close()
	}()

	if err != nil {
		panic("")
	}
	w := csv.NewWriter(f)
	err = w.WriteAll(data)
	if err != nil {
		panic("")
	}
}

func runTransactionsAndGetData(blocks int) *transactionDataCollector {
	rand.Seed(time.Now().UnixNano())

	longString := strings.Repeat("0", 100)
	chain := flow.Testnet.Chain()

	dc := newTransactionDataCollector()

	logger := zerolog.New(dc).Level(zerolog.DebugLevel)

	blockExecutor, err := NewBasicBlockExecutor(chain, logger)
	if err != nil {
		panic(err)
	}
	serviceAccount := blockExecutor.ServiceAccount()

	// Create an account private key.
	privateKeys, err := testutil.GenerateAccountPrivateKeys(2)
	if err != nil {
		panic(err)
	}

	accounts, err := blockExecutor.SetupAccounts(privateKeys)
	if err != nil {
		panic(err)
	}
	err = accounts[0].DeployContract(blockExecutor, "TestContract", `
			access(all) contract TestContract {
				pub var totalSupply: UInt64
				pub var nfts: @[NFT]

				access(all) event SomeEvent()
				access(all) fun empty() {
				}
				access(all) fun emit() {
					emit SomeEvent()
				}

				access(all) fun mintNFT() {
					var newNFT <- create NFT(
						id: TestContract.totalSupply,
						data: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
					)
					self.nfts.append( <- newNFT)

					TestContract.totalSupply = TestContract.totalSupply + UInt64(1)
				}

				pub resource NFT {
					pub let id: UInt64
					pub let data: String
			
					init(
						id: UInt64,
						data: String,
					) {
						self.id = id
						self.data = data
					}
				}

				init() {
					self.totalSupply = 0
					self.nfts <- []
				}
			}
			`)
	if err != nil {
		panic(err)
	}

	err = accounts[0].MintTokens(blockExecutor, 100_000_000_0000_0000) //100M FLOW
	if err != nil {
		panic(err)
	}

	err = accounts[1].MintTokens(blockExecutor, 100_000_000_0000_0000) //100M FLOW
	if err != nil {
		panic(err)
	}

	err = serviceAccount.AddArrayToStorage(blockExecutor, []string{longString, longString, longString, longString, longString})
	if err != nil {
		panic(err)
	}
	ttctx := TransactionTypeContext{
		AddressReplacements: map[string]string{
			"FUNGIBLETOKEN": fvm.FungibleTokenAddress(chain).Hex(),
			"FLOWTOKEN":     fvm.FlowTokenAddress(chain).Hex(),
			"TESTCONTRACT":  "754aed9de6197641",
		},
	}

	// random transactions per block
	transactionsPerBlock := rand.Intn(50) + 1

	for i := 0; i < blocks; i++ {
		transactions := make([]*flow.TransactionBody, transactionsPerBlock)
		generatedTransactions := make([]GeneratedTransaction, transactionsPerBlock)
		for j := 0; j < transactionsPerBlock; j++ {
			txType := Pool.GetRandomTransactionType()
			gtx, err := txType.GenerateTransaction(ttctx)
			if err != nil {
				panic(err)
			}
			generatedTransactions[j] = gtx
			txBody := gtx.Transaction.
				AddAuthorizer(serviceAccount.Address).
				SetProposalKey(serviceAccount.Address, 0, serviceAccount.RetAndIncSeqNumber()).
				SetPayer(accounts[1].Address).
				SetGasLimit(100000000)

			err = testutil.SignPayload(txBody, serviceAccount.Address, serviceAccount.PrivateKey)
			if err != nil {
				panic(err)
			}

			err = testutil.SignEnvelope(txBody, accounts[1].Address, accounts[1].PrivateKey)
			if err != nil {
				panic(err)
			}

			transactions[j] = txBody
			dc.TransactionNames[txBody.ID().String()] = txType.Name()
		}

		computationResult, err := blockExecutor.ExecuteCollections([][]*flow.TransactionBody{transactions})
		if err != nil {
			panic(err)
		}

		for j := 0; j < transactionsPerBlock; j++ {
			if len(computationResult.TransactionResults[j].ErrorMessage) > 0 {
				fmt.Println(generatedTransactions[j].Type.Name())
				panic(computationResult.TransactionResults[j].ErrorMessage)
			}
			generatedTransactions[j].AdjustParameterRange(dc.TimeSpent[computationResult.TransactionResults[j].ID().String()])
		}
	}

	return dc
}

type TestBenchBlockExecutor interface {
	ExecuteCollections(collections [][]*flow.TransactionBody) (*execution.ComputationResult, error)
	Chain() flow.Chain
	ServiceAccount() *TestBenchAccount
	ResetProgramCache()
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

func (account *TestBenchAccount) DeployContract(blockExec TestBenchBlockExecutor, contractName string, contract string) error {
	serviceAccount := blockExec.ServiceAccount()
	txBody := testutil.CreateContractDeploymentTransaction(
		contractName,
		contract,
		account.Address,
		blockExec.Chain())

	txBody.SetProposalKey(serviceAccount.Address, 0, serviceAccount.RetAndIncSeqNumber())
	txBody.SetPayer(serviceAccount.Address)

	err := testutil.SignPayload(txBody, account.Address, account.PrivateKey)
	if err != nil {
		return err
	}

	err = testutil.SignEnvelope(txBody, serviceAccount.Address, serviceAccount.PrivateKey)
	if err != nil {
		return err
	}

	computationResult, err := blockExec.ExecuteCollections([][]*flow.TransactionBody{{txBody}})
	if err != nil {
		return err
	}

	if len(computationResult.TransactionResults[0].ErrorMessage) > 0 {
		return fmt.Errorf(computationResult.TransactionResults[0].ErrorMessage)
	}
	return nil
}

func (account *TestBenchAccount) MintTokens(blockExec TestBenchBlockExecutor, tokens uint64) (err error) {
	serviceAccount := blockExec.ServiceAccount()
	txBody := flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(`
			import FungibleToken from 0x%s
			import FlowToken from 0x%s
			
			transaction(recipient: Address, amount: UFix64) {
				let tokenAdmin: &FlowToken.Administrator
				let tokenReceiver: &{FungibleToken.Receiver}
			
				prepare(signer: AuthAccount) {
					self.tokenAdmin = signer
						.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)
						?? panic("Signer is not the token admin")
			
					self.tokenReceiver = getAccount(recipient)
						.getCapability(/public/flowTokenReceiver)
						.borrow<&{FungibleToken.Receiver}>()
						?? panic("Unable to borrow receiver reference")
				}
			
				execute {
					let minter <- self.tokenAdmin.createNewMinter(allowedAmount: amount)
					let mintedVault <- minter.mintTokens(amount: amount)
			
					self.tokenReceiver.deposit(from: <-mintedVault)
			
					destroy minter
				}
			}
		`, fvm.FungibleTokenAddress(blockExec.Chain()), fvm.FlowTokenAddress(blockExec.Chain())))).
		AddAuthorizer(blockExec.ServiceAccount().Address)

	txBody.AddArgument(jsoncdc.MustEncode(cadence.BytesToAddress(account.Address.Bytes())))
	txBody.AddArgument(jsoncdc.MustEncode(cadence.UFix64(tokens)))

	txBody.SetProposalKey(serviceAccount.Address, 0, serviceAccount.RetAndIncSeqNumber())
	txBody.SetPayer(serviceAccount.Address)

	if account.Address != serviceAccount.Address {
		err = testutil.SignPayload(txBody, account.Address, account.PrivateKey)
		if err != nil {
			return err
		}
	}

	err = testutil.SignEnvelope(txBody, serviceAccount.Address, serviceAccount.PrivateKey)
	if err != nil {
		return err
	}

	computationResult, err := blockExec.ExecuteCollections([][]*flow.TransactionBody{{txBody}})
	if err != nil {
		return err
	}
	if len(computationResult.TransactionResults[0].ErrorMessage) > 0 {
		return fmt.Errorf(computationResult.TransactionResults[0].ErrorMessage)
	}
	return nil
}

func (account *TestBenchAccount) AddArrayToStorage(blockExec TestBenchBlockExecutor, list []string) error {
	serviceAccount := blockExec.ServiceAccount()
	txBody := flow.NewTransactionBody().
		SetScript([]byte(`
		transaction(list: [String]) {
		  prepare(acct: AuthAccount) {
			acct.load<[String]>(from: /storage/test)
			acct.save(list, to: /storage/test)
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
	if err != nil {
		return err
	}
	txBody.AddArgument(cadenceArray)

	txBody.SetProposalKey(serviceAccount.Address, 0, serviceAccount.RetAndIncSeqNumber())
	txBody.SetPayer(serviceAccount.Address)

	if account.Address != serviceAccount.Address {
		err = testutil.SignPayload(txBody, account.Address, account.PrivateKey)
		if err != nil {
			return err
		}
	}

	err = testutil.SignEnvelope(txBody, serviceAccount.Address, serviceAccount.PrivateKey)
	if err != nil {
		return err
	}

	computationResult, err := blockExec.ExecuteCollections([][]*flow.TransactionBody{{txBody}})
	if err != nil {
		return err
	}
	if len(computationResult.TransactionResults[0].ErrorMessage) > 0 {
		return fmt.Errorf(computationResult.TransactionResults[0].ErrorMessage)
	}
	return nil
}

// BasicBlockExecutor executes blocks in sequence and applies all changes (not fork aware)
type BasicBlockExecutor struct {
	blockComputer         computer.BlockComputer
	programCache          *programs.Programs
	activeView            state.View
	activeStateCommitment flow.StateCommitment
	chain                 flow.Chain
	serviceAccount        *TestBenchAccount
}

func NewBasicBlockExecutor(chain flow.Chain, logger zerolog.Logger) (*BasicBlockExecutor, error) {
	rt := fvm.NewInterpreterRuntime()
	vm := fvm.NewVirtualMachine(rt)

	opts := []fvm.Option{
		fvm.WithTransactionFeesEnabled(true),
		fvm.WithAccountStorageLimit(true),
		fvm.WithMaxStateInteractionSize(2_000_000_000),
		fvm.WithEventCollectionSizeLimit(math.MaxUint64),
		fvm.WithGasLimit(20_000_000),
		fvm.WithChain(chain),
	}
	fvmContext := fvm.NewContext(logger, opts...)

	collector := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()

	wal := &fixtures.NoopWAL{}

	ledger, err := completeLedger.NewLedger(wal, 100, collector, logger, completeLedger.DefaultPathFinderVersion)
	if err != nil {
		return nil, err
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
	if err != nil {
		return nil, err
	}

	ledgerCommitter := committer.NewLedgerViewCommitter(ledger, tracer)
	blockComputer, err := computer.NewBlockComputer(vm, fvmContext, collector, tracer, logger, ledgerCommitter)
	if err != nil {
		return nil, err
	}

	view := delta.NewView(exeState.LedgerGetRegister(ledger, initialCommit))

	return &BasicBlockExecutor{
		blockComputer:         blockComputer,
		programCache:          programs.NewEmptyPrograms(),
		activeStateCommitment: initialCommit,
		activeView:            view,
		chain:                 chain,
		serviceAccount:        serviceAccount,
	}, nil
}

func (b *BasicBlockExecutor) Chain() flow.Chain {
	return b.chain
}

func (b *BasicBlockExecutor) ResetProgramCache() {
	b.programCache = programs.NewEmptyPrograms()
}

func (b *BasicBlockExecutor) ServiceAccount() *TestBenchAccount {
	return b.serviceAccount
}

func (b *BasicBlockExecutor) ExecuteCollections(collections [][]*flow.TransactionBody) (*execution.ComputationResult, error) {
	executableBlock := unittest.ExecutableBlockFromTransactions(b.chain.ChainID(), collections)
	executableBlock.StartState = &b.activeStateCommitment

	computationResult, err := b.blockComputer.ExecuteBlock(context.Background(), executableBlock, b.activeView, b.programCache)
	if err != nil {
		return nil, err
	}

	endState, _, _, err := execution.GenerateExecutionResultAndChunkDataPacks(unittest.IdentifierFixture(), b.activeStateCommitment, computationResult)
	if err != nil {
		return nil, err
	}
	b.activeStateCommitment = endState

	return computationResult, nil
}

func (b *BasicBlockExecutor) SetupAccounts(privateKeys []flow.AccountPrivateKey) ([]TestBenchAccount, error) {
	accounts := make([]TestBenchAccount, 0)
	serviceAddress := b.Chain().ServiceAddress()

	accontCreationScript := `
		transaction(publicKey: [UInt8]) {
			prepare(signer: AuthAccount) {
				let acct = AuthAccount(payer: signer)
				acct.addPublicKey(publicKey)
			}
		}
	`

	for _, privateKey := range privateKeys {
		accountKey := privateKey.PublicKey(fvm.AccountKeyWeightThreshold)
		encAccountKey, _ := flow.EncodeRuntimeAccountPublicKey(accountKey)
		cadAccountKey := testutil.BytesToCadenceArray(encAccountKey)
		encCadAccountKey, _ := jsoncdc.Encode(cadAccountKey)

		txBody := flow.NewTransactionBody().
			SetScript([]byte(accontCreationScript)).
			AddArgument(encCadAccountKey).
			AddAuthorizer(serviceAddress).
			SetProposalKey(serviceAddress, 0, b.ServiceAccount().RetAndIncSeqNumber()).
			SetPayer(serviceAddress)

		err := testutil.SignEnvelope(txBody, b.Chain().ServiceAddress(), unittest.ServiceAccountPrivateKey)
		if err != nil {
			return nil, err
		}

		computationResult, err := b.ExecuteCollections([][]*flow.TransactionBody{{txBody}})
		if err != nil {
			return nil, err
		}
		if len(computationResult.TransactionResults[0].ErrorMessage) > 0 {
			return nil, fmt.Errorf(computationResult.TransactionResults[0].ErrorMessage)
		}

		var addr flow.Address

		for _, eventList := range computationResult.Events {
			for _, event := range eventList {
				if event.Type == flow.EventAccountCreated {
					data, err := jsoncdc.Decode(nil, event.Payload)
					if err != nil {
						panic("setup account failed, error decoding events")
					}
					addr = flow.Address(data.(cadence.Event).Fields[0].(cadence.Address))
					break
				}
			}
		}
		if addr == flow.EmptyAddress {
			panic("setup account failed, no account creation event emitted")
		}
		accounts = append(accounts, TestBenchAccount{Address: addr, PrivateKey: privateKey, SeqNumber: 0})
	}

	return accounts, nil
}

// transactionDataCollector collects data from a transaction execution logs.
type transactionDataCollector struct {
	TimeSpent              map[string]uint64
	MemAlloc               map[string]uint64
	ComputationIntensities map[string]map[string]uint64
	MemoryIntensities      map[string]map[string]uint64
	ComputationColumns     map[string]struct{}
	MemoryColumns          map[string]struct{}
	ComputationUsed        map[string]uint
	MemoryUsed             map[string]uint
	TransactionNames       map[string]string
}

type txWeights struct {
	TXHash                 string            `json:"txHash"`
	LedgerInteractionUsed  uint64            `json:"ledgerInteractionUsed"`
	ComputationUsed        uint              `json:"computationUsed"`
	MemoryUsed             uint              `json:"memoryUsed"`
	ComputationIntensities map[string]uint64 `json:"computationIntensities"`
	MemoryIntensities      map[string]uint64 `json:"memoryIntensities"`
}

type txDoneLog struct {
	TXHash        string `json:"tx_id"`
	TimeSpentInMS uint64 `json:"timeSpentInMS"`
	MemAlloc      uint64 `json:"memAlloc"`
}

func newTransactionDataCollector() *transactionDataCollector {
	return &transactionDataCollector{
		TimeSpent:              map[string]uint64{},
		MemAlloc:               map[string]uint64{},
		ComputationIntensities: map[string]map[string]uint64{},
		MemoryIntensities:      map[string]map[string]uint64{},
		ComputationColumns:     map[string]struct{}{},
		MemoryColumns:          map[string]struct{}{},
		ComputationUsed:        map[string]uint{},
		MemoryUsed:             map[string]uint{},
		TransactionNames:       map[string]string{},
	}
}

func (l *transactionDataCollector) Write(p []byte) (n int, err error) {
	if strings.Contains(string(p), "transaction execution data") {
		w := txWeights{}
		err := json.Unmarshal(p, &w)

		if err != nil {
			fmt.Println(err)
			return len(p), nil
		}

		l.ComputationIntensities[w.TXHash] = w.ComputationIntensities
		l.MemoryIntensities[w.TXHash] = w.MemoryIntensities
		l.ComputationUsed[w.TXHash] = w.ComputationUsed
		l.MemoryUsed[w.TXHash] = w.MemoryUsed

		for s := range w.MemoryIntensities {
			l.MemoryColumns[s] = struct{}{}
		}
		for s := range w.ComputationIntensities {
			l.ComputationColumns[s] = struct{}{}
		}

	}
	if strings.Contains(string(p), "transaction executed successfully") || strings.Contains(string(p), "transaction executed failed") {
		w := txDoneLog{}
		err := json.Unmarshal(p, &w)

		if err != nil {
			fmt.Println(err)
			return len(p), nil
		}

		l.MemAlloc[w.TXHash] = w.MemAlloc
		l.TimeSpent[w.TXHash] = w.TimeSpentInMS
	}
	return len(p), nil

}

// Merge merges the data from the other collector into this one.
func (l *transactionDataCollector) Merge(l2 *transactionDataCollector) {
	for k, v := range l2.TimeSpent {
		l.TimeSpent[k] = v
	}
	for k, v := range l2.MemAlloc {
		l.MemAlloc[k] = v
	}
	for k, v := range l2.ComputationIntensities {
		l.ComputationIntensities[k] = v
	}
	for k, v := range l2.MemoryIntensities {
		l.MemoryIntensities[k] = v
	}
	for k, v := range l2.ComputationColumns {
		l.ComputationColumns[k] = v
	}
	for k, v := range l2.MemoryColumns {
		l.MemoryColumns[k] = v
	}
	for k, v := range l2.ComputationUsed {
		l.ComputationUsed[k] = v
	}
	for k, v := range l2.MemoryUsed {
		l.MemoryUsed[k] = v
	}
	for k, v := range l2.TransactionNames {
		l.TransactionNames[k] = v
	}
}

var _ io.Writer = &transactionDataCollector{}
