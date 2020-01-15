package execution

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution"
	executormock "github.com/dapperlabs/flow-go/engine/execution/execution/executor/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	network "github.com/dapperlabs/flow-go/network/mock"
)

func TestExecutionEngine_OnCompleteBlock(t *testing.T) {
	exec := &executormock.BlockExecutor{}
	receipts := &network.Engine{}

	e := &Engine{
		receipts: receipts,
		executor: exec,
	}

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

	content := flow.Content{
		Guarantees: []*flow.CollectionGuarantee{&guarantee},
	}

	block := flow.Block{
		Header: flow.Header{
			Number: 42,
		},
		Content: content,
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

	receipts.On("SubmitLocal", mock.Anything)
	exec.On("ExecuteBlock", completeBlock).Return(&flow.ExecutionResult{}, nil)

	err := e.onCompleteBlock(completeBlock)
	require.NoError(t, err)

	receipts.AssertExpectations(t)
	exec.AssertExpectations(t)
}
