package storage

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/logical"
	"github.com/onflow/flow-go/fvm/storage/primary"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/storage/state"
)

// BlockDatabase packages the primary index (BlockData) and secondary indices
// (DerivedBlockData) into a single database via 2PC.
type BlockDatabase struct {
	*primary.BlockData
	*derived.DerivedBlockData
}

type transaction struct {
	*primary.TransactionData
	*derived.DerivedTransactionData
}

// NOTE: storageSnapshot must be thread safe.
func NewBlockDatabase(
	storageSnapshot snapshot.StorageSnapshot,
	snapshotTime logical.Time,
	cachedDerivedBlockData *derived.DerivedBlockData, // optional
) *BlockDatabase {
	derivedBlockData := cachedDerivedBlockData
	if derivedBlockData == nil {
		derivedBlockData = derived.NewEmptyDerivedBlockData(snapshotTime)
	}

	return &BlockDatabase{
		BlockData:        primary.NewBlockData(storageSnapshot, snapshotTime),
		DerivedBlockData: derivedBlockData,
	}
}

func (database *BlockDatabase) NewTransaction(
	executionTime logical.Time,
	parameters state.StateParameters,
) (
	Transaction,
	error,
) {
	primaryTxn, err := database.BlockData.NewTransactionData(
		executionTime,
		parameters)
	if err != nil {
		return nil, fmt.Errorf("failed to create primary transaction: %w", err)
	}

	derivedTxn, err := database.DerivedBlockData.NewDerivedTransactionData(
		primaryTxn.SnapshotTime(),
		executionTime)
	if err != nil {
		return nil, fmt.Errorf("failed to create dervied transaction: %w", err)
	}

	return &transaction{
		TransactionData:        primaryTxn,
		DerivedTransactionData: derivedTxn,
	}, nil
}

func (database *BlockDatabase) NewSnapshotReadTransaction(
	parameters state.StateParameters,
) Transaction {

	return &transaction{
		TransactionData: database.BlockData.
			NewSnapshotReadTransactionData(parameters),
		DerivedTransactionData: database.DerivedBlockData.
			NewSnapshotReadDerivedTransactionData(),
	}
}

func (txn *transaction) Validate() error {
	err := txn.DerivedTransactionData.Validate()
	if err != nil {
		return fmt.Errorf("derived indices validate failed: %w", err)
	}

	// NOTE: Since the primary txn's SnapshotTime() is exposed to the user,
	// the primary txn should be validated last to prevent primary txn'
	// snapshot time advancement in case of derived txn validation failure.
	err = txn.TransactionData.Validate()
	if err != nil {
		return fmt.Errorf("primary index validate failed: %w", err)
	}

	return nil
}

func (txn *transaction) Finalize() error {
	// NOTE: DerivedTransactionData does not need to be finalized.
	return txn.TransactionData.Finalize()
}

func (txn *transaction) Commit() (*snapshot.ExecutionSnapshot, error) {
	err := txn.DerivedTransactionData.Commit()
	if err != nil {
		return nil, fmt.Errorf("derived indices commit failed: %w", err)
	}

	// NOTE: Since the primary txn's SnapshotTime() is exposed to the user,
	// the primary txn should be committed last to prevent primary txn'
	// snapshot time advancement in case of derived txn commit failure.
	executionSnapshot, err := txn.TransactionData.Commit()
	if err != nil {
		return nil, fmt.Errorf("primary index commit failed: %w", err)
	}

	return executionSnapshot, nil
}
