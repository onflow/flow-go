package execution

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution"
	executor "github.com/dapperlabs/flow-go/engine/execution/execution/executor/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
)

func TestExecutionEngine_OnCompleteBlock(t *testing.T) {
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
			Number: 42,
		},
		Content: flow.Content{
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

	t.Run("non-local engine", func(t *testing.T) {
		me := &module.Local{}
		me.On("NodeID").Return(flow.ZeroID)

		e := Engine{me: me}

		// submit using origin ID that does not match node ID
		err := e.onCompleteBlock(flow.Identifier{42}, completeBlock)
		assert.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		exec := &executor.BlockExecutor{}
		receipts := &network.Engine{}
		me := &module.Local{}
		me.On("NodeID").Return(flow.ZeroID)

		e := &Engine{
			receipts: receipts,
			executor: exec,
			me:       me,
		}

		receipts.On("SubmitLocal", mock.Anything)
		exec.On("ExecuteBlock", completeBlock).Return(&flow.ExecutionResult{}, nil)

		err := e.onCompleteBlock(e.me.NodeID(), completeBlock)
		require.NoError(t, err)

		receipts.AssertExpectations(t)
		exec.AssertExpectations(t)
	})
}
