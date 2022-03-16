package computer_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation/committer"
	"github.com/onflow/flow-go/engine/execution/computation/computer"
	computermock "github.com/onflow/flow-go/engine/execution/computation/computer/mock"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	fvmErrors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/epochs"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/metrics"
	modulemock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBlockExecutor_ExecuteBlock(t *testing.T) {

	rag := &RandomAddressGenerator{}

	t.Run("single collection", func(t *testing.T) {

		execCtx := fvm.NewContext(zerolog.Nop())

		vm := new(computermock.VirtualMachine)
		vm.On("Run", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).
			Run(func(args mock.Arguments) {
				//ctx := args[0].(fvm.Context)
				tx := args[1].(*fvm.TransactionProcedure)

				tx.Events = generateEvents(1, tx.TxIndex)
			}).
			Times(2 + 1) // 2 txs in collection + system chunk

		committer := new(computermock.ViewCommitter)
		committer.On("CommitView", mock.Anything, mock.Anything).
			Return(nil, nil, nil, nil).
			Times(2 + 1) // 2 txs in collection + system chunk

		metrics := new(modulemock.ExecutionMetrics)
		metrics.On("ExecutionCollectionExecuted", mock.Anything, mock.Anything, mock.Anything).
			Return(nil).
			Times(2) // 1 collection + system collection

		metrics.On("ExecutionTransactionExecuted", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).
			Times(2 + 1) // 2 txs in collection + system chunk tx

		exe, err := computer.NewBlockComputer(vm, execCtx, metrics, trace.NewNoopTracer(), zerolog.Nop(), committer)
		require.NoError(t, err)

		// create a block with 1 collection with 2 transactions
		block := generateBlock(1, 2, rag)

		view := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		result, err := exe.ExecuteBlock(context.Background(), block, view, programs.NewEmptyPrograms())
		assert.NoError(t, err)
		assert.Len(t, result.StateSnapshots, 1+1) // +1 system chunk
		assert.Len(t, result.TrieUpdates, 1+1)    // +1 system chunk

		assertEventHashesMatch(t, 1+1, result)

		vm.AssertExpectations(t)
	})

	t.Run("empty block still computes system chunk", func(t *testing.T) {

		execCtx := fvm.NewContext(
			zerolog.Nop(),
		)

		vm := new(computermock.VirtualMachine)
		committer := new(computermock.ViewCommitter)

		exe, err := computer.NewBlockComputer(vm, execCtx, metrics.NewNoopCollector(), trace.NewNoopTracer(), zerolog.Nop(), committer)
		require.NoError(t, err)

		// create an empty block
		block := generateBlock(0, 0, rag)
		programs := programs.NewEmptyPrograms()

		vm.On("Run", mock.Anything, mock.Anything, mock.Anything, programs).
			Return(nil).
			Once() // just system chunk

		committer.On("CommitView", mock.Anything, mock.Anything).
			Return(nil, nil, nil, nil).
			Once() // just system chunk

		view := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		result, err := exe.ExecuteBlock(context.Background(), block, view, programs)
		assert.NoError(t, err)
		assert.Len(t, result.StateSnapshots, 1)
		assert.Len(t, result.TrieUpdates, 1)
		assert.Len(t, result.TransactionResults, 1)

		assertEventHashesMatch(t, 1, result)

		vm.AssertExpectations(t)
	})

	t.Run("system chunk transaction should not fail", func(t *testing.T) {

		// include all fees. System chunk should ignore them
		contextOptions := []fvm.Option{
			fvm.WithTransactionFeesEnabled(true),
			fvm.WithAccountStorageLimit(true),
			fvm.WithBlocks(&fvm.NoopBlockFinder{}),
		}
		// set 0 clusters to pass n_collectors >= n_clusters check
		epochConfig := epochs.DefaultEpochConfig()
		epochConfig.NumCollectorClusters = 0
		bootstrapOptions := []fvm.BootstrapProcedureOption{
			fvm.WithTransactionFee(fvm.DefaultTransactionFees),
			fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
			fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
			fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
			fvm.WithEpochConfig(epochConfig),
		}

		rt := fvm.NewInterpreterRuntime()
		chain := flow.Localnet.Chain()
		vm := fvm.NewVirtualMachine(rt)
		baseOpts := []fvm.Option{
			fvm.WithChain(chain),
		}

		opts := append(baseOpts, contextOptions...)
		ctx := fvm.NewContext(zerolog.Nop(), opts...)
		view := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		baseBootstrapOpts := []fvm.BootstrapProcedureOption{
			fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
		}
		progs := programs.NewEmptyPrograms()
		bootstrapOpts := append(baseBootstrapOpts, bootstrapOptions...)
		err := vm.Run(ctx, fvm.Bootstrap(unittest.ServiceAccountPublicKey, bootstrapOpts...), view, progs)
		require.NoError(t, err)

		comm := new(computermock.ViewCommitter)

		exe, err := computer.NewBlockComputer(vm, ctx, metrics.NewNoopCollector(), trace.NewNoopTracer(), zerolog.Nop(), comm)
		require.NoError(t, err)

		// create an empty block
		block := generateBlock(0, 0, rag)

		comm.On("CommitView", mock.Anything, mock.Anything).
			Return(nil, nil, nil, nil).
			Once() // just system chunk

		result, err := exe.ExecuteBlock(context.Background(), block, view, progs)
		assert.NoError(t, err)
		assert.Len(t, result.StateSnapshots, 1)
		assert.Len(t, result.TrieUpdates, 1)
		assert.Len(t, result.TransactionResults, 1)

		assert.Empty(t, result.TransactionResults[0].ErrorMessage)
	})

	t.Run("multiple collections", func(t *testing.T) {
		execCtx := fvm.NewContext(zerolog.Nop())

		vm := new(computermock.VirtualMachine)
		committer := new(computermock.ViewCommitter)

		exe, err := computer.NewBlockComputer(vm, execCtx, metrics.NewNoopCollector(), trace.NewNoopTracer(), zerolog.Nop(), committer)
		require.NoError(t, err)

		collectionCount := 2
		transactionsPerCollection := 2
		eventsPerTransaction := 2
		eventsPerCollection := eventsPerTransaction * transactionsPerCollection
		totalTransactionCount := (collectionCount * transactionsPerCollection) + 1 //+1 for system chunk
		//totalEventCount := eventsPerTransaction * totalTransactionCount

		// create a block with 2 collections with 2 transactions each
		block := generateBlock(collectionCount, transactionsPerCollection, rag)
		programs := programs.NewEmptyPrograms()

		vm.On("Run", mock.Anything, mock.Anything, mock.Anything, programs).
			Run(func(args mock.Arguments) {
				tx := args[1].(*fvm.TransactionProcedure)

				tx.Err = fvmErrors.NewInvalidAddressErrorf(flow.Address{}, "no payer address provided")
				// create dummy events
				tx.Events = generateEvents(eventsPerTransaction, tx.TxIndex)
			}).
			Return(nil).
			Times(totalTransactionCount)

		committer.On("CommitView", mock.Anything, mock.Anything).
			Return(nil, nil, nil, nil).
			Times(collectionCount + 1)

		view := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		result, err := exe.ExecuteBlock(context.Background(), block, view, programs)
		assert.NoError(t, err)

		// chunk count should match collection count
		assert.Len(t, result.StateSnapshots, collectionCount+1) // system chunk

		// all events should have been collected
		assert.Len(t, result.Events, collectionCount+1)

		for i := 0; i < collectionCount; i++ {
			assert.Len(t, result.Events[i], eventsPerCollection)
		}

		assert.Len(t, result.Events[len(result.Events)-1], eventsPerTransaction)

		// events should have been indexed by transaction and event
		k := 0
		for expectedTxIndex := 0; expectedTxIndex < totalTransactionCount; expectedTxIndex++ {
			for expectedEventIndex := 0; expectedEventIndex < eventsPerTransaction; expectedEventIndex++ {

				chunkIndex := k / eventsPerCollection
				eventIndex := k % eventsPerCollection

				e := result.Events[chunkIndex][eventIndex]
				assert.EqualValues(t, expectedEventIndex, int(e.EventIndex))
				assert.EqualValues(t, expectedTxIndex, e.TransactionIndex)
				k++
			}
		}

		expectedResults := make([]flow.TransactionResult, 0)
		for _, c := range block.CompleteCollections {
			for _, t := range c.Transactions {
				txResult := flow.TransactionResult{
					TransactionID: t.ID(),
					ErrorMessage:  fvmErrors.NewInvalidAddressErrorf(flow.Address{}, "no payer address provided").Error(),
				}
				expectedResults = append(expectedResults, txResult)
			}
		}
		assert.ElementsMatch(t, expectedResults, result.TransactionResults[0:len(result.TransactionResults)-1]) //strip system chunk

		assertEventHashesMatch(t, collectionCount+1, result)

		vm.AssertExpectations(t)
	})

	t.Run("Same block has same execution result", func(t *testing.T) {
		execCtx := fvm.NewContext(zerolog.Nop())

		vm := new(computermock.VirtualMachine)
		committer := new(computermock.ViewCommitter)

		exe, err := computer.NewBlockComputer(vm, execCtx, metrics.NewNoopCollector(), trace.NewNoopTracer(), zerolog.Nop(), committer)
		require.NoError(t, err)

		collectionCount := 2
		transactionsPerCollection := 2
		eventsPerTransaction := 2
		eventsPerCollection := eventsPerTransaction * transactionsPerCollection
		totalTransactionCount := (collectionCount * transactionsPerCollection) + 1 //+1 for system chunk
		//totalEventCount := eventsPerTransaction * totalTransactionCount

		// create a block with 2 collections with 2 transactions each
		block1 := generateBlock(collectionCount, transactionsPerCollection, rag)
		block2 := blockCopy(block1)

		programs := programs.NewEmptyPrograms()

		vm.On("Run", mock.Anything, mock.Anything, mock.Anything, programs).
			Run(func(args mock.Arguments) {
				tx := args[1].(*fvm.TransactionProcedure)

				tx.Err = fvmErrors.NewInvalidAddressErrorf(flow.Address{}, "no payer address provided")
				// create dummy events
				tx.Events = generateEvents(eventsPerTransaction, tx.TxIndex)
			}).
			Return(nil).
			Times(totalTransactionCount * 2)

		committer.On("CommitView", mock.Anything, mock.Anything).
			Return(nil, nil, nil, nil).
			Times((collectionCount + 1) * 2)

		view1 := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})
		view2 := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		result1, err1 := exe.ExecuteBlock(context.Background(), block1, view1, programs)
		result2, err2 := exe.ExecuteBlock(context.Background(), block2, view2, programs)
		assert.NoError(t, err1)
		assert.NoError(t, err2)

		for i := 0; i < collectionCount; i++ {
			assert.Len(t, result1.Events[i], eventsPerCollection)
			assert.Len(t, result2.Events[i], eventsPerCollection)
		}

		// events should have been indexed by transaction and event
		k := 0
		for expectedTxIndex := 0; expectedTxIndex < totalTransactionCount; expectedTxIndex++ {
			for expectedEventIndex := 0; expectedEventIndex < eventsPerTransaction; expectedEventIndex++ {

				chunkIndex := k / eventsPerCollection
				eventIndex := k % eventsPerCollection

				e1 := result1.Events[chunkIndex][eventIndex]
				e2 := result2.Events[chunkIndex][eventIndex]
				assert.EqualValues(t, expectedEventIndex, int(e1.EventIndex))
				assert.EqualValues(t, expectedTxIndex, e1.TransactionIndex)
				assert.EqualValues(t, expectedEventIndex, int(e2.EventIndex))
				assert.EqualValues(t, expectedTxIndex, e2.TransactionIndex)
				k++
			}
		}

		expectedResults := make([]flow.TransactionResult, 0)
		for _, c := range block1.CompleteCollections {
			for _, t := range c.Transactions {
				txResult := flow.TransactionResult{
					TransactionID: t.ID(),
					ErrorMessage:  fvmErrors.NewInvalidAddressErrorf(flow.Address{}, "no payer address provided").Error(),
				}
				expectedResults = append(expectedResults, txResult)
			}
		}
		assert.ElementsMatch(t, expectedResults, result1.TransactionResults[0:len(result1.TransactionResults)-1]) //strip system chunk
		assert.ElementsMatch(t, expectedResults, result2.TransactionResults[0:len(result2.TransactionResults)-1]) //strip system chunk

		assertEventHashesMatch(t, collectionCount+1, result1)
		assertEventHashesMatch(t, collectionCount+1, result2)

		vm.AssertExpectations(t)
	})

	t.Run("service events are emitted", func(t *testing.T) {
		execCtx := fvm.NewContext(zerolog.Nop(), fvm.WithServiceEventCollectionEnabled(), fvm.WithTransactionProcessors(
			fvm.NewTransactionInvoker(zerolog.Nop()), //we don't need to check signatures or sequence numbers
		))

		collectionCount := 2
		transactionsPerCollection := 2

		totalTransactionCount := (collectionCount * transactionsPerCollection) + 1 //+1 for system chunk

		// create a block with 2 collections with 2 transactions each
		block := generateBlock(collectionCount, transactionsPerCollection, rag)

		ordinaryEvent := cadence.Event{
			EventType: &cadence.EventType{
				Location:            stdlib.FlowLocation{},
				QualifiedIdentifier: "what.ever",
			},
		}

		serviceEvents, err := systemcontracts.ServiceEventsForChain(execCtx.Chain.ChainID())
		require.NoError(t, err)

		serviceEventA := cadence.Event{
			EventType: &cadence.EventType{
				Location: common.AddressLocation{
					Address: common.Address(serviceEvents.EpochSetup.Address),
				},
				QualifiedIdentifier: serviceEvents.EpochSetup.QualifiedIdentifier(),
			},
		}
		serviceEventB := cadence.Event{
			EventType: &cadence.EventType{
				Location: common.AddressLocation{
					Address: common.Address(serviceEvents.EpochCommit.Address),
				},
				QualifiedIdentifier: serviceEvents.EpochCommit.QualifiedIdentifier(),
			},
		}

		//events to emit for each iteration/transaction
		events := make([][]cadence.Event, totalTransactionCount)
		events[0] = nil
		events[1] = []cadence.Event{serviceEventA, ordinaryEvent}
		events[2] = []cadence.Event{ordinaryEvent}
		events[3] = nil
		events[4] = []cadence.Event{serviceEventB}

		emittingRuntime := &testRuntime{
			executeTransaction: func(script runtime.Script, context runtime.Context) error {
				for _, e := range events[0] {
					err := context.Interface.EmitEvent(e)
					if err != nil {
						return err
					}
				}
				events = events[1:]
				return nil
			},
		}

		vm := fvm.NewVirtualMachine(emittingRuntime)

		exe, err := computer.NewBlockComputer(vm, execCtx, metrics.NewNoopCollector(), trace.NewNoopTracer(), zerolog.Nop(), committer.NewNoopViewCommitter())
		require.NoError(t, err)

		view := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		result, err := exe.ExecuteBlock(context.Background(), block, view, programs.NewEmptyPrograms())
		require.NoError(t, err)

		// all events should have been collected
		require.Len(t, result.ServiceEvents, 2)

		// events are ordered
		require.Equal(t, serviceEventA.EventType.ID(), string(result.ServiceEvents[0].Type))
		require.Equal(t, serviceEventB.EventType.ID(), string(result.ServiceEvents[1].Type))

		assertEventHashesMatch(t, collectionCount+1, result)
	})

	t.Run("succeeding transactions store programs", func(t *testing.T) {

		execCtx := fvm.NewContext(zerolog.Nop())

		contractLocation := common.AddressLocation{
			Address: common.Address{0x1},
			Name:    "Test",
		}

		contractProgram := &interpreter.Program{}

		rt := &testRuntime{
			executeTransaction: func(script runtime.Script, r runtime.Context) error {

				program, err := r.Interface.GetProgram(contractLocation)
				require.NoError(t, err)
				require.Nil(t, program)

				err = r.Interface.SetProgram(
					contractLocation,
					contractProgram,
				)
				require.NoError(t, err)

				return nil
			},
		}

		vm := fvm.NewVirtualMachine(rt)

		exe, err := computer.NewBlockComputer(vm, execCtx, metrics.NewNoopCollector(), trace.NewNoopTracer(), zerolog.Nop(), committer.NewNoopViewCommitter())
		require.NoError(t, err)

		const collectionCount = 2
		const transactionCount = 2
		block := generateBlock(collectionCount, transactionCount, rag)

		view := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		result, err := exe.ExecuteBlock(context.Background(), block, view, programs.NewEmptyPrograms())
		assert.NoError(t, err)
		assert.Len(t, result.StateSnapshots, collectionCount+1) // +1 system chunk
	})

	t.Run("failing transactions do not store programs", func(t *testing.T) {

		logger := zerolog.Nop()

		execCtx := fvm.NewContext(
			logger,
			fvm.WithTransactionProcessors(
				fvm.NewTransactionInvoker(logger),
			),
		)

		contractLocation := common.AddressLocation{
			Address: common.Address{0x1},
			Name:    "Test",
		}

		contractProgram := &interpreter.Program{}

		const collectionCount = 2
		const transactionCount = 2

		var executionCalls int

		rt := &testRuntime{
			executeTransaction: func(script runtime.Script, r runtime.Context) error {

				executionCalls++

				// NOTE: set a program and revert all transactions but the system chunk transaction

				program, err := r.Interface.GetProgram(contractLocation)
				require.NoError(t, err)

				if executionCalls > collectionCount*transactionCount {
					return nil
				}
				if program == nil {

					err = r.Interface.SetProgram(
						contractLocation,
						contractProgram,
					)
					require.NoError(t, err)

				}
				return runtime.Error{
					Err: fmt.Errorf("TX reverted"),
				}
			},
		}

		vm := fvm.NewVirtualMachine(rt)

		exe, err := computer.NewBlockComputer(vm, execCtx, metrics.NewNoopCollector(), trace.NewNoopTracer(), zerolog.Nop(), committer.NewNoopViewCommitter())
		require.NoError(t, err)

		block := generateBlock(collectionCount, transactionCount, rag)

		view := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		result, err := exe.ExecuteBlock(context.Background(), block, view, programs.NewEmptyPrograms())
		require.NoError(t, err)
		assert.Len(t, result.StateSnapshots, collectionCount+1) // +1 system chunk
	})
}

func assertEventHashesMatch(t *testing.T, expectedNoOfChunks int, result *execution.ComputationResult) {

	require.Len(t, result.Events, expectedNoOfChunks)
	require.Len(t, result.EventsHashes, expectedNoOfChunks)

	for i := 0; i < expectedNoOfChunks; i++ {
		calculatedHash, err := flow.EventsMerkleRootHash(result.Events[i])
		require.NoError(t, err)

		require.Equal(t, calculatedHash, result.EventsHashes[i])
	}
}

type testRuntime struct {
	executeScript      func(runtime.Script, runtime.Context) (cadence.Value, error)
	executeTransaction func(runtime.Script, runtime.Context) error
}

var _ runtime.Runtime = &testRuntime{}

func (e *testRuntime) SetTracingEnabled(_ bool) {
	panic("SetTracingEnabled not expected")
}

func (e *testRuntime) SetResourceOwnerChangeHandlerEnabled(_ bool) {
	panic("SetResourceOwnerChangeHandlerEnabled not expected")
}

func (e *testRuntime) InvokeContractFunction(_ common.AddressLocation, _ string, _ []interpreter.Value, _ []sema.Type, _ runtime.Context) (cadence.Value, error) {
	panic("InvokeContractFunction not expected")
}

func (e *testRuntime) ExecuteScript(script runtime.Script, context runtime.Context) (cadence.Value, error) {
	return e.executeScript(script, context)
}

func (e *testRuntime) ExecuteTransaction(script runtime.Script, context runtime.Context) error {
	return e.executeTransaction(script, context)
}

func (*testRuntime) ParseAndCheckProgram(_ []byte, _ runtime.Context) (*interpreter.Program, error) {
	panic("ParseAndCheckProgram not expected")
}

func (*testRuntime) SetCoverageReport(_ *runtime.CoverageReport) {
	panic("SetCoverageReport not expected")
}

func (*testRuntime) SetContractUpdateValidationEnabled(_ bool) {
	panic("SetContractUpdateValidationEnabled not expected")
}

func (*testRuntime) SetAtreeValidationEnabled(_ bool) {
	panic("SetAtreeValidationEnabled not expected")
}

func (*testRuntime) ReadStored(_ common.Address, _ cadence.Path, _ runtime.Context) (cadence.Value, error) {
	panic("ReadStored not expected")
}

func (*testRuntime) ReadLinked(_ common.Address, _ cadence.Path, _ runtime.Context) (cadence.Value, error) {
	panic("ReadLinked not expected")
}

type RandomAddressGenerator struct{}

func (r *RandomAddressGenerator) NextAddress() (flow.Address, error) {
	return flow.HexToAddress(fmt.Sprintf("0%d", rand.Intn(1000))), nil
}

func (r *RandomAddressGenerator) CurrentAddress() flow.Address {
	return flow.HexToAddress(fmt.Sprintf("0%d", rand.Intn(1000)))
}

func (r *RandomAddressGenerator) Bytes() []byte {
	panic("not implemented")
}

func (r *RandomAddressGenerator) AddressCount() uint64 {
	panic("not implemented")
}

type FixedAddressGenerator struct {
	Address flow.Address
}

func (f *FixedAddressGenerator) NextAddress() (flow.Address, error) {
	return f.Address, nil
}

func (f *FixedAddressGenerator) CurrentAddress() flow.Address {
	return f.Address
}

func (f *FixedAddressGenerator) Bytes() []byte {
	panic("not implemented")
}

func (f *FixedAddressGenerator) AddressCount() uint64 {
	panic("not implemented")
}

func Test_FreezeAccountChecksAreIncluded(t *testing.T) {

	address := flow.HexToAddress("1234")
	fag := &FixedAddressGenerator{Address: address}

	rt := fvm.NewInterpreterRuntime()
	vm := fvm.NewVirtualMachine(rt)
	execCtx := fvm.NewContext(zerolog.Nop())

	ledger := testutil.RootBootstrappedLedger(vm, execCtx)

	key, err := unittest.AccountKeyDefaultFixture()
	require.NoError(t, err)

	view := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
		return ledger.Get(owner, controller, key)
	})
	sth := state.NewStateHolder(state.NewState(view))
	accounts := state.NewAccounts(sth)

	// account creation, signing of transaction and bootstrapping ledger should not be required for this test
	// as freeze check should happen before a transaction signature is checked
	// but we currently discard all the touches if it fails and any point
	err = accounts.Create([]flow.AccountPublicKey{key.PublicKey(1000)}, address)
	require.NoError(t, err)

	exe, err := computer.NewBlockComputer(vm, execCtx, metrics.NewNoopCollector(), trace.NewNoopTracer(), zerolog.Nop(), committer.NewNoopViewCommitter())
	require.NoError(t, err)

	block := generateBlockWithVisitor(1, 1, fag, func(txBody *flow.TransactionBody) {
		err := testutil.SignTransaction(txBody, txBody.Payer, *key, 0)
		require.NoError(t, err)
	})

	_, err = exe.ExecuteBlock(context.Background(), block, view, programs.NewEmptyPrograms())
	assert.NoError(t, err)

	registerTouches := view.Interactions().RegisterTouches()

	// make sure check for frozen account has been registered
	id := flow.RegisterID{
		Owner:      string(address.Bytes()),
		Controller: "",
		Key:        state.KeyAccountFrozen,
	}

	require.Contains(t, registerTouches, id.String())
	require.Equal(t, id, registerTouches[id.String()])
}

func Test_ExecutingSystemCollection(t *testing.T) {

	execCtx := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(flow.Localnet.Chain()),
		fvm.WithBlocks(&fvm.NoopBlockFinder{}),
	)

	runtime := fvm.NewInterpreterRuntime()
	vm := fvm.NewVirtualMachine(runtime)

	rag := &RandomAddressGenerator{}

	ledger := testutil.RootBootstrappedLedger(vm, execCtx)

	committer := new(computermock.ViewCommitter)
	committer.On("CommitView", mock.Anything, mock.Anything).
		Return(nil, nil, nil, nil).
		Times(1) // only system chunk

	metrics := new(modulemock.ExecutionMetrics)
	metrics.On("ExecutionCollectionExecuted", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Times(1) // system collection

	metrics.On("ExecutionTransactionExecuted", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Times(1) // system chunk tx

	exe, err := computer.NewBlockComputer(vm, execCtx, metrics, trace.NewNoopTracer(), zerolog.Nop(), committer)
	require.NoError(t, err)

	// create empty block, it will have system collection attached while executing
	block := generateBlock(0, 0, rag)

	view := delta.NewView(ledger.Get)

	result, err := exe.ExecuteBlock(context.Background(), block, view, programs.NewEmptyPrograms())
	assert.NoError(t, err)
	assert.Len(t, result.StateSnapshots, 1) // +1 system chunk
	assert.Len(t, result.TransactionResults, 1)

	assert.Empty(t, result.TransactionResults[0].ErrorMessage)

	committer.AssertExpectations(t)
}

func generateBlock(collectionCount, transactionCount int, addressGenerator flow.AddressGenerator) *entity.ExecutableBlock {
	return generateBlockWithVisitor(collectionCount, transactionCount, addressGenerator, nil)
}

func generateBlockWithVisitor(collectionCount, transactionCount int, addressGenerator flow.AddressGenerator, visitor func(body *flow.TransactionBody)) *entity.ExecutableBlock {
	collections := make([]*entity.CompleteCollection, collectionCount)
	guarantees := make([]*flow.CollectionGuarantee, collectionCount)
	completeCollections := make(map[flow.Identifier]*entity.CompleteCollection)

	for i := 0; i < collectionCount; i++ {
		collection := generateCollection(transactionCount, addressGenerator, visitor)
		collections[i] = collection
		guarantees[i] = collection.Guarantee
		completeCollections[collection.Guarantee.ID()] = collection
	}

	block := flow.Block{
		Header: &flow.Header{
			Timestamp: flow.GenesisTime,
			Height:    42,
			View:      42,
		},
		Payload: &flow.Payload{
			Guarantees: guarantees,
		},
	}

	return &entity.ExecutableBlock{
		Block:               &block,
		CompleteCollections: completeCollections,
		StartState:          unittest.StateCommitmentPointerFixture(),
	}
}

func generateCollection(transactionCount int, addressGenerator flow.AddressGenerator, visitor func(body *flow.TransactionBody)) *entity.CompleteCollection {
	transactions := make([]*flow.TransactionBody, transactionCount)

	for i := 0; i < transactionCount; i++ {
		nextAddress, err := addressGenerator.NextAddress()
		if err != nil {
			panic(fmt.Errorf("cannot generate next address in test: %w", err))
		}
		txBody := &flow.TransactionBody{
			Payer:  nextAddress, // a unique payer for each tx to generate a unique id
			Script: []byte("transaction { execute {} }"),
		}
		if visitor != nil {
			visitor(txBody)
		}
		transactions[i] = txBody
	}

	collection := flow.Collection{Transactions: transactions}

	guarantee := &flow.CollectionGuarantee{CollectionID: collection.ID()}

	return &entity.CompleteCollection{
		Guarantee:    guarantee,
		Transactions: transactions,
	}
}

func generateEvents(eventCount int, txIndex uint32) []flow.Event {
	events := make([]flow.Event, eventCount)
	for i := 0; i < eventCount; i++ {
		// creating some dummy event
		event := flow.Event{Type: "whatever", EventIndex: uint32(i), TransactionIndex: txIndex}
		events[i] = event
	}
	return events
}

func blockCopy(oldBlock *entity.ExecutableBlock) *entity.ExecutableBlock {
	newBlock := &entity.ExecutableBlock{}
	newBlock.Block = oldBlock.Block
	newBlock.CompleteCollections = oldBlock.CompleteCollections
	newBlock.StartState = oldBlock.StartState
	newBlock.Executing = oldBlock.Executing
	return newBlock
}
