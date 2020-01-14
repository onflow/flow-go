package execution

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/execution/executor"
	executormock "github.com/dapperlabs/flow-go/engine/execution/execution/executor/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	network "github.com/dapperlabs/flow-go/network/mock"
)

func TestExecutionEngine_OnExecutableBlock(t *testing.T) {
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

	receipts.On("SubmitLocal", mock.AnythingOfType("*flow.ExecutionResult"))
	exec.On("ExecuteBlock", executableBlock).Return(&flow.ExecutionResult{}, nil)

	err := e.onExecutableBlock(executableBlock)
	require.NoError(t, err)

	receipts.AssertExpectations(t)
	exec.AssertExpectations(t)
}
