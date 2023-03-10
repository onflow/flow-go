package testutils

import (
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm/derived"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/storage"
)

type SimpleTestTransaction struct {
	*delta.View

	storage.SerialTransaction
}

// NewSimpleTransaction returns a transaction which can be used to test
// fvm evaluation.  The returned transaction should not be committed.
func NewSimpleTransaction(
	snapshot state.StorageSnapshot,
) *SimpleTestTransaction {
	view := delta.NewDeltaView(snapshot)

	derivedBlockData := derived.NewEmptyDerivedBlockData()
	derivedTxnData, err := derivedBlockData.NewDerivedTransactionData(0, 0)
	if err != nil {
		panic(err)
	}

	return &SimpleTestTransaction{
		View: view,
		SerialTransaction: storage.SerialTransaction{
			NestedTransaction: state.NewTransactionState(
				view,
				state.DefaultParameters()),
			DerivedTransactionCommitter: derivedTxnData,
		},
	}
}
