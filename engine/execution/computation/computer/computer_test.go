package computer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/dapperlabs/flow-go/engine/execution/computation/computer"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	vmmock "github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine/mock"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
)

func TestBlockExecutor_ExecuteBlock(t *testing.T) {

	t.Run("single collection", func(t *testing.T) {
		vm := new(vmmock.VirtualMachine)
		bc := new(vmmock.BlockContext)

		exe := computer.NewBlockComputer(vm)

		// create a block with 1 collection with 2 transactions
		block := generateBlock(1, 2)

		vm.On("NewBlockContext", &block.Block.Header).Return(bc)

		bc.On("ExecuteTransaction", mock.Anything, mock.Anything).
			Return(&virtualmachine.TransactionResult{}, nil).
			Twice()

		view := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		result, err := exe.ExecuteBlock(block, view)
		assert.NoError(t, err)
		assert.Len(t, result.StateViews, 1)

		vm.AssertExpectations(t)
		bc.AssertExpectations(t)
	})

	t.Run("multiple collections", func(t *testing.T) {
		vm := new(vmmock.VirtualMachine)
		bc := new(vmmock.BlockContext)

		exe := computer.NewBlockComputer(vm)

		collectionCount := 2
		transactionsPerCollection := 2
		totalTransactionCount := collectionCount * transactionsPerCollection

		// create a block with 2 collections with 2 transactions each
		block := generateBlock(collectionCount, transactionsPerCollection)

		vm.On("NewBlockContext", &block.Block.Header).Return(bc)

		bc.On("ExecuteTransaction", mock.Anything, mock.Anything).
			Return(&virtualmachine.TransactionResult{}, nil).
			Times(totalTransactionCount)

		view := state.NewView(func(key flow.RegisterID) (flow.RegisterValue, error) {
			return nil, nil
		})

		result, err := exe.ExecuteBlock(block, view)
		assert.NoError(t, err)

		//chunk count should match collection count
		assert.Len(t, result.StateViews, collectionCount)

		vm.AssertExpectations(t)
		bc.AssertExpectations(t)
	})
}

func generateBlock(collectionCount, transactionCount int) *entity.ExecutableBlock {
	collections := make([]*entity.CompleteCollection, collectionCount)
	guarantees := make([]*flow.CollectionGuarantee, collectionCount)
	completeCollections := make(map[flow.Identifier]*entity.CompleteCollection)

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

	return &entity.ExecutableBlock{
		Block:               &block,
		CompleteCollections: completeCollections,
	}
}

func generateCollection(transactionCount int) *entity.CompleteCollection {
	transactions := make([]*flow.TransactionBody, transactionCount)

	for i := 0; i < transactionCount; i++ {
		transactions[i] = &flow.TransactionBody{
			Script: []byte("transaction { execute {} }"),
		}
	}

	collection := flow.Collection{Transactions: transactions}

	guarantee := &flow.CollectionGuarantee{CollectionID: collection.ID()}

	return &entity.CompleteCollection{
		Guarantee:    guarantee,
		Transactions: transactions,
	}
}
