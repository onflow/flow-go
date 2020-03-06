package computation

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"

	"github.com/dapperlabs/flow-go/engine/execution"
	computer "github.com/dapperlabs/flow-go/engine/execution/computation/computer/mock"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/model/flow"
	module "github.com/dapperlabs/flow-go/module/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestExecutionEngine_ComputeBlock(t *testing.T) {
	tx1 := flow.TransactionBody{
		Script: []byte("transaction { execute {} }"),
	}

	tx2 := flow.TransactionBody{
		Script: []byte("transaction { execute {} }"),
	}

	transactions := []*flow.TransactionBody{&tx1, &tx2}

	col := flow.Collection{Transactions: transactions}

	guarantee := flow.CollectionGuarantee{
		CollectionID: col.ID(),
		Signatures:   nil,
	}

	block := flow.Block{
		Header: flow.Header{
			View: 42,
		},
		Payload: flow.Payload{
			Guarantees: []*flow.CollectionGuarantee{&guarantee},
		},
	}

	completeBlock := &execution.CompleteBlock{
		Block: &block,
		CompleteCollections: map[flow.Identifier]*execution.CompleteCollection{
			guarantee.ID(): {
				Guarantee:    &guarantee,
				Transactions: transactions,
			},
		},
	}

	computationResult := unittest.ComputationResultFixture(2)

	me := new(module.Local)
	me.On("NodeID").Return(flow.ZeroID)

	blockComputer := new(computer.BlockComputer)
	blockComputer.On("ExecuteBlock", mock.Anything, mock.Anything, mock.Anything).Return(computationResult, nil)

	engine := &Engine{
		blockComputer: blockComputer,
		me:            me,
	}

	view := state.NewView(func(key flow.RegisterID) (bytes []byte, e error) {
		return nil, nil
	})

	returnedComputationResult, err := engine.ComputeBlock(completeBlock, view)
	require.NoError(t, err)
	assert.Equal(t, computationResult, returnedComputationResult)
}
