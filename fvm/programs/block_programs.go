package programs

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/state"
)

// DerivedBlockData is a simple fork-aware OCC database for "caching" derived
// data for a particular block.
type DerivedBlockData struct {
	*BlockDerivedData[common.AddressLocation, *interpreter.Program]
}

// DerivedTransactionData is the derived data scratch space for a single
// transaction.
type DerivedTransactionData struct {
	*TransactionDerivedData[common.AddressLocation, *interpreter.Program]
}

func NewEmptyDerivedBlockData() *DerivedBlockData {
	return &DerivedBlockData{
		NewEmptyBlockDerivedData[common.AddressLocation, *interpreter.Program](),
	}
}

// This variant is needed by the chunk verifier, which does not start at the
// beginning of the block.
func NewEmptyDerivedBlockDataWithTransactionOffset(offset uint32) *DerivedBlockData {
	return &DerivedBlockData{
		NewEmptyBlockDerivedDataWithOffset[common.AddressLocation, *interpreter.Program](offset),
	}
}

func (block *DerivedBlockData) NewChildDerivedBlockData() *DerivedBlockData {
	return &DerivedBlockData{
		block.NewChildBlockDerivedData(),
	}
}

func (block *DerivedBlockData) NewSnapshotReadDerivedTransactionData(
	snapshotTime LogicalTime,
	executionTime LogicalTime,
) (
	*DerivedTransactionData,
	error,
) {
	txn, err := block.NewSnapshotReadTransactionDerivedData(
		snapshotTime,
		executionTime)
	if err != nil {
		return nil, err
	}

	return &DerivedTransactionData{
		TransactionDerivedData: txn,
	}, nil
}

func (block *DerivedBlockData) NewDerivedTransactionData(
	snapshotTime LogicalTime,
	executionTime LogicalTime,
) (
	*DerivedTransactionData,
	error,
) {
	txn, err := block.NewTransactionDerivedData(snapshotTime, executionTime)
	if err != nil {
		return nil, err
	}

	return &DerivedTransactionData{
		TransactionDerivedData: txn,
	}, nil
}

func (transaction *DerivedTransactionData) GetProgram(
	addressLocation common.AddressLocation,
) (
	*interpreter.Program,
	*state.State,
	bool,
) {
	return transaction.TransactionDerivedData.Get(addressLocation)
}

func (transaction *DerivedTransactionData) SetProgram(
	addressLocation common.AddressLocation,
	program *interpreter.Program,
	state *state.State,
) {
	transaction.TransactionDerivedData.Set(addressLocation, program, state)
}

func (transaction *DerivedTransactionData) AddInvalidator(
	invalidator DerivedDataInvalidator[common.AddressLocation, *interpreter.Program],
) {
	transaction.TransactionDerivedData.AddInvalidator(invalidator)
}

func (transaction *DerivedTransactionData) Validate() RetryableError {
	return transaction.TransactionDerivedData.Validate()
}

func (transaction *DerivedTransactionData) Commit() RetryableError {
	return transaction.TransactionDerivedData.Commit()
}
