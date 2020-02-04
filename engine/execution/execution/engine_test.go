package execution

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution"
	executor "github.com/dapperlabs/flow-go/engine/execution/execution/executor/mock"
	realState "github.com/dapperlabs/flow-go/engine/execution/execution/state"
	"github.com/dapperlabs/flow-go/model/flow"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
)

func TestExecutionEngine_OnExecutableBlock(t *testing.T) {
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

	t.Run("non-local engine", func(t *testing.T) {
		me := new(module.Local)
		me.On("NodeID").Return(flow.ZeroID)

		e := Engine{me: me}

		view := realState.NewView(func(key string) (bytes []byte, e error) {
			return nil, nil
		})

		// submit using origin ID that does not match node ID
		err := e.onCompleteBlock(flow.Identifier{42}, completeBlock, view)
		assert.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		exec := new(executor.BlockExecutor)
		receipts := new(network.Engine)
		me := new(module.Local)
		me.On("NodeID").Return(flow.ZeroID)

		e := &Engine{
			provider: receipts,
			executor: exec,
			me:       me,
		}

		receipts.On("SubmitLocal", mock.Anything)
		exec.On("ExecuteBlock", completeBlock).Return(&flow.ExecutionResult{}, nil)

		view := realState.NewView(func(key string) (bytes []byte, e error) {
			return nil, nil
		})

		err := e.onCompleteBlock(e.me.NodeID(), completeBlock, view)
		require.NoError(t, err)

		receipts.AssertExpectations(t)
		exec.AssertExpectations(t)
	})
}

