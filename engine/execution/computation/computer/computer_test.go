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
	"github.com/onflow/cadence/runtime/stdlib"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/computation/computer"
	computermock "github.com/onflow/flow-go/engine/execution/computation/computer/mock"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	fvmErrors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/event"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBlockExecutor_ExecuteBlock(t *testing.T) {

	rag := &RandomAddressGenerator{}

	t.Run("single collection", func(t *testing.T) {

		execCtx := fvm.NewContext(zerolog.Nop())

		vm := new(computermock.VirtualMachine)

		exe, err := computer.NewBlockComputer(vm, execCtx, nil, nil, zerolog.Nop())
		require.NoError(t, err)

		// create a block with 1 collection with 2 transactions
		block := generateBlock(1, 2, rag)

		vm.On("Run", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil).
			Times(2 + 1) // 2 txs in collection + system chunk

		view := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		result, err := exe.ExecuteBlock(context.Background(), block, view, programs.NewEmptyPrograms())
		assert.NoError(t, err)
		assert.Len(t, result.StateSnapshots, 1+1) // +1 system chunk

		vm.AssertExpectations(t)
	})

	t.Run("empty block still computes system chunk", func(t *testing.T) {

		execCtx := fvm.NewContext(zerolog.Nop())

		vm := new(computermock.VirtualMachine)

		exe, err := computer.NewBlockComputer(vm, execCtx, nil, nil, zerolog.Nop())
		require.NoError(t, err)

		// create an empty block
		block := generateBlock(0, 0, rag)
		programs := programs.NewEmptyPrograms()

		vm.On("Run", mock.Anything, mock.Anything, mock.Anything, programs).
			Return(nil).
			Once() // just system chunk

		view := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		result, err := exe.ExecuteBlock(context.Background(), block, view, programs)
		assert.NoError(t, err)
		assert.Len(t, result.StateSnapshots, 1)
		assert.Len(t, result.TransactionResult, 1)

		vm.AssertExpectations(t)
	})

	t.Run("multiple collections", func(t *testing.T) {
		execCtx := fvm.NewContext(zerolog.Nop())

		vm := new(computermock.VirtualMachine)

		exe, err := computer.NewBlockComputer(vm, execCtx, nil, nil, zerolog.Nop())
		require.NoError(t, err)

		collectionCount := 2
		transactionsPerCollection := 2
		eventsPerTransaction := 2
		totalTransactionCount := (collectionCount * transactionsPerCollection) + 1 //+1 for system chunk
		totalEventCount := eventsPerTransaction * totalTransactionCount

		// create a block with 2 collections with 2 transactions each
		block := generateBlock(collectionCount, transactionsPerCollection, rag)
		programs := programs.NewEmptyPrograms()

		vm.On("Run", mock.Anything, mock.Anything, mock.Anything, programs).
			Run(func(args mock.Arguments) {
				tx := args[1].(*fvm.TransactionProcedure)

				tx.Err = fvmErrors.NewInvalidAddressError("no payer address provided", flow.Address{})
				// create dummy events
				tx.Events = generateEvents(eventsPerTransaction, tx.TxIndex)
			}).
			Return(nil).
			Times(totalTransactionCount)

		view := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		result, err := exe.ExecuteBlock(context.Background(), block, view, programs)
		assert.NoError(t, err)

		// chunk count should match collection count
		assert.Len(t, result.StateSnapshots, collectionCount+1) // system chunk

		// all events should have been collected
		assert.Len(t, result.Events, totalEventCount)

		// events should have been indexed by transaction and event
		k := 0
		for expectedTxIndex := 0; expectedTxIndex < totalTransactionCount; expectedTxIndex++ {
			for expectedEventIndex := 0; expectedEventIndex < eventsPerTransaction; expectedEventIndex++ {
				e := result.Events[k]
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
					ErrorMessage:  "invalid address (0000000000000000): no payer address provided",
				}
				expectedResults = append(expectedResults, txResult)
			}
		}
		assert.ElementsMatch(t, expectedResults, result.TransactionResult[0:len(result.TransactionResult)-1]) //strip system chunk

		vm.AssertExpectations(t)
	})

	t.Run("service events are emitted", func(t *testing.T) {
		execCtx := fvm.NewContext(zerolog.Nop(), fvm.WithTransactionProcessors(
			fvm.NewTransactionInvocator(zerolog.Nop()), //we don't need to check signatures or sequence numbers
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

		eventWhitelist := event.GetServiceEventWhitelist()
		serviceEventA := cadence.Event{
			EventType: &cadence.EventType{
				Location: common.AddressLocation{
					Address: common.BytesToAddress(execCtx.Chain.ServiceAddress().Bytes()),
				},
				QualifiedIdentifier: eventWhitelist[rand.Intn(len(eventWhitelist))], //lets assume its not empty
			},
		}
		serviceEventB := cadence.Event{
			EventType: &cadence.EventType{
				Location: common.AddressLocation{
					Address: common.BytesToAddress(execCtx.Chain.ServiceAddress().Bytes()),
				},
				QualifiedIdentifier: eventWhitelist[rand.Intn(len(eventWhitelist))], //lets assume its not empty
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

		vm := fvm.New(emittingRuntime)

		exe, err := computer.NewBlockComputer(vm, execCtx, nil, nil, zerolog.Nop())
		require.NoError(t, err)

		//vm.On("Run", mock.Anything, mock.Anything, mock.Anything).
		//	Run(func(args mock.Arguments) {
		//
		//		tx := args[1].(*fvm.TransactionProcedure)
		//
		//
		//		tx.Err = &fvm.MissingPayerError{}
		//		tx.Events = events[txCount]
		//		txCount++
		//	}).
		//	Return(nil).
		//	Times(totalTransactionCount)

		view := delta.NewView(func(owner, controller, key string) (flow.RegisterValue, error) {
			return nil, nil
		})

		result, err := exe.ExecuteBlock(context.Background(), block, view, programs.NewEmptyPrograms())
		require.NoError(t, err)

		// all events should have been collected
		require.Len(t, result.ServiceEvents, 2)

		//events are ordered
		require.Equal(t, serviceEventA.EventType.ID(), string(result.ServiceEvents[0].Type))
		require.Equal(t, serviceEventB.EventType.ID(), string(result.ServiceEvents[1].Type))
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

		vm := fvm.New(rt)

		exe, err := computer.NewBlockComputer(vm, execCtx, nil, nil, zerolog.Nop())
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
				fvm.NewTransactionInvocator(logger),
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

		vm := fvm.New(rt)

		exe, err := computer.NewBlockComputer(vm, execCtx, nil, nil, zerolog.Nop())
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

type testRuntime struct {
	executeScript      func(runtime.Script, runtime.Context) (cadence.Value, error)
	executeTransaction func(runtime.Script, runtime.Context) error
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

func Test_FreezeAccountChecksAreIncluded(t *testing.T) {

	address := flow.HexToAddress("1234")
	fag := &FixedAddressGenerator{Address: address}

	rt := runtime.NewInterpreterRuntime()
	vm := fvm.New(rt)
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

	exe, err := computer.NewBlockComputer(vm, execCtx, nil, nil, zerolog.Nop())
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
			View: 42,
		},
		Payload: &flow.Payload{
			Guarantees: guarantees,
		},
	}

	return &entity.ExecutableBlock{
		Block:               &block,
		CompleteCollections: completeCollections,
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
