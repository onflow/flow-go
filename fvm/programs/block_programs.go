package programs

import (
	"fmt"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/state"
)

// DerivedBlockData is a simple fork-aware OCC database for "caching" derived
// data for a particular block.
type DerivedBlockData struct {
	programs *BlockDerivedData[common.AddressLocation, *interpreter.Program]

	meterParamOverrides *BlockDerivedData[struct{}, MeterParamOverrides]
}

// DerivedTransactionData is the derived data scratch space for a single
// transaction.
type DerivedTransactionData struct {
	programs *TransactionDerivedData[
		common.AddressLocation,
		*interpreter.Program,
	]

	// There's only a single entry in this table.  For simplicity, we'll use
	// struct{} as the entry's key.
	meterParamOverrides *TransactionDerivedData[struct{}, MeterParamOverrides]
}

func NewEmptyDerivedBlockData() *DerivedBlockData {
	return &DerivedBlockData{
		programs: NewEmptyBlockDerivedData[
			common.AddressLocation,
			*interpreter.Program,
		](),
		meterParamOverrides: NewEmptyBlockDerivedData[
			struct{},
			MeterParamOverrides,
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
		meterParamOverrides: NewEmptyBlockDerivedDataWithOffset[
			struct{},
			MeterParamOverrides,
		](offset),
	}
}

func (block *DerivedBlockData) NewChildDerivedBlockData() *DerivedBlockData {
	return &DerivedBlockData{
		programs:            block.programs.NewChildBlockDerivedData(),
		meterParamOverrides: block.meterParamOverrides.NewChildBlockDerivedData(),
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

	txnMeterParamOverrides, err := block.meterParamOverrides.NewSnapshotReadTransactionDerivedData(
		snapshotTime,
		executionTime)
	if err != nil {
		return nil, err
	}

	return &DerivedTransactionData{
		programs:            txnPrograms,
		meterParamOverrides: txnMeterParamOverrides,
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

	txnMeterParamOverrides, err := block.meterParamOverrides.NewTransactionDerivedData(
		snapshotTime,
		executionTime)
	if err != nil {
		return nil, err
	}

	return &DerivedTransactionData{
		programs:            txnPrograms,
		meterParamOverrides: txnMeterParamOverrides,
	}, nil
}

func (block *DerivedBlockData) NextTxIndexForTestingOnly() uint32 {
	// NOTE: We can use next tx index from any table since they are identical.
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
	transaction.meterParamOverrides.AddInvalidator(
		invalidator.MeterParamOverridesInvalidator())
}

func (transaction *DerivedTransactionData) GetMeterParamOverrides(
	txnState *state.TransactionState,
	getMeterParamOverrides ValueComputer[struct{}, MeterParamOverrides],
) (
	MeterParamOverrides,
	error,
) {
	return transaction.meterParamOverrides.GetOrCompute(
		txnState,
		struct{}{},
		getMeterParamOverrides)
}

func (transaction *DerivedTransactionData) Validate() error {
	err := transaction.programs.Validate()
	if err != nil {
		return fmt.Errorf("programs validate failed: %w", err)
	}

	err = transaction.meterParamOverrides.Validate()
	if err != nil {
		return fmt.Errorf("meter param overrides validate failed: %w", err)
	}

	return nil
}

func (transaction *DerivedTransactionData) Commit() error {
	err := transaction.programs.Commit()
	if err != nil {
		return fmt.Errorf("programs commit failed: %w", err)
	}

	err = transaction.meterParamOverrides.Commit()
	if err != nil {
		return fmt.Errorf("meter param overrides commit failed: %w", err)
	}

	return nil
}
