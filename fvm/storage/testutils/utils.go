package testutils

import (
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/derived"
)

// NewSimpleTransaction returns a transaction which can be used to test
// fvm evaluation.  The returned transaction should not be committed.
func NewSimpleTransaction(
	snapshot state.StorageSnapshot,
) *storage.SerialTransaction {
	derivedBlockData := derived.NewEmptyDerivedBlockData()
	derivedTxnData, err := derivedBlockData.NewDerivedTransactionData(0, 0)
	if err != nil {
		panic(err)
	}

	return &storage.SerialTransaction{
		NestedTransaction: state.NewTransactionState(
			delta.NewDeltaView(snapshot),
			state.DefaultParameters()),
		DerivedTransactionCommitter: derivedTxnData,
	}
}
