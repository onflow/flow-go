package executor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/engine/execution/execution/executor"
	"github.com/dapperlabs/flow-go/engine/execution/execution/state"
	statemock "github.com/dapperlabs/flow-go/engine/execution/execution/state/mock"
	"github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine"
	vmmock "github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestBlockExecutor_ExecuteBlock(t *testing.T) {

	t.Run("single collection", func(t *testing.T) {
		vm := new(vmmock.VirtualMachine)
		bc := new(vmmock.BlockContext)
		es := new(statemock.ExecutionState)

		exe := executor.NewBlockExecutor(vm, es)

		// create a block with 1 collection with 2 transactions
		block := generateBlock(1, 2)

		vm.On("NewBlockContext", &block.Block.Header).Return(bc)

		bc.On("ExecuteTransaction", mock.Anything, mock.Anything).
			Return(&virtualmachine.TransactionResult{}, nil).
			Twice()

		es.On("StateCommitmentByBlockID", block.Block.ParentID).
			Return(unittest.StateCommitmentFixture(), nil)

		es.On("NewView", mock.Anything).
			Return(
				state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
					return nil, nil
				}))

		es.On("CommitDelta", mock.Anything).Return(nil, nil)
		es.On("PersistChunkHeader", mock.Anything, mock.Anything).Return(nil)
		es.On("PersistStateCommitment", block.Block.ID(), mock.Anything).Return(nil)

		result, err := exe.ExecuteBlock(block)
		assert.NoError(t, err)
		assert.Len(t, result.Chunks, 1)

		chunk := result.Chunks[0]
		assert.EqualValues(t, 0, chunk.CollectionIndex)

		vm.AssertExpectations(t)
		bc.AssertExpectations(t)
		es.AssertExpectations(t)
	})

	t.Run("multiple collections", func(t *testing.T) {
		vm := new(vmmock.VirtualMachine)
		bc := new(vmmock.BlockContext)
		es := new(statemock.ExecutionState)

		exe := executor.NewBlockExecutor(vm, es)

		collectionCount := 2
		transactionsPerCollection := 2
		totalTransactionCount := collectionCount * transactionsPerCollection

		// create a block with 2 collections with 2 transactions each
		block := generateBlock(collectionCount, transactionsPerCollection)

		vm.On("NewBlockContext", &block.Block.Header).Return(bc)

		bc.On("ExecuteTransaction", mock.Anything, mock.Anything).
			Return(&virtualmachine.TransactionResult{}, nil).
			Times(totalTransactionCount)

		es.On("StateCommitmentByBlockID", block.Block.ParentID).
			Return(unittest.StateCommitmentFixture(), nil)

		es.On("NewView", mock.Anything).
			Return(
				state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
					return nil, nil
				})).
			Times(collectionCount)

		es.On("CommitDelta", mock.Anything).
			Return(nil, nil).
			Times(collectionCount)

		es.On("PersistChunkHeader", mock.Anything, mock.Anything).
			Return(nil).
			Times(collectionCount)

		es.On("PersistStateCommitment", block.Block.ID(), mock.Anything).
			Return(nil)

		result, err := exe.ExecuteBlock(block)
		assert.NoError(t, err)

		// chunk count should match collection count
		assert.Len(t, result.Chunks, collectionCount)

		// chunks should follow the same order as collections
		for i, chunk := range result.Chunks {
			assert.EqualValues(t, i, chunk.CollectionIndex)
		}

		vm.AssertExpectations(t)
		bc.AssertExpectations(t)
		es.AssertExpectations(t)
	})
}

func generateBlock(collectionCount, transactionCount int) *execution.CompleteBlock {
	collections := make([]*execution.CompleteCollection, collectionCount)
	guarantees := make([]*flow.CollectionGuarantee, collectionCount)
	completeCollections := make(map[flow.Identifier]*execution.CompleteCollection)

	for i := 0; i < collectionCount; i++ {
		collection := generateCollection(transactionCount)
		collections[i] = collection
		guarantees[i] = collection.Guarantee
		completeCollections[collection.Guarantee.ID()] = collection
	}

	block := flow.Block{
		Header: flow.Header{
			View: 42,
		},
		Payload: flow.Payload{
			Guarantees: guarantees,
		},
	}

	return &execution.CompleteBlock{
		Block:               &block,
		CompleteCollections: completeCollections,
	}
}

func generateCollection(transactionCount int) *execution.CompleteCollection {
	transactions := make([]*flow.TransactionBody, transactionCount)

	for i := 0; i < transactionCount; i++ {
		transactions[i] = &flow.TransactionBody{
			Script: []byte("transaction { execute {} }"),
		}
	}

	collection := flow.Collection{Transactions: transactions}

	guarantee := &flow.CollectionGuarantee{CollectionID: collection.ID()}

	return &execution.CompleteCollection{
		Guarantee:    guarantee,
		Transactions: transactions,
	}
}
