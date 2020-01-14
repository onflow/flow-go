package execution

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/execution/executor"
	executormock "github.com/dapperlabs/flow-go/engine/execution/execution/executor/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	storage "github.com/dapperlabs/flow-go/storage/mock"
)

func TestExecutionEngine_OnBlock(t *testing.T) {
	collections := &storage.Collections{}
	exec := &executormock.BlockExecutor{}

	e := &Engine{
		collections: collections,
		executor:    exec,
	}

	tx1 := flow.TransactionBody{
		Script: []byte("transaction { execute {} }"),
	}

	tx2 := flow.TransactionBody{
		Script: []byte("transaction { execute {} }"),
	}

	col := flow.Collection{Transactions: []flow.TransactionBody{tx1, tx2}}

	content := flow.Content{
		Guarantees: []*flow.CollectionGuarantee{
			{
				CollectionID: col.ID(),
				Signatures:   nil,
			},
		},
	}

	block := flow.Block{
		Header: flow.Header{
			Number: 42,
		},
		Content: content,
	}

	transactions := []flow.TransactionBody{tx1, tx2}

	executableBlock := executor.ExecutableBlock{
		Block:        block,
		Transactions: transactions,
	}

	collections.On("ByID", col.ID()).
		Return(&col, nil).
		Once()

	exec.On("ExecuteBlock", executableBlock).
		Return(&flow.ExecutionResult{}, nil).
		Once()

	err := e.onBlock(&block)
	require.NoError(t, err)

	collections.AssertExpectations(t)
	exec.AssertExpectations(t)
}
