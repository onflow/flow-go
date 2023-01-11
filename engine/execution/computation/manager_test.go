package computation

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
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
	state2 "github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	unittest2 "github.com/onflow/flow-go/engine/execution/state/unittest"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/derived"
	"github.com/onflow/flow-go/fvm/environment"
	fvmErrors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/provider"
	"github.com/onflow/flow-go/module/executiondatasync/tracker"
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

	ledger := testutil.RootBootstrappedLedger(vm, execCtx)
	accounts, err := testutil.CreateAccounts(vm, ledger, derived.NewEmptyDerivedBlockData(), privateKeys, chain)
	require.NoError(t, err)

	tx1 := testutil.DeployCounterContractTransaction(accounts[0], chain)
	tx1.SetProposalKey(chain.ServiceAddress(), 0, 0).
		SetGasLimit(1000).
		SetPayer(chain.ServiceAddress())

	err = testutil.SignPayload(tx1, accounts[0], privateKeys[0])
	require.NoError(t, err)

	err = testutil.SignEnvelope(tx1, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	tx2 := testutil.CreateCounterTransaction(accounts[0], accounts[1])
	tx2.SetProposalKey(chain.ServiceAddress(), 0, 0).
		SetGasLimit(1000).
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
	me.On("NodeID").Return(flow.ZeroID)
	me.On("SignFunc", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)

	bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	trackerStorage := new(mocktracker.Storage)
	trackerStorage.On("Update", mock.Anything).Return(func(fn tracker.UpdateFn) error {
		return fn(func(uint64, ...cid.Cid) error { return nil })
	})

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
		prov)
	require.NoError(t, err)

	derivedChainData, err := derived.NewDerivedChainData(10)
	require.NoError(t, err)

	engine := &Manager{
		blockComputer:    blockComputer,
		me:               me,
		derivedChainData: derivedChainData,
		tracer:           trace.NewNoopTracer(),
	}

	view := delta.NewView(ledger.Get)
	blockView := view.NewChild()

	returnedComputationResult, err := engine.ComputeBlock(context.Background(), executableBlock, blockView)
	require.NoError(t, err)

	require.NotEmpty(t, blockView.(*delta.View).Delta())
	require.Len(t, returnedComputationResult.StateSnapshots, 1+1) // 1 coll + 1 system chunk
	assert.NotEmpty(t, returnedComputationResult.StateSnapshots[0].Delta)
	stats := returnedComputationResult.BlockStats()
	assert.True(t, stats.ComputationUsed > 0)
	assert.True(t, stats.MemoryUsed > 0)
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
	me.On("NodeID").Return(flow.ZeroID)
	me.On("SignFunc", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)

	computationResult := unittest2.ComputationResultFixture([][]flow.Identifier{
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
		me:               me,
		derivedChainData: derivedChainData,
		tracer:           trace.NewNoopTracer(),
	}

	view := delta.NewView(state2.LedgerGetRegister(ledger, flow.StateCommitment(ledger.InitialState())))
	blockView := view.NewChild()

	_, err = manager.ComputeBlock(context.Background(), computationResult.ExecutableBlock, blockView)
	require.NoError(t, err)
}

func TestExecuteScript(t *testing.T) {

	logger := zerolog.Nop()

	execCtx := fvm.NewContext(fvm.WithLogger(logger))

	me := new(module.Local)
	me.On("NodeID").Return(flow.ZeroID)
	me.On("SignFunc", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)

	vm := fvm.NewVirtualMachine()

	ledger := testutil.RootBootstrappedLedger(vm, execCtx, fvm.WithExecutionMemoryLimit(math.MaxUint64))

	view := delta.NewView(ledger.Get)

	scriptView := view.NewChild()

	script := []byte(fmt.Sprintf(
		`
			import FungibleToken from %s

			pub fun main() {}
		`,
		fvm.FungibleTokenAddress(execCtx.Chain).HexWithPrefix(),
	))

	bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	trackerStorage := new(mocktracker.Storage)
	trackerStorage.On("Update", mock.Anything).Return(func(fn tracker.UpdateFn) error {
		return fn(func(uint64, ...cid.Cid) error { return nil })
	})

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
		nil,
		execCtx,
		committer.NewNoopViewCommitter(),
		prov,
		ComputationConfig{
			DerivedDataCacheSize:     derived.DefaultDerivedDataCacheSize,
			ScriptLogThreshold:       scriptLogThreshold,
			ScriptExecutionTimeLimit: DefaultScriptExecutionTimeLimit,
		},
	)
	require.NoError(t, err)

	header := unittest.BlockHeaderFixture()
	_, err = engine.ExecuteScript(context.Background(), script, nil, header, scriptView)
	require.NoError(t, err)
}

// Balance script used to swallow errors, which meant that even if the view was empty, a script that did nothing but get
// the balance of an account would succeed and return 0.
func TestExecuteScript_BalanceScriptFailsIfViewIsEmpty(t *testing.T) {

	logger := zerolog.Nop()

	execCtx := fvm.NewContext(fvm.WithLogger(logger))

	me := new(module.Local)
	me.On("NodeID").Return(flow.ZeroID)
	me.On("SignFunc", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)

	view := delta.NewView(func(owner, key string) (flow.RegisterValue, error) {
		return nil, fmt.Errorf("error getting register")
	})

	scriptView := view.NewChild()

	script := []byte(fmt.Sprintf(
		`
			pub fun main(): UFix64 {
				return getAccount(%s).balance
			}
		`,
		fvm.FungibleTokenAddress(execCtx.Chain).HexWithPrefix(),
	))

	bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	trackerStorage := new(mocktracker.Storage)
	trackerStorage.On("Update", mock.Anything).Return(func(fn tracker.UpdateFn) error {
		return fn(func(uint64, ...cid.Cid) error { return nil })
	})

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
		nil,
		execCtx,
		committer.NewNoopViewCommitter(),
		prov,
		ComputationConfig{
			DerivedDataCacheSize:     derived.DefaultDerivedDataCacheSize,
			ScriptLogThreshold:       scriptLogThreshold,
			ScriptExecutionTimeLimit: DefaultScriptExecutionTimeLimit,
		},
	)
	require.NoError(t, err)

	header := unittest.BlockHeaderFixture()
	_, err = engine.ExecuteScript(context.Background(), script, nil, header, scriptView)
	require.ErrorContains(t, err, "error getting register")
}

func TestExecuteScripPanicsAreHandled(t *testing.T) {

	ctx := fvm.NewContext()

	buffer := &bytes.Buffer{}
	log := zerolog.New(buffer)

	header := unittest.BlockHeaderFixture()

	bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	trackerStorage := new(mocktracker.Storage)
	trackerStorage.On("Update", mock.Anything).Return(func(fn tracker.UpdateFn) error {
		return fn(func(uint64, ...cid.Cid) error { return nil })
	})

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
		nil,
		ctx,
		committer.NewNoopViewCommitter(),
		prov,
		ComputationConfig{
			DerivedDataCacheSize:     derived.DefaultDerivedDataCacheSize,
			ScriptLogThreshold:       scriptLogThreshold,
			ScriptExecutionTimeLimit: DefaultScriptExecutionTimeLimit,
			NewCustomVirtualMachine: func() fvm.VM {
				return &PanickingVM{}
			},
		},
	)
	require.NoError(t, err)

	_, err = manager.ExecuteScript(context.Background(), []byte("whatever"), nil, header, noopView())

	require.Error(t, err)

	require.Contains(t, buffer.String(), "Verunsicherung")
}

func TestExecuteScript_LongScriptsAreLogged(t *testing.T) {

	ctx := fvm.NewContext()

	buffer := &bytes.Buffer{}
	log := zerolog.New(buffer)

	header := unittest.BlockHeaderFixture()

	bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	trackerStorage := new(mocktracker.Storage)
	trackerStorage.On("Update", mock.Anything).Return(func(fn tracker.UpdateFn) error {
		return fn(func(uint64, ...cid.Cid) error { return nil })
	})

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
		nil,
		ctx,
		committer.NewNoopViewCommitter(),
		prov,
		ComputationConfig{
			DerivedDataCacheSize:     derived.DefaultDerivedDataCacheSize,
			ScriptLogThreshold:       1 * time.Millisecond,
			ScriptExecutionTimeLimit: DefaultScriptExecutionTimeLimit,
			NewCustomVirtualMachine: func() fvm.VM {
				return &LongRunningVM{duration: 2 * time.Millisecond}
			},
		},
	)
	require.NoError(t, err)

	_, err = manager.ExecuteScript(context.Background(), []byte("whatever"), nil, header, noopView())

	require.NoError(t, err)

	require.Contains(t, buffer.String(), "exceeded threshold")
}

func TestExecuteScript_ShortScriptsAreNotLogged(t *testing.T) {

	ctx := fvm.NewContext()

	buffer := &bytes.Buffer{}
	log := zerolog.New(buffer)

	header := unittest.BlockHeaderFixture()

	bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	trackerStorage := new(mocktracker.Storage)
	trackerStorage.On("Update", mock.Anything).Return(func(fn tracker.UpdateFn) error {
		return fn(func(uint64, ...cid.Cid) error { return nil })
	})

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
		nil,
		ctx,
		committer.NewNoopViewCommitter(),
		prov,
		ComputationConfig{
			DerivedDataCacheSize:     derived.DefaultDerivedDataCacheSize,
			ScriptLogThreshold:       1 * time.Second,
			ScriptExecutionTimeLimit: DefaultScriptExecutionTimeLimit,
			NewCustomVirtualMachine: func() fvm.VM {
				return &LongRunningVM{duration: 0}
			},
		},
	)
	require.NoError(t, err)

	_, err = manager.ExecuteScript(context.Background(), []byte("whatever"), nil, header, noopView())

	require.NoError(t, err)

	require.NotContains(t, buffer.String(), "exceeded threshold")
}

type PanickingVM struct{}

func (p *PanickingVM) Run(f fvm.Context, procedure fvm.Procedure, view state.View) error {
	panic("panic, but expected with sentinel for test: Verunsicherung ")
}

func (p *PanickingVM) GetAccount(f fvm.Context, address flow.Address, view state.View) (*flow.Account, error) {
	panic("not expected")
}

type LongRunningVM struct {
	duration time.Duration
}

func (l *LongRunningVM) Run(f fvm.Context, procedure fvm.Procedure, view state.View) error {
	time.Sleep(l.duration)
	// satisfy value marshaller
	if scriptProcedure, is := procedure.(*fvm.ScriptProcedure); is {
		scriptProcedure.Value = cadence.NewVoid()
	}

	return nil
}

func (l *LongRunningVM) GetAccount(f fvm.Context, address flow.Address, view state.View) (*flow.Account, error) {
	panic("not expected")
}

type FakeBlockComputer struct {
	computationResult *execution.ComputationResult
}

func (f *FakeBlockComputer) ExecuteBlock(context.Context, *entity.ExecutableBlock, state.View, *derived.DerivedBlockData) (*execution.ComputationResult, error) {
	return f.computationResult, nil
}

func noopView() *delta.View {
	return delta.NewView(func(_, _ string) (flow.RegisterValue, error) {
		return nil, nil
	})
}

func TestExecuteScriptTimeout(t *testing.T) {

	timeout := 1 * time.Millisecond
	manager, err := New(
		zerolog.Nop(),
		metrics.NewNoopCollector(),
		trace.NewNoopTracer(),
		nil,
		nil,
		fvm.NewContext(),
		committer.NewNoopViewCommitter(),
		nil,
		ComputationConfig{
			DerivedDataCacheSize:     derived.DefaultDerivedDataCacheSize,
			ScriptLogThreshold:       DefaultScriptLogThreshold,
			ScriptExecutionTimeLimit: timeout,
		},
	)

	require.NoError(t, err)

	script := []byte(`
	pub fun main(): Int {
		var i = 0
		while i < 10000 {
			i = i + 1
		}
		return i
	}
	`)

	header := unittest.BlockHeaderFixture()
	value, err := manager.ExecuteScript(context.Background(), script, nil, header, noopView())

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
		nil,
		fvm.NewContext(),
		committer.NewNoopViewCommitter(),
		nil,
		ComputationConfig{
			DerivedDataCacheSize:     derived.DefaultDerivedDataCacheSize,
			ScriptLogThreshold:       DefaultScriptLogThreshold,
			ScriptExecutionTimeLimit: timeout,
		},
	)

	require.NoError(t, err)

	script := []byte(`
	pub fun main(): Int {
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
		value, err = manager.ExecuteScript(reqCtx, script, nil, header, noopView())
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
	ledger := testutil.RootBootstrappedLedger(vm, execCtx)
	accounts, err := testutil.CreateAccounts(vm, ledger, derived.NewEmptyDerivedBlockData(), privateKeys, chain)
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
	me.On("NodeID").Return(flow.ZeroID)
	me.On("SignFunc", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)

	bservice := requesterunit.MockBlobService(blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore())))
	trackerStorage := new(mocktracker.Storage)
	trackerStorage.On("Update", mock.Anything).Return(func(fn tracker.UpdateFn) error {
		return fn(func(uint64, ...cid.Cid) error { return nil })
	})

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
	)
	require.NoError(t, err)

	derivedChainData, err := derived.NewDerivedChainData(10)
	require.NoError(t, err)

	engine := &Manager{
		blockComputer:    blockComputer,
		me:               me,
		derivedChainData: derivedChainData,
		tracer:           trace.NewNoopTracer(),
	}

	view := delta.NewView(ledger.Get)
	blockView := view.NewChild()

	eventEncoder.enabled = true

	returnedComputationResult, err := engine.ComputeBlock(context.Background(), executableBlock, blockView)
	require.NoError(t, err)

	require.Len(t, returnedComputationResult.Events, 2)             // 1 collection + 1 system chunk
	require.Len(t, returnedComputationResult.TransactionResults, 4) // 2 txs + 1 system tx

	require.Empty(t, returnedComputationResult.TransactionResults[0].ErrorMessage)
	require.Contains(t, returnedComputationResult.TransactionResults[1].ErrorMessage, "I failed encoding")
	require.Empty(t, returnedComputationResult.TransactionResults[2].ErrorMessage)

	// first event should be contract deployed
	assert.EqualValues(t, "flow.AccountContractAdded", returnedComputationResult.Events[0][0].Type)

	// second event should come from tx3 (index 2)  as tx2 (index 1) should fail encoding
	hasValidEventValue(t, returnedComputationResult.Events[0][1], 1)
	assert.Equal(t, returnedComputationResult.Events[0][1].TransactionIndex, uint32(2))
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
		nil,
		ctx,
		committer.NewNoopViewCommitter(),
		nil,
		ComputationConfig{
			DerivedDataCacheSize:     derived.DefaultDerivedDataCacheSize,
			ScriptLogThreshold:       DefaultScriptLogThreshold,
			ScriptExecutionTimeLimit: timeout,
		},
	)
	vm := manager.vm
	view := testutil.RootBootstrappedLedger(vm, ctx)

	derivedBlockData := derived.NewEmptyDerivedBlockData()
	derivedTxnData, err := derivedBlockData.NewDerivedTransactionData(0, 0)
	require.NoError(t, err)

	txnState := state.NewTransactionState(view, state.DefaultParameters())

	// Create an account private key.
	privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
	require.NoError(t, err)

	// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
	accounts, err := testutil.CreateAccounts(vm, view, derivedBlockData, privateKeys, chain)
	require.NoError(t, err)
	account := accounts[0]
	address := cadence.NewAddress(account)
	commonAddress, _ := common.HexToAddress(address.Hex())

	script := []byte(`
	pub fun main(account: Address) {
		let acc = getAuthAccount(account)
		acc.save(3, to: /storage/x)
	}
	`)

	header := unittest.BlockHeaderFixture()
	scriptView := view.NewChild()
	_, err = manager.ExecuteScript(context.Background(), script, [][]byte{jsoncdc.MustEncode(address)}, header, scriptView)

	require.NoError(t, err)

	env := environment.NewScriptEnvironment(
		context.Background(),
		ctx.TracerSpan,
		ctx.EnvironmentParams,
		txnState,
		derivedTxnData)

	rt := env.BorrowCadenceRuntime()
	defer env.ReturnCadenceRuntime(rt)

	v, err := rt.ReadStored(
		commonAddress,
		cadence.NewPath("storage", "x"),
	)

	// the save should not update account storage by writing the delta from the child view back to the parent
	require.NoError(t, err)
	require.Equal(t, nil, v)
}
