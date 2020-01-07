package executor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/dapperlabs/flow-go/crypto"
	computer "github.com/dapperlabs/flow-go/engine/execution/execution/components/computer/mock"
	"github.com/dapperlabs/flow-go/engine/execution/execution/components/executor"
	"github.com/dapperlabs/flow-go/model/flow"
)

func TestExecutorExecuteBlock(t *testing.T) {
	computer := &computer.Computer{}

	exe := executor.New(computer)

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

	computer.On("ExecuteTransaction", mock.AnythingOfType("flow.TransactionBody")).
		Return(nil, nil).
		Twice()

	chunks, err := exe.ExecuteBlock(executableBlock)
	assert.NoError(t, err)
	assert.Len(t, chunks, 1)

	chunk := chunks[0]
	assert.EqualValues(t, chunk.TxCounts, 2)

	computer.AssertExpectations(t)
}
