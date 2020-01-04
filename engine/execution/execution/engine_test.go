package execution

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	executor "github.com/dapperlabs/flow-go/engine/execution/execution/components/executor/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	storage "github.com/dapperlabs/flow-go/storage/mock"
)

func TestExecutionEngine_OnBlock(t *testing.T) {
	collections := &storage.Collections{}
	transactions := &storage.Transactions{}
	exec := &executor.Executor{}

	e := &Engine{
		collections:  collections,
		transactions: transactions,
		executor:     exec,
	}

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

	txs := []*flow.Transaction{tx1, tx2}
	cols := []*flow.Collection{col}

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

	collections.On("ByFingerprint", col.Fingerprint()).
		Return(col, nil).
		Once()

	transactions.On("ByFingerprint", tx1.Fingerprint()).
		Return(tx1, nil).
		Once()

	transactions.On("ByFingerprint", tx2.Fingerprint()).
		Return(tx2, nil).
		Once()

	exec.On("ExecuteBlock", block, cols, txs).
		Return(nil, nil).
		Once()

	err := e.onBlock(block)
	require.NoError(t, err)

	collections.AssertExpectations(t)
	transactions.AssertExpectations(t)
	exec.AssertExpectations(t)
}
