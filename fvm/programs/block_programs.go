package programs

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/state"
)

// BlockPrograms is a simple fork-aware OCC database for "caching" programs
// for a particular block.
type BlockPrograms struct {
	*BlockDerivedData[common.AddressLocation, *interpreter.Program]
}

// TransactionPrograms is the scratch space for programs of a single transaction.
type TransactionPrograms struct {
	*TransactionDerivedData[common.AddressLocation, *interpreter.Program]
}

func NewEmptyBlockPrograms() *BlockPrograms {
	return &BlockPrograms{
		NewEmptyBlockDerivedData[common.AddressLocation, *interpreter.Program](),
	}
}

// This variant is needed by the chunk verifier, which does not start at the
// beginning of the block.
func NewEmptyBlockProgramsWithTransactionOffset(offset uint32) *BlockPrograms {
	return &BlockPrograms{
		NewEmptyBlockDerivedDataWithOffset[common.AddressLocation, *interpreter.Program](offset),
	}
}

func (block *BlockPrograms) NewChildBlockPrograms() *BlockPrograms {
	return &BlockPrograms{
		block.NewChildBlockDerivedData(),
	}
}

func (block *BlockPrograms) NewSnapshotReadTransactionPrograms(
	snapshotTime LogicalTime,
	executionTime LogicalTime,
) (
	*TransactionPrograms,
	error,
) {
	txn, err := block.NewSnapshotReadTransactionDerivedData(
		snapshotTime,
		executionTime)
	if err != nil {
		return nil, err
	}

	return &TransactionPrograms{
		TransactionDerivedData: txn,
	}, nil
}

func (block *BlockPrograms) NewTransactionPrograms(
	snapshotTime LogicalTime,
	executionTime LogicalTime,
) (
	*TransactionPrograms,
	error,
) {
	txn, err := block.NewTransactionDerivedData(snapshotTime, executionTime)
	if err != nil {
		return nil, err
	}

	return &TransactionPrograms{
		TransactionDerivedData: txn,
	}, nil
}

func (transaction *TransactionPrograms) Get(
	addressLocation common.AddressLocation,
) (
	*interpreter.Program,
	*state.State,
	bool,
) {
	return transaction.TransactionDerivedData.Get(addressLocation)
}

func (transaction *TransactionPrograms) Set(
	addressLocation common.AddressLocation,
	program *interpreter.Program,
	state *state.State,
) {
	transaction.TransactionDerivedData.Set(addressLocation, program, state)
}

func (transaction *TransactionPrograms) AddInvalidator(
	invalidator DerivedDataInvalidator[common.AddressLocation, *interpreter.Program],
) {
	transaction.TransactionDerivedData.AddInvalidator(invalidator)
}

func (transaction *TransactionPrograms) Validate() RetryableError {
	return transaction.TransactionDerivedData.Validate()
}

func (transaction *TransactionPrograms) Commit() RetryableError {
	return transaction.TransactionDerivedData.Commit()
}
