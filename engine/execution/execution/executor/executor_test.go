package executor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/dapperlabs/flow-go/engine/execution/execution/executor"
	"github.com/dapperlabs/flow-go/engine/execution/execution/state"
	statemock "github.com/dapperlabs/flow-go/engine/execution/execution/state/mock"
	"github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine"
	vmmock "github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestBlockExecutor_ExecuteBlock(t *testing.T) {
	vm := &vmmock.VirtualMachine{}
	bc := &vmmock.BlockContext{}
	es := &statemock.ExecutionState{}

	exe := executor.NewBlockExecutor(vm, es)

	tx1 := flow.TransactionBody{
		Script: []byte("transaction { execute {} }"),
	}

	tx2 := flow.TransactionBody{
		Script: []byte("transaction { execute {} }"),
	}

	col := flow.Collection{Transactions: []flow.TransactionBody{tx1, tx2}}

	guarantee := flow.CollectionGuarantee{
		CollectionID: col.ID(),
		Signatures:   nil,
	}

	content := flow.Content{
		Guarantees: []*flow.CollectionGuarantee{&guarantee},
	}

	block := flow.Block{
		Header: flow.Header{
			Number: 42,
		},
		Content: content,
	}

	transactions := []*flow.TransactionBody{&tx1, &tx2}

	executableBlock := &executor.ExecutableBlock{
		Block: &block,
		Collections: []*executor.ExecutableCollection{
			{
				Guarantee:    &guarantee,
				Transactions: transactions,
			},
		},
		PreviousResultID: flow.ZeroID,
	}

	vm.On("NewBlockContext", &block).Return(bc)

	bc.On(
		"ExecuteTransaction",
		mock.AnythingOfType("*state.View"),
		mock.AnythingOfType("*flow.TransactionBody"),
	).
		Return(&virtualmachine.TransactionResult{}, nil).
		Twice()

	es.On("StateCommitmentByBlockID", block.ParentID).
		Return(unittest.StateCommitmentFixture(), nil)

	es.On(
		"NewView",
		mock.AnythingOfType("[]uint8"),
	).
		Return(
			state.NewView(func(key string) (bytes []byte, e error) {
				return nil, nil
			}))

	es.On(
		"CommitDelta",
		mock.AnythingOfType("state.Delta"),
	).
		Return(nil, nil)

	es.On("PersistStateCommitment",
		block.ID(), mock.AnythingOfType("*[]uint8")).Return(nil)

	result, err := exe.ExecuteBlock(executableBlock)
	assert.NoError(t, err)
	assert.Len(t, result.Chunks.Chunks, 1)

	chunk := result.Chunks.Chunks[0]
	assert.EqualValues(t, 0, chunk.CollectionIndex)

	vm.AssertExpectations(t)
	bc.AssertExpectations(t)
	es.AssertExpectations(t)
}
