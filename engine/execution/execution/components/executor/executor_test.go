package executor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	computer "github.com/dapperlabs/flow-go/engine/execution/execution/components/computer/mock"
	"github.com/dapperlabs/flow-go/engine/execution/execution/components/executor"
	"github.com/dapperlabs/flow-go/model/flow"
	storage "github.com/dapperlabs/flow-go/storage/mock"
)

func TestExecutorExecuteBlock(t *testing.T) {
	cols := &storage.Collections{}
	txs := &storage.Transactions{}
	computer := &computer.Computer{}

	exe := executor.New(cols, txs, computer)

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
				CollectionFingerprint: col.Fingerprint(),
				Signatures:            nil,
			},
		},
	}

	cols.On("ByFingerprint", col.Fingerprint()).
		Return(col, nil).
		Once()

	txs.On("ByFingerprint", tx1.Fingerprint()).
		Return(tx1, nil).
		Once()

	txs.On("ByFingerprint", tx2.Fingerprint()).
		Return(tx2, nil).
		Once()

	computer.On("ExecuteTransaction", mock.AnythingOfType("*flow.Transaction")).
		Return(nil, nil).
		Twice()

	chunks, err := exe.ExecuteBlock(block)
	assert.NoError(t, err)
	assert.Len(t, chunks, 1)

	chunk := chunks[0]
	assert.EqualValues(t, chunk.TxCounts, 2)

	cols.AssertExpectations(t)
	txs.AssertExpectations(t)
	computer.AssertExpectations(t)
}
