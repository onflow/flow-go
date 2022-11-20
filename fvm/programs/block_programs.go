package programs

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/state"
)

// DerivedBlockData is a simple fork-aware OCC database for "caching" derived
// data for a particular block.
type DerivedBlockData struct {
	programs *BlockDerivedData[common.AddressLocation, *interpreter.Program]
}

// DerivedTransactionData is the derived data scratch space for a single
// transaction.
type DerivedTransactionData struct {
	programs *TransactionDerivedData[
		common.AddressLocation,
		*interpreter.Program,
	]
}

func NewEmptyDerivedBlockData() *DerivedBlockData {
	return &DerivedBlockData{
		programs: NewEmptyBlockDerivedData[
			common.AddressLocation,
			*interpreter.Program,
		](),
	}
}

// This variant is needed by the chunk verifier, which does not start at the
// beginning of the block.
func NewEmptyDerivedBlockDataWithTransactionOffset(offset uint32) *DerivedBlockData {
	return &DerivedBlockData{
		programs: NewEmptyBlockDerivedDataWithOffset[
			common.AddressLocation,
			*interpreter.Program,
		](offset),
	}
}

func (block *DerivedBlockData) NewChildDerivedBlockData() *DerivedBlockData {
	return &DerivedBlockData{
		programs: block.programs.NewChildBlockDerivedData(),
	}
}

func (block *DerivedBlockData) NewSnapshotReadDerivedTransactionData(
	snapshotTime LogicalTime,
	executionTime LogicalTime,
) (
	*DerivedTransactionData,
	error,
) {
	txnPrograms, err := block.programs.NewSnapshotReadTransactionDerivedData(
		snapshotTime,
		executionTime)
	if err != nil {
		return nil, err
	}

	return &DerivedTransactionData{
		programs: txnPrograms,
	}, nil
}

func (block *DerivedBlockData) NewDerivedTransactionData(
	snapshotTime LogicalTime,
	executionTime LogicalTime,
) (
	*DerivedTransactionData,
	error,
) {
	txnPrograms, err := block.programs.NewTransactionDerivedData(
		snapshotTime,
		executionTime)
	if err != nil {
		return nil, err
	}

	return &DerivedTransactionData{
		programs: txnPrograms,
	}, nil
}

func (block *DerivedBlockData) NextTxIndexForTestingOnly() uint32 {
	return block.programs.NextTxIndexForTestingOnly()
}

func (block *DerivedBlockData) GetProgramForTestingOnly(
	addressLocation common.AddressLocation,
) *invalidatableEntry[*interpreter.Program] {
	return block.programs.GetForTestingOnly(addressLocation)
}

func (transaction *DerivedTransactionData) GetProgram(
	addressLocation common.AddressLocation,
) (
	*interpreter.Program,
	*state.State,
	bool,
) {
	return transaction.programs.Get(addressLocation)
}

func (transaction *DerivedTransactionData) SetProgram(
	addressLocation common.AddressLocation,
	program *interpreter.Program,
	state *state.State,
) {
	transaction.programs.Set(addressLocation, program, state)
}

func (transaction *DerivedTransactionData) AddInvalidator(
	invalidator TransactionInvalidator,
) {
	transaction.programs.AddInvalidator(invalidator.ProgramInvalidator())
}

func (transaction *DerivedTransactionData) Validate() RetryableError {
	return transaction.programs.Validate()
}

func (transaction *DerivedTransactionData) Commit() RetryableError {
	return transaction.programs.Commit()
}
