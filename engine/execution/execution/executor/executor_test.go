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

func TestBlockExecutorExecuteBlock(t *testing.T) {
	vm := &vmmock.VirtualMachine{}
	es := &statemock.ExecutionState{}

	exe := executor.NewBlockExecutor(vm, es)

	tx1 := &flow.Transaction{
		TransactionBody: flow.TransactionBody{
			Script: []byte("transaction { execute {} }"),
		},
	}

	tx2 := &flow.Transaction{
		TransactionBody: flow.TransactionBody{
			Script: []byte("transaction { execute {} }"),
		},
	}

	col := &flow.Collection{Transactions: []flow.Fingerprint{
		tx1.Fingerprint(),
		tx2.Fingerprint(),
	}}

	block := &flow.Block{
		Header: flow.Header{
			Number: 42,
		},
		GuaranteedCollections: []*flow.GuaranteedCollection{
			{
				CollectionHash: crypto.Hash(col.Fingerprint()),
				Signatures:     nil,
			},
		},
	}

	collections := []*flow.Collection{col}
	transactions := []*flow.Transaction{tx1, tx2}

	vm.On("SetBlock", block).
		Once()

	vm.On(
		"ExecuteTransaction",
		mock.AnythingOfType("*state.View"),
		mock.AnythingOfType("*flow.Transaction"),
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

	chunks, err := exe.ExecuteBlock(block, collections, transactions)
	assert.NoError(t, err)
	assert.Len(t, chunks, 1)

	chunk := chunks[0]
	assert.EqualValues(t, chunk.TxCounts, 2)

	vm.AssertExpectations(t)
	es.AssertExpectations(t)
}
