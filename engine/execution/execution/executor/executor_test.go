package executor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/execution/execution/executor"
	"github.com/dapperlabs/flow-go/engine/execution/execution/state"
	statemock "github.com/dapperlabs/flow-go/engine/execution/execution/state/mock"
	"github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine"
	vmmock "github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine/mock"
	"github.com/dapperlabs/flow-go/model/flow"
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

	block := flow.Block{
		Header: flow.Header{
			Number: 42,
		},
		CollectionGuarantees: []*flow.CollectionGuarantee{
			{
				Hash:       crypto.Hash(col.Fingerprint()),
				Signatures: nil,
			},
		},
	}

	transactions := []flow.TransactionBody{tx1, tx2}

	executableBlock := executor.ExecutableBlock{
		Block:        block,
		Transactions: transactions,
	}

	vm.On("NewBlockContext", &block).
		Return(bc).
		Once()

	bc.On(
		"ExecuteTransaction",
		mock.AnythingOfType("*state.View"),
		mock.AnythingOfType("flow.TransactionBody"),
	).
		Return(&virtualmachine.TransactionResult{}, nil).
		Twice()

	es.On(
		"NewView",
		mock.AnythingOfType("flow.StateCommitment"),
	).
		Return(
			state.NewView(func(key string) (bytes []byte, e error) {
				return nil, nil
			})).
		Once()

	es.On(
		"CommitDelta",
		mock.AnythingOfType("state.Delta"),
	).
		Return(nil, nil).
		Once()

	chunks, err := exe.ExecuteBlock(executableBlock)
	assert.NoError(t, err)
	assert.Len(t, chunks, 1)

	chunk := chunks[0]
	assert.EqualValues(t, chunk.TxCounts, 2)

	vm.AssertExpectations(t)
	bc.AssertExpectations(t)
	es.AssertExpectations(t)
}
