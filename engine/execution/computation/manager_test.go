package computation

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/computation/computer"
	"github.com/onflow/flow-go/engine/execution/computation/query"
	state2 "github.com/onflow/flow-go/engine/execution/state"
	unittest2 "github.com/onflow/flow-go/engine/execution/state/unittest"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/environment"
	fvmErrors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/provider"
	mocktracker "github.com/onflow/flow-go/module/executiondatasync/tracker/mock"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	requesterunit "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

var scriptLogThreshold = 1 * time.Second

func TestComputeBlockWithStorage(t *testing.T) {
	chain := flow.Mainnet.Chain()

	vm := fvm.NewVirtualMachine()
	execCtx := fvm.NewContext(fvm.WithChain(chain))

	privateKeys, err := testutil.GenerateAccountPrivateKeys(2)
	require.NoError(t, err)

	snapshotTree, accounts, err := testutil.CreateAccounts(
		vm,
		testutil.RootBootstrappedLedger(vm, execCtx),
		privateKeys,
		chain)
	require.NoError(t, err)

	tx1 := testutil.DeployCounterContractTransaction(accounts[0], chain)
	tx1.SetProposalKey(chain.ServiceAddress(), 0, 0).
		SetComputeLimit(1000).
		SetPayer(chain.ServiceAddress())

	err = testutil.SignPayload(tx1, accounts[0], privateKeys[0])
	require.NoError(t, err)

	err = testutil.SignEnvelope(tx1, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	tx2 := testutil.CreateCounterTransaction(accounts[0], accounts[1])
	tx2.SetProposalKey(chain.ServiceAddress(), 0, 0).
		SetComputeLimit(1000).
		SetPayer(chain.ServiceAddress())

	err = testutil.SignPayload(tx2, accounts[1], privateKeys[1])
	require.NoError(t, err)

	err = testutil.SignEnvelope(tx2, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	transactions := []*flow.TransactionBody{tx1, tx2}

	col := flow.Collection{Transactions: transactions}

	guarantee := flow.CollectionGuarantee{
		CollectionID: col.ID(),
		Signature:    nil,
	}

	block := flow.Block{
		Header: &flow.Header{
			View: 42,
		},
		Payload: &flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{&guarantee},
		},
	}

	executableBlock := &entity.ExecutableBlock{
		Block: &block,
		CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{
			guarantee.ID(): {
				Guarantee:    &guarantee,
				Transactions: transactions,
			},
		},
		StartState: unittest.StateCommitmentPointerFixture(),
	}

	me := new(module.Local)
	me.On("NodeID").Return(unittest.IdentifierFixture())
	me.On("Sign", mock.Anything, mock.Anything).Return(nil, nil)
	me.On("SignFunc", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)

	bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	trackerStorage := mocktracker.NewMockStorage()

	prov := provider.NewProvider(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		execution_data.DefaultSerializer,
		bservice,
		trackerStorage,
	)

	blockComputer, err := computer.NewBlockComputer(
		vm,
		execCtx,
		metrics.NewNoopCollector(),
		trace.NewNoopTracer(),
		zerolog.Nop(),
		committer.NewNoopViewCommitter(),
		me,
		prov,
		nil,
		testutil.ProtocolStateWithSourceFixture(nil),
		testMaxConcurrency)
	require.NoError(t, err)

	derivedChainData, err := derived.NewDerivedChainData(10)
	require.NoError(t, err)

	engine := &Manager{
		blockComputer:    blockComputer,
		derivedChainData: derivedChainData,
	}

	returnedComputationResult, err := engine.ComputeBlock(
		context.Background(),
		unittest.IdentifierFixture(),
		executableBlock,
		snapshotTree)
	require.NoError(t, err)

	hasUpdates := false
	for _, snapshot := range returnedComputationResult.AllExecutionSnapshots() {
		if len(snapshot.WriteSet) > 0 {
			hasUpdates = true
			break
		}
	}
	require.True(t, hasUpdates)
	require.Equal(t, returnedComputationResult.BlockExecutionResult.Size(), 1+1) // 1 coll + 1 system chunk
	assert.NotEmpty(t, returnedComputationResult.AllExecutionSnapshots()[0].UpdatedRegisters())
}

func TestComputeBlock_Uploader(t *testing.T) {

	noopCollector := &metrics.NoopCollector{}

	ledger, err := complete.NewLedger(&fixtures.NoopWAL{}, 10, noopCollector, zerolog.Nop(), complete.DefaultPathFinderVersion)
	require.NoError(t, err)

	compactor := fixtures.NewNoopCompactor(ledger)
	<-compactor.Ready()
	defer func() {
		<-ledger.Done()
		<-compactor.Done()
	}()

	me := new(module.Local)
	me.On("NodeID").Return(unittest.IdentifierFixture())
	me.On("Sign", mock.Anything, mock.Anything).Return(nil, nil)
	me.On("SignFunc", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)

	computationResult := unittest2.ComputationResultFixture(
		t,
		unittest.IdentifierFixture(),
		[][]flow.Identifier{
			{unittest.IdentifierFixture()},
			{unittest.IdentifierFixture()},
		})

	blockComputer := &FakeBlockComputer{
		computationResult: computationResult,
	}

	derivedChainData, err := derived.NewDerivedChainData(10)
	require.NoError(t, err)

	manager := &Manager{
		blockComputer:    blockComputer,
		derivedChainData: derivedChainData,
	}

	_, err = manager.ComputeBlock(
		context.Background(),
		unittest.IdentifierFixture(),
		computationResult.ExecutableBlock,
		state2.NewLedgerStorageSnapshot(
			ledger,
			flow.StateCommitment(ledger.InitialState())))
	require.NoError(t, err)
}

func TestExecuteScript(t *testing.T) {

	logger := zerolog.Nop()

	execCtx := fvm.NewContext(fvm.WithLogger(logger))

	me := new(module.Local)
	me.On("NodeID").Return(unittest.IdentifierFixture())
	me.On("Sign", mock.Anything, mock.Anything).Return(nil, nil)
	me.On("SignFunc", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)

	vm := fvm.NewVirtualMachine()

	ledger := testutil.RootBootstrappedLedger(vm, execCtx, fvm.WithExecutionMemoryLimit(math.MaxUint64))

	sc := systemcontracts.SystemContractsForChain(execCtx.Chain.ChainID())

	script := []byte(fmt.Sprintf(
		`
			import FungibleToken from %s

			access(all) fun main() {}
		`,
		sc.FungibleToken.Address.HexWithPrefix(),
	))

	bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	trackerStorage := mocktracker.NewMockStorage()

	prov := provider.NewProvider(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		execution_data.DefaultSerializer,
		bservice,
		trackerStorage,
	)

	engine, err := New(logger,
		metrics.NewNoopCollector(),
		trace.NewNoopTracer(),
		me,
		testutil.ProtocolStateWithSourceFixture(nil),
		execCtx,
		committer.NewNoopViewCommitter(),
		prov,
		ComputationConfig{
			QueryConfig:          query.NewDefaultConfig(),
			DerivedDataCacheSize: derived.DefaultDerivedDataCacheSize,
			MaxConcurrency:       1,
		},
	)
	require.NoError(t, err)

	header := unittest.BlockHeaderFixture()
	_, err = engine.ExecuteScript(
		context.Background(),
		script,
		nil,
		header,
		ledger)
	require.NoError(t, err)
}

// Balance script used to swallow errors, which meant that even if the view was empty, a script that did nothing but get
// the balance of an account would succeed and return 0.
func TestExecuteScript_BalanceScriptFailsIfViewIsEmpty(t *testing.T) {

	logger := zerolog.Nop()

	execCtx := fvm.NewContext(fvm.WithLogger(logger))

	me := new(module.Local)
	me.On("NodeID").Return(unittest.IdentifierFixture())
	me.On("Sign", mock.Anything, mock.Anything).Return(nil, nil)
	me.On("SignFunc", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)

	snapshot := snapshot.NewReadFuncStorageSnapshot(
		func(id flow.RegisterID) (flow.RegisterValue, error) {
			return nil, fmt.Errorf("error getting register")
		})

	sc := systemcontracts.SystemContractsForChain(execCtx.Chain.ChainID())

	script := []byte(fmt.Sprintf(
		`
			access(all) fun main(): UFix64 {
				return getAccount(%s).balance
			}
		`,
		sc.FungibleToken.Address.HexWithPrefix(),
	))

	bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	trackerStorage := mocktracker.NewMockStorage()

	prov := provider.NewProvider(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		execution_data.DefaultSerializer,
		bservice,
		trackerStorage,
	)

	engine, err := New(logger,
		metrics.NewNoopCollector(),
		trace.NewNoopTracer(),
		me,
		testutil.ProtocolStateWithSourceFixture(nil),
		execCtx,
		committer.NewNoopViewCommitter(),
		prov,
		ComputationConfig{
			QueryConfig:          query.NewDefaultConfig(),
			DerivedDataCacheSize: derived.DefaultDerivedDataCacheSize,
			MaxConcurrency:       1,
		},
	)
	require.NoError(t, err)

	header := unittest.BlockHeaderFixture()
	_, err = engine.ExecuteScript(
		context.Background(),
		script,
		nil,
		header,
		snapshot)
	require.ErrorContains(t, err, "error getting register")
}

func TestExecuteScripPanicsAreHandled(t *testing.T) {

	ctx := fvm.NewContext()

	buffer := &bytes.Buffer{}
	log := zerolog.New(buffer)

	header := unittest.BlockHeaderFixture()

	bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	trackerStorage := mocktracker.NewMockStorage()

	prov := provider.NewProvider(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		execution_data.DefaultSerializer,
		bservice,
		trackerStorage,
	)

	manager, err := New(log,
		metrics.NewNoopCollector(),
		trace.NewNoopTracer(),
		nil,
		testutil.ProtocolStateWithSourceFixture(nil),
		ctx,
		committer.NewNoopViewCommitter(),
		prov,
		ComputationConfig{
			QueryConfig:          query.NewDefaultConfig(),
			DerivedDataCacheSize: derived.DefaultDerivedDataCacheSize,
			MaxConcurrency:       1,
			NewCustomVirtualMachine: func() fvm.VM {
				return &PanickingVM{}
			},
		},
	)
	require.NoError(t, err)

	_, err = manager.ExecuteScript(
		context.Background(),
		[]byte("whatever"),
		nil,
		header,
		nil)

	require.Error(t, err)

	require.Contains(t, buffer.String(), "Verunsicherung")
}

func TestExecuteScript_LongScriptsAreLogged(t *testing.T) {

	ctx := fvm.NewContext()

	buffer := &bytes.Buffer{}
	log := zerolog.New(buffer)

	header := unittest.BlockHeaderFixture()

	bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	trackerStorage := mocktracker.NewMockStorage()

	prov := provider.NewProvider(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		execution_data.DefaultSerializer,
		bservice,
		trackerStorage,
	)

	manager, err := New(log,
		metrics.NewNoopCollector(),
		trace.NewNoopTracer(),
		nil,
		testutil.ProtocolStateWithSourceFixture(nil),
		ctx,
		committer.NewNoopViewCommitter(),
		prov,
		ComputationConfig{
			QueryConfig: query.QueryConfig{
				LogTimeThreshold:   1 * time.Millisecond,
				ExecutionTimeLimit: query.DefaultExecutionTimeLimit,
			},
			DerivedDataCacheSize: 10,
			MaxConcurrency:       1,
			NewCustomVirtualMachine: func() fvm.VM {
				return &LongRunningVM{duration: 2 * time.Millisecond}
			},
		},
	)
	require.NoError(t, err)

	_, err = manager.ExecuteScript(
		context.Background(),
		[]byte("whatever"),
		nil,
		header,
		nil)

	require.NoError(t, err)

	require.Contains(t, buffer.String(), "exceeded threshold")
}

func TestExecuteScript_ShortScriptsAreNotLogged(t *testing.T) {

	ctx := fvm.NewContext()

	buffer := &bytes.Buffer{}
	log := zerolog.New(buffer)

	header := unittest.BlockHeaderFixture()

	bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	trackerStorage := mocktracker.NewMockStorage()

	prov := provider.NewProvider(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		execution_data.DefaultSerializer,
		bservice,
		trackerStorage,
	)

	manager, err := New(log,
		metrics.NewNoopCollector(),
		trace.NewNoopTracer(),
		nil,
		testutil.ProtocolStateWithSourceFixture(nil),
		ctx,
		committer.NewNoopViewCommitter(),
		prov,
		ComputationConfig{
			QueryConfig: query.QueryConfig{
				LogTimeThreshold:   1 * time.Second,
				ExecutionTimeLimit: query.DefaultExecutionTimeLimit,
			},
			DerivedDataCacheSize: derived.DefaultDerivedDataCacheSize,
			MaxConcurrency:       1,
			NewCustomVirtualMachine: func() fvm.VM {
				return &LongRunningVM{duration: 0}
			},
		},
	)
	require.NoError(t, err)

	_, err = manager.ExecuteScript(
		context.Background(),
		[]byte("whatever"),
		nil,
		header,
		nil)

	require.NoError(t, err)

	require.NotContains(t, buffer.String(), "exceeded threshold")
}

type PanickingExecutor struct{}

func (PanickingExecutor) Cleanup() {}

func (PanickingExecutor) Preprocess() error {
	return nil
}

func (PanickingExecutor) Execute() error {
	panic("panic, but expected with sentinel for test: Verunsicherung ")
}

func (PanickingExecutor) Output() fvm.ProcedureOutput {
	return fvm.ProcedureOutput{}
}

type PanickingVM struct{}

func (p *PanickingVM) NewExecutor(
	f fvm.Context,
	procedure fvm.Procedure,
	txn storage.TransactionPreparer,
) fvm.ProcedureExecutor {
	return PanickingExecutor{}
}

func (p *PanickingVM) Run(
	f fvm.Context,
	procedure fvm.Procedure,
	storageSnapshot snapshot.StorageSnapshot,
) (
	*snapshot.ExecutionSnapshot,
	fvm.ProcedureOutput,
	error,
) {
	panic("panic, but expected with sentinel for test: Verunsicherung ")
}

func (p *PanickingVM) GetAccount(
	ctx fvm.Context,
	address flow.Address,
	storageSnapshot snapshot.StorageSnapshot,
) (
	*flow.Account,
	error,
) {
	panic("not expected")
}

type LongRunningExecutor struct {
	duration time.Duration
}

func (LongRunningExecutor) Cleanup() {}

func (LongRunningExecutor) Preprocess() error {
	return nil
}

func (l LongRunningExecutor) Execute() error {
	time.Sleep(l.duration)
	return nil
}

func (LongRunningExecutor) Output() fvm.ProcedureOutput {
	return fvm.ProcedureOutput{
		Value: cadence.NewVoid(),
	}
}

type LongRunningVM struct {
	duration time.Duration
}

func (l *LongRunningVM) NewExecutor(
	f fvm.Context,
	procedure fvm.Procedure,
	txn storage.TransactionPreparer,
) fvm.ProcedureExecutor {
	return LongRunningExecutor{
		duration: l.duration,
	}
}

func (l *LongRunningVM) Run(
	f fvm.Context,
	procedure fvm.Procedure,
	storageSnapshot snapshot.StorageSnapshot,
) (
	*snapshot.ExecutionSnapshot,
	fvm.ProcedureOutput,
	error,
) {
	time.Sleep(l.duration)

	snapshot := &snapshot.ExecutionSnapshot{}
	output := fvm.ProcedureOutput{
		Value: cadence.NewVoid(),
	}
	return snapshot, output, nil
}

func (l *LongRunningVM) GetAccount(
	ctx fvm.Context,
	address flow.Address,
	storageSnapshot snapshot.StorageSnapshot,
) (
	*flow.Account,
	error,
) {
	panic("not expected")
}

type FakeBlockComputer struct {
	computationResult *execution.ComputationResult
}

func (f *FakeBlockComputer) ExecuteBlock(
	context.Context,
	flow.Identifier,
	*entity.ExecutableBlock,
	snapshot.StorageSnapshot,
	*derived.DerivedBlockData,
) (
	*execution.ComputationResult,
	error,
) {
	return f.computationResult, nil
}

func TestExecuteScriptTimeout(t *testing.T) {

	timeout := 1 * time.Millisecond
	manager, err := New(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		trace.NewNoopTracer(),
		nil,
		testutil.ProtocolStateWithSourceFixture(nil),
		fvm.NewContext(),
		committer.NewNoopViewCommitter(),
		nil,
		ComputationConfig{
			QueryConfig: query.QueryConfig{
				LogTimeThreshold:   query.DefaultLogTimeThreshold,
				ExecutionTimeLimit: timeout,
			},
			DerivedDataCacheSize: derived.DefaultDerivedDataCacheSize,
			MaxConcurrency:       1,
		},
	)

	require.NoError(t, err)

	script := []byte(`
	access(all) fun main(): Int {
		var i = 0
		while i < 10000 {
			i = i + 1
		}
		return i
	}
	`)

	header := unittest.BlockHeaderFixture()
	value, err := manager.ExecuteScript(
		context.Background(),
		script,
		nil,
		header,
		nil)

	require.Error(t, err)
	require.Nil(t, value)
	require.Contains(t, err.Error(), fvmErrors.ErrCodeScriptExecutionTimedOutError.String())
}

func TestExecuteScriptCancelled(t *testing.T) {

	timeout := 30 * time.Second
	manager, err := New(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		trace.NewNoopTracer(),
		nil,
		testutil.ProtocolStateWithSourceFixture(nil),
		fvm.NewContext(),
		committer.NewNoopViewCommitter(),
		nil,
		ComputationConfig{
			QueryConfig: query.QueryConfig{
				LogTimeThreshold:   query.DefaultLogTimeThreshold,
				ExecutionTimeLimit: timeout,
			},
			DerivedDataCacheSize: derived.DefaultDerivedDataCacheSize,
			MaxConcurrency:       1,
		},
	)

	require.NoError(t, err)

	script := []byte(`
	access(all) fun main(): Int {
		var i = 0
		var j = 0 
		while i < 10000000 {
			i = i + 1
			j = i + j
		}
		return i
	}
	`)

	var value []byte
	var wg sync.WaitGroup
	reqCtx, cancel := context.WithCancel(context.Background())
	wg.Add(1)
	go func() {
		header := unittest.BlockHeaderFixture()
		value, err = manager.ExecuteScript(
			reqCtx,
			script,
			nil,
			header,
			nil)
		wg.Done()
	}()
	cancel()
	wg.Wait()
	require.Nil(t, value)
	require.Contains(t, err.Error(), fvmErrors.ErrCodeScriptExecutionCancelledError.String())
}

func Test_EventEncodingFailsOnlyTxAndCarriesOn(t *testing.T) {

	chain := flow.Mainnet.Chain()
	vm := fvm.NewVirtualMachine()

	eventEncoder := &testingEventEncoder{
		realEncoder: environment.NewCadenceEventEncoder(),
	}

	execCtx := fvm.NewContext(
		fvm.WithChain(chain),
		fvm.WithEventEncoder(eventEncoder),
	)

	privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
	require.NoError(t, err)
	snapshotTree, accounts, err := testutil.CreateAccounts(
		vm,
		testutil.RootBootstrappedLedger(vm, execCtx),
		privateKeys,
		chain)
	require.NoError(t, err)

	// setup transactions
	account := accounts[0]
	privKey := privateKeys[0]
	// tx1 deploys contract version 1
	tx1 := testutil.DeployEventContractTransaction(account, chain, 1)
	prepareTx(t, tx1, account, privKey, 0, chain)

	// tx2 emits event which will fail encoding
	tx2 := testutil.CreateEmitEventTransaction(account, account)
	prepareTx(t, tx2, account, privKey, 1, chain)

	// tx3 emits event that will work fine
	tx3 := testutil.CreateEmitEventTransaction(account, account)
	prepareTx(t, tx3, account, privKey, 2, chain)

	transactions := []*flow.TransactionBody{tx1, tx2, tx3}

	col := flow.Collection{Transactions: transactions}

	guarantee := flow.CollectionGuarantee{
		CollectionID: col.ID(),
		Signature:    nil,
	}

	block := flow.Block{
		Header: &flow.Header{
			View: 26,
		},
		Payload: &flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{&guarantee},
		},
	}

	executableBlock := &entity.ExecutableBlock{
		Block: &block,
		CompleteCollections: map[flow.Identifier]*entity.CompleteCollection{
			guarantee.ID(): {
				Guarantee:    &guarantee,
				Transactions: transactions,
			},
		},
		StartState: unittest.StateCommitmentPointerFixture(),
	}

	me := new(module.Local)
	me.On("NodeID").Return(unittest.IdentifierFixture())
	me.On("Sign", mock.Anything, mock.Anything).Return(nil, nil)
	me.On("SignFunc", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)

	bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	trackerStorage := mocktracker.NewMockStorage()

	prov := provider.NewProvider(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		execution_data.DefaultSerializer,
		bservice,
		trackerStorage,
	)

	blockComputer, err := computer.NewBlockComputer(
		vm,
		execCtx,
		metrics.NewNoopCollector(),
		trace.NewNoopTracer(),
		zerolog.Nop(),
		committer.NewNoopViewCommitter(),
		me,
		prov,
		nil,
		testutil.ProtocolStateWithSourceFixture(nil),
		testMaxConcurrency)
	require.NoError(t, err)

	derivedChainData, err := derived.NewDerivedChainData(10)
	require.NoError(t, err)

	engine := &Manager{
		blockComputer:    blockComputer,
		derivedChainData: derivedChainData,
	}

	eventEncoder.enabled = true

	returnedComputationResult, err := engine.ComputeBlock(
		context.Background(),
		unittest.IdentifierFixture(),
		executableBlock,
		snapshotTree)
	require.NoError(t, err)

	txResults := returnedComputationResult.AllTransactionResults()
	require.Len(t, txResults, 4) // 2 txs + 1 system tx

	require.Empty(t, txResults[0].ErrorMessage)
	require.Contains(t, txResults[1].ErrorMessage, "I failed encoding")
	require.Empty(t, txResults[2].ErrorMessage)

	colRes := returnedComputationResult.CollectionExecutionResultAt(0)
	events := colRes.Events()
	require.Len(t, events, 2) // 1 collection + 1 system chunk

	// first event should be contract deployed
	assert.EqualValues(t, "flow.AccountContractAdded", events[0].Type)

	// second event should come from tx3 (index 2)  as tx2 (index 1) should fail encoding
	hasValidEventValue(t, events[1], 1)
	assert.Equal(t, events[1].TransactionIndex, uint32(2))
}

type testingEventEncoder struct {
	realEncoder *environment.CadenceEventEncoder
	calls       int
	enabled     bool
}

func (e *testingEventEncoder) Encode(event cadence.Event) ([]byte, error) {
	defer func() {
		if e.enabled {
			e.calls++
		}
	}()

	if e.calls == 1 && e.enabled {
		return nil, fmt.Errorf("I failed encoding")
	}
	return e.realEncoder.Encode(event)
}

func TestScriptStorageMutationsDiscarded(t *testing.T) {

	timeout := 10 * time.Second
	chain := flow.Mainnet.Chain()
	ctx := fvm.NewContext(fvm.WithChain(chain))
	manager, _ := New(
		zerolog.Nop(),
		metrics.NewExecutionCollector(ctx.Tracer),
		trace.NewNoopTracer(),
		nil,
		testutil.ProtocolStateWithSourceFixture(nil),
		ctx,
		committer.NewNoopViewCommitter(),
		nil,
		ComputationConfig{
			QueryConfig: query.QueryConfig{
				LogTimeThreshold:   query.DefaultLogTimeThreshold,
				ExecutionTimeLimit: timeout,
			},
			DerivedDataCacheSize: derived.DefaultDerivedDataCacheSize,
			MaxConcurrency:       1,
		},
	)
	vm := manager.vm

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
	account := accounts[0]
	address := cadence.NewAddress(account)
	commonAddress, _ := common.HexToAddress(address.Hex())

	script := []byte(`
	access(all) fun main(account: Address) {
		let acc = getAuthAccount<auth(SaveValue) &Account>(account)
		acc.storage.save(3, to: /storage/x)
	}
	`)

	header := unittest.BlockHeaderFixture()
	_, err = manager.ExecuteScript(
		context.Background(),
		script,
		[][]byte{jsoncdc.MustEncode(address)},
		header,
		snapshotTree)

	require.NoError(t, err)

	env := environment.NewScriptEnvironmentFromStorageSnapshot(
		ctx.EnvironmentParams,
		snapshotTree)

	rt := env.BorrowCadenceRuntime()
	defer env.ReturnCadenceRuntime(rt)

	path, err := cadence.NewPath(common.PathDomainStorage, "x")
	require.NoError(t, err)

	v, err := rt.ReadStored(commonAddress, path)
	// the save should not update account storage by writing the updates
	// back to the snapshotTree
	require.NoError(t, err)
	require.Equal(t, nil, v)
}
