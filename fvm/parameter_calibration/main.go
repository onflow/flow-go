package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"os"
	"strconv"
	"strings"
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

func init() {
	flag.StringVar(&filenameFlag, "file", "data.csv", "help message for flagname")
	flag.IntVar(&blocksFlag, "blocks", 1000, "help message for flagname")
}

func main() {
	flag.Parse()

	data := runTransactionsAndGetData(blocksFlag)

	f, err := os.Create(filenameFlag)
	defer f.Close()

	if err != nil {

		panic("")
	}

	w := csv.NewWriter(f)
	err = w.WriteAll(data)

	if err != nil {
		panic("")
	}
}

func runTransactionsAndGetData(blocks int) [][]string {
	rand.Seed(time.Now().UnixNano())

	longString := strings.Repeat("0", 100)
	chain := flow.Testnet.Chain()

	logE := &logExtractor{
		TimeSpent:        map[string]uint64{},
		Weights:          map[string]map[string]uint64{},
		Columns:          map[string]struct{}{},
		InteractionUsed:  map[string]uint64{},
		TransactionNames: map[string]string{},
	}

	logger := zerolog.New(logE).Level(zerolog.InfoLevel)

	blockExecutor, err := NewBasicBlockExecutor(chain, logger)
	if err != nil {
		panic(err)
	}
	serviceAccount := blockExecutor.ServiceAccount()

	// Create an account private key.
	privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
	if err != nil {
		panic(err)
	}

	accounts, err := blockExecutor.SetupAccounts(privateKeys)
	if err != nil {
		panic(err)
	}
	err = accounts[0].DeployContract(blockExecutor, "TestContract", `
			access(all) contract TestContract {
				access(all) event SomeEvent()
				access(all) fun empty() {
				}
				access(all) fun emit() {
					emit SomeEvent()
				}
			}
			`)
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
		if i%100 == 0 {
			fmt.Println("progress", i)
		}
		transactions := make([]*flow.TransactionBody, transactionsPerBlock)
		generatedTransactions := make([]GeneratedTransaction, transactionsPerBlock)
		for j := 0; j < transactionsPerBlock; j++ {
			txType := Pool.GetRandomTransactionType()
			gtx, err := txType.GenerateTransaction(ttctx)
			generatedTransactions[j] = gtx
			txBody := gtx.Transaction.
				AddAuthorizer(serviceAccount.Address).
				SetProposalKey(serviceAccount.Address, 0, serviceAccount.RetAndIncSeqNumber()).
				SetPayer(serviceAccount.Address)

			err = testutil.SignEnvelope(txBody, serviceAccount.Address, serviceAccount.PrivateKey)
			if err != nil {
				panic(err)
			}

			transactions[j] = txBody
			logE.TransactionNames[txBody.ID().String()] = txType.Name()
		}

		computationResult, err := blockExecutor.ExecuteCollections([][]*flow.TransactionBody{transactions})
		if err != nil {
			panic(err)
		}
		totalInteractionUsed := uint64(0)
		totalComputationUsed := uint64(0)
		for j := 0; j < transactionsPerBlock; j++ {
			if len(computationResult.TransactionResults[j].ErrorMessage) > 0 {
				fmt.Println(generatedTransactions[j].Type.Name())
				panic(computationResult.TransactionResults[j].ErrorMessage)
			}
			generatedTransactions[j].AdjustParameterRange(logE.TimeSpent[computationResult.TransactionResults[j].ID().String()])
			totalInteractionUsed += logE.InteractionUsed[computationResult.TransactionResults[j].ID().String()]
			totalComputationUsed += computationResult.TransactionResults[j].ComputationUsed
		}
	}

	var data [][]string
	columns := []string{"tx"}
	for s := range logE.Columns {
		columns = append(columns, s)
	}
	columns = append(columns, "ms")

	data = append(data, columns)

	for s, u := range logE.TimeSpent {
		cdata := make([]string, len(columns))
		cdata[0] = logE.TransactionNames[s]
		for i := 1; i < len(columns)-1; i++ {
			cdata[i] = strconv.FormatUint(logE.Weights[s][columns[i]], 10)
		}
		cdata[len(columns)-1] = strconv.FormatUint(u, 10)
		data = append(data, cdata)
	}
	return data
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
		fvm.WithMaxStateInteractionSize(200_000_000),
		fvm.WithComputationLimit(20_000_000),
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
	executableBlock := unittest.ExecutableBlockFromTransactions(collections)
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
					data, err := jsoncdc.Decode(event.Payload)
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

type logExtractor struct {
	TimeSpent        map[string]uint64
	Weights          map[string]map[string]uint64
	Columns          map[string]struct{}
	InteractionUsed  map[string]uint64
	TransactionNames map[string]string
}

type txWeights struct {
	TXHash        string
	TimeSpentInMS uint64
	Weights       map[string]uint64
}

type txSuccessfulLog struct {
	TXHash                string `json:"txHash"`
	LedgerInteractionUsed uint64 `json:"ledgerInteractionUsed"`
}

func (l *logExtractor) Write(p []byte) (n int, err error) {
	if strings.Contains(string(p), "weights") {
		w := txWeights{}
		err := json.Unmarshal(p, &w)

		if err != nil {
			fmt.Println(err)
			return len(p), nil
		}

		l.TimeSpent[w.TXHash] = w.TimeSpentInMS
		l.Weights[w.TXHash] = w.Weights
		for s := range w.Weights {
			l.Columns[s] = struct{}{}
		}

	}
	if strings.Contains(string(p), "transaction executed successfully") {
		w := txSuccessfulLog{}
		err := json.Unmarshal(p, &w)

		if err != nil {
			fmt.Println(err)
			return len(p), nil
		}
		l.InteractionUsed[w.TXHash] = w.LedgerInteractionUsed
	}
	return len(p), nil

}

var _ io.Writer = &logExtractor{}
