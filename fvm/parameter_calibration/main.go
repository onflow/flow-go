package main

import (
	"context"
	"encoding/csv"
	"encoding/hex"
	"flag"
	"fmt"
	gethCommon "github.com/ethereum/go-ethereum/common"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	reusableRuntime "github.com/onflow/flow-go/fvm/runtime"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/computation/computer"
	exeState "github.com/onflow/flow-go/engine/execution/state"
	bootstrapexec "github.com/onflow/flow-go/engine/execution/state/bootstrap"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/systemcontracts"
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
				for n, c := range Generator.controllers {
					fmt.Println(n, c.slope)
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
	allColumnsConverted := []string{"tx"}
	for s := range columns {
		allColumns = append(allColumns, s)
		// TODO: fix for memory
		allColumnsConverted = append(allColumnsConverted, computationIntensityName(s))
	}
	allColumns = append(allColumns, "estimated", "actual")
	allColumnsConverted = append(allColumnsConverted, "estimated", "ms")

	data = append(data, allColumnsConverted)

	for s, u := range actual {
		cdata := make([]string, len(allColumns))
		name, ok := transactionNames[s]
		if !ok {
			continue
		}
		cdata[0] = name
		for i := 1; i < len(allColumns)-2; i++ {
			cdata[i] = strconv.FormatUint(intensities[s][allColumns[i]], 10)
		}
		cdata[len(allColumns)-2] = strconv.FormatUint(uint64(estimated[s]), 10)
		cdata[len(allColumns)-1] = strconv.FormatUint(u, 10)
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

	longString := strings.Repeat("0", 100)
	chain := flow.Testnet.Chain()

	sc := systemcontracts.SystemContractsForChain(chain.ChainID())

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

	addressBytes, err := hex.DecodeString("3da9cb19b06A645BA6F12F79D8d7fcdd8A00cfD4")
	if err != nil {
		panic(err)
	}
	evmAddress := gethCommon.BytesToAddress(addressBytes)
	evmAddressPrivateKey, err := gethcrypto.HexToECDSA("e43eb57b2f3be8009ea545059e171dc4fdf543ae97220a76c0684706357f7f39")
	if err != nil {
		panic(err)
	}

	err = serviceAccount.FundEVMAccount(blockExecutor, evmAddress)
	if err != nil {
		panic(err)
	}

	testContractAddress, err := chain.AddressAtIndex(systemcontracts.EVMAccountIndex + 1)

	ttctx := &TransactionGenerationContext{
		AddressReplacements: map[string]string{
			"0xFUNGIBLETOKEN": sc.FungibleToken.Address.HexWithPrefix(),
			"0xFLOWTOKEN":     sc.FlowToken.Address.HexWithPrefix(),
			"0xTESTCONTRACT":  testContractAddress.HexWithPrefix(),
			"0xEVM":           sc.FlowServiceAccount.Address.HexWithPrefix(),
		},
		ETHAddressNonce:      0,
		EthAddressPrivateKey: evmAddressPrivateKey,
	}

	// random transactions per block
	transactionsPerBlock := rand.Intn(50) + 1

	for i := 0; i < blocks; i++ {

		transactions := make([]*flow.TransactionBody, transactionsPerBlock)
		generatedTransactions := make([]GeneratedTransaction, transactionsPerBlock)
		for j := 0; j < transactionsPerBlock; j++ {
			gtx, err := Generator.Generate(ttctx)
			if err != nil {
				panic(err)
			}
			generatedTransactions[j] = gtx
			txBody := gtx.Transaction.
				AddAuthorizer(serviceAccount.Address).
				SetProposalKey(serviceAccount.Address, 0, serviceAccount.RetAndIncSeqNumber()).
				SetPayer(accounts[1].Address).
				SetGasLimit(1000000000)

			err = testutil.SignPayload(txBody, serviceAccount.Address, serviceAccount.PrivateKey)
			if err != nil {
				panic(err)
			}

			err = testutil.SignEnvelope(txBody, accounts[1].Address, accounts[1].PrivateKey)
			if err != nil {
				panic(err)
			}

			transactions[j] = txBody
			dc.TransactionNames[txBody.ID().String()] = string(gtx.Name)
		}

		computationResult, err := blockExecutor.ExecuteCollections([][]*flow.TransactionBody{transactions})
		if err != nil {
			panic(err)
		}

		for j := 0; j < transactionsPerBlock; j++ {
			if len(computationResult.AllTransactionResults()[j].ErrorMessage) > 0 {
				fmt.Println(generatedTransactions[j].Name)
				panic(computationResult.AllTransactionResults()[j].ErrorMessage)
			}
			generatedTransactions[j].AdjustParametersCallback(dc.TimeSpent[computationResult.AllTransactionResults()[j].ID().String()])
		}
	}

	return dc
}

type TestBenchBlockExecutor interface {
	ExecuteCollections(collections [][]*flow.TransactionBody) (*execution.ComputationResult, error)
	Chain() flow.Chain
	ServiceAccount() *TestBenchAccount
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

	if len(computationResult.AllTransactionResults()[0].ErrorMessage) > 0 {
		return fmt.Errorf(computationResult.AllTransactionResults()[0].ErrorMessage)
	}
	return nil
}

func (account *TestBenchAccount) MintTokens(blockExec TestBenchBlockExecutor, tokens uint64) (err error) {
	sc := systemcontracts.SystemContractsForChain(blockExec.Chain().ChainID())
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
		`, sc.FungibleToken.Address, sc.FlowToken.Address))).
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
	if len(computationResult.AllTransactionResults()[0].ErrorMessage) > 0 {
		return fmt.Errorf(computationResult.AllTransactionResults()[0].ErrorMessage)
	}
	return nil
}

func (account *TestBenchAccount) FundEVMAccount(blockExec TestBenchBlockExecutor, evmAddress gethCommon.Address) error {
	chain := blockExec.Chain()
	serviceAccount := blockExec.ServiceAccount()

	sc := systemcontracts.SystemContractsForChain(chain.ChainID())

	addressCadenceBytes := make([]cadence.Value, 20)
	for i := range addressCadenceBytes {
		addressCadenceBytes[i] = cadence.UInt8(evmAddress[i])
	}
	addressArg, err := jsoncdc.Encode(cadence.NewArray(addressCadenceBytes).WithType(stdlib.EVMAddressBytesCadenceType))
	if err != nil {
		return err
	}

	// a million FLOW
	amount, err := cadence.NewUFix64("1000000.0")
	if err != nil {
		return err
	}
	amountArg, err := jsoncdc.Encode(amount)
	if err != nil {
		return err
	}

	// Fund evm address
	txBody := flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(
			`
						import EVM from %s
						import FungibleToken from %s
						import FlowToken from %s

						transaction(address: [UInt8; 20], amount: UFix64) {
							let tokenAdmin: &FlowToken.Administrator

							prepare(signer: AuthAccount) {
								self.tokenAdmin = signer
									.borrow<&FlowToken.Administrator>(from: /storage/flowTokenAdmin)
									?? panic("Signer is not the token admin")
							}

							execute {
								let minter <- self.tokenAdmin.createNewMinter(allowedAmount: amount)
								let mintedVault <- minter.mintTokens(amount: amount)

								let fundAddress = EVM.EVMAddress(bytes: address)
								fundAddress.deposit(from: <-mintedVault)

								destroy minter
							}
						}
					`,
			sc.FlowServiceAccount.Address.HexWithPrefix(),
			sc.FungibleToken.Address.HexWithPrefix(),
			sc.FlowToken.Address.HexWithPrefix(),
		))).
		SetProposalKey(serviceAccount.Address, 0, serviceAccount.RetAndIncSeqNumber()).
		SetPayer(serviceAccount.Address).
		AddAuthorizer(serviceAccount.Address).
		AddArgument(addressArg).
		AddArgument(amountArg)

	err = testutil.SignEnvelope(txBody, serviceAccount.Address, serviceAccount.PrivateKey)
	if err != nil {
		return err
	}

	computationResult, err := blockExec.ExecuteCollections([][]*flow.TransactionBody{{txBody}})
	if err != nil {
		return err
	}
	if len(computationResult.AllTransactionResults()[0].ErrorMessage) > 0 {
		return fmt.Errorf(computationResult.AllTransactionResults()[0].ErrorMessage)
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
	if len(computationResult.AllTransactionResults()[0].ErrorMessage) > 0 {
		return fmt.Errorf(computationResult.AllTransactionResults()[0].ErrorMessage)
	}
	return nil
}

// BasicBlockExecutor executes blocks in sequence and applies all changes (not fork aware)
type BasicBlockExecutor struct {
	blockComputer         computer.BlockComputer
	derivedChainData      *derived.DerivedChainData
	activeSnapshot        snapshot.SnapshotTree
	activeStateCommitment flow.StateCommitment
	chain                 flow.Chain
	serviceAccount        *TestBenchAccount
}

func NewBasicBlockExecutor(chain flow.Chain, logger zerolog.Logger) (*BasicBlockExecutor, error) {
	vm := fvm.NewVirtualMachine()

	opts := []fvm.Option{
		fvm.WithTransactionFeesEnabled(true),
		fvm.WithAccountStorageLimit(true),
		fvm.WithMaxStateInteractionSize(2_000_000_000),
		fvm.WithEventCollectionSizeLimit(math.MaxUint64),
		fvm.WithGasLimit(20_000_000),
		fvm.WithChain(chain),
		fvm.WithLogger(logger),
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
	if err != nil {
		return nil, err
	}

	compactor := fixtures.NewNoopCompactor(ledger)
	<-compactor.Ready()

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
		fvm.WithSetupEVMEnabled(true),
	)
	if err != nil {
		return nil, err
	}

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
		1,
	)
	if err != nil {
		return nil, err
	}

	activeSnapshot := snapshot.NewSnapshotTree(
		exeState.NewLedgerStorageSnapshot(ledger, initialCommit))

	derivedChainData, err := derived.NewDerivedChainData(
		derived.DefaultDerivedDataCacheSize)
	if err != nil {
		return nil, err
	}

	return &BasicBlockExecutor{
		blockComputer:         blockComputer,
		derivedChainData:      derivedChainData,
		activeStateCommitment: initialCommit,
		activeSnapshot:        activeSnapshot,
		chain:                 chain,
		serviceAccount:        serviceAccount,
	}, nil
}

func (b *BasicBlockExecutor) Chain() flow.Chain {
	return b.chain
}

func (b *BasicBlockExecutor) ServiceAccount() *TestBenchAccount {
	return b.serviceAccount
}

func (b *BasicBlockExecutor) ExecuteCollections(collections [][]*flow.TransactionBody) (*execution.ComputationResult, error) {
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
	if err != nil {
		return nil, err
	}

	b.activeStateCommitment = computationResult.CurrentEndState()

	for _, snapshot := range computationResult.AllExecutionSnapshots() {
		b.activeSnapshot = b.activeSnapshot.Append(snapshot)
	}

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
		if len(computationResult.AllTransactionResults()[0].ErrorMessage) > 0 {
			return nil, fmt.Errorf(computationResult.AllTransactionResults()[0].ErrorMessage)
		}

		var addr flow.Address

		for _, event := range computationResult.AllEvents() {
			if event.Type == flow.EventAccountCreated {
				data, err := ccf.Decode(nil, event.Payload)
				if err != nil {
					panic("setup account failed, error decoding events")
				}
				addr = flow.Address(data.(cadence.Event).Fields[0].(cadence.Address))
				break
			}

		}
		if addr == flow.EmptyAddress {
			panic("setup account failed, no account creation event emitted")
		}
		accounts = append(accounts, TestBenchAccount{Address: addr, PrivateKey: privateKey, SeqNumber: 0})
	}

	return accounts, nil
}
