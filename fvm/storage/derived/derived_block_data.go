package derived

import (
	"fmt"

	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/storage/logical"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/storage/state"
)

type DerivedTransactionPreparer interface {
	GetOrComputeProgram(
		txState state.NestedTransactionPreparer,
		addressLocation common.AddressLocation,
		programComputer ValueComputer[common.AddressLocation, *Program],
	) (
		*Program,
		error,
	)
	GetProgram(location common.AddressLocation) (*Program, bool)

	// GetStateExecutionParameters returns parameters needed for execution from the state.
	GetStateExecutionParameters(
		txnState state.NestedTransactionPreparer,
		getMeterParamOverrides ValueComputer[struct{}, StateExecutionParameters],
	) (
		StateExecutionParameters,
		*snapshot.ExecutionSnapshot,
		error,
	)

	AddInvalidator(invalidator TransactionInvalidator)
}

type Program struct {
	*runtime.Program

	Dependencies ProgramDependencies
}

// DerivedBlockData is a simple fork-aware OCC database for "caching" derived
// data for a particular block.
type DerivedBlockData struct {
	programs *DerivedDataTable[common.AddressLocation, *Program]

	meterParamOverrides *DerivedDataTable[struct{}, StateExecutionParameters]
}

// DerivedTransactionData is the derived data scratch space for a single
// transaction.
type DerivedTransactionData struct {
	programs *TableTransaction[
		common.AddressLocation,
		*Program,
	]

	// There's only a single entry in this table.  For simplicity, we'll use
	// struct{} as the entry's key.
	executionParameters *TableTransaction[struct{}, StateExecutionParameters]
}

func NewEmptyDerivedBlockData(
	initialSnapshotTime logical.Time,
) *DerivedBlockData {
	return &DerivedBlockData{
		programs: NewEmptyTable[
			common.AddressLocation,
			*Program,
		](initialSnapshotTime),
		meterParamOverrides: NewEmptyTable[
			struct{},
			StateExecutionParameters,
		](initialSnapshotTime),
	}
}

func (block *DerivedBlockData) NewChildDerivedBlockData() *DerivedBlockData {
	return &DerivedBlockData{
		programs:            block.programs.NewChildTable(),
		meterParamOverrides: block.meterParamOverrides.NewChildTable(),
	}
}

func (block *DerivedBlockData) NewSnapshotReadDerivedTransactionData() *DerivedTransactionData {
	return &DerivedTransactionData{
		programs:            block.programs.NewSnapshotReadTableTransaction(),
		executionParameters: block.meterParamOverrides.NewSnapshotReadTableTransaction(),
	}
}

func (block *DerivedBlockData) NewCachingSnapshotReadDerivedTransactionData() *DerivedTransactionData {
	return &DerivedTransactionData{
		programs:            block.programs.NewCachingSnapshotReadTableTransaction(),
		executionParameters: block.meterParamOverrides.NewCachingSnapshotReadTableTransaction(),
	}
}

func (block *DerivedBlockData) NewDerivedTransactionData(
	snapshotTime logical.Time,
	executionTime logical.Time,
) (
	*DerivedTransactionData,
	error,
) {
	txnPrograms, err := block.programs.NewTableTransaction(
		snapshotTime,
		executionTime)
	if err != nil {
		return nil, err
	}

	txnMeterParamOverrides, err := block.meterParamOverrides.NewTableTransaction(
		snapshotTime,
		executionTime)
	if err != nil {
		return nil, err
	}

	return &DerivedTransactionData{
		programs:            txnPrograms,
		executionParameters: txnMeterParamOverrides,
	}, nil
}

func (block *DerivedBlockData) NextTxIndexForTestingOnly() uint32 {
	// NOTE: We can use next tx index from any table since they are identical.
	return block.programs.NextTxIndexForTestingOnly()
}

func (block *DerivedBlockData) GetProgramForTestingOnly(
	addressLocation common.AddressLocation,
) *invalidatableEntry[*Program] {
	return block.programs.GetForTestingOnly(addressLocation)
}

// CachedPrograms returns the number of programs cached.
// Note: this should only be called after calling commit, otherwise
// the count will contain invalidated entries.
func (block *DerivedBlockData) CachedPrograms() int {
	return len(block.programs.items)
}

func (transaction *DerivedTransactionData) GetOrComputeProgram(
	txState state.NestedTransactionPreparer,
	addressLocation common.AddressLocation,
	programComputer ValueComputer[common.AddressLocation, *Program],
) (
	*Program,
	error,
) {
	return transaction.programs.GetOrCompute(
		txState,
		addressLocation,
		programComputer)
}

// GetProgram returns the program for the given address location.
// This does NOT apply reads/metering to any nested transaction.
// Use with caution!
func (transaction *DerivedTransactionData) GetProgram(
	location common.AddressLocation,
) (
	*Program,
	bool,
) {
	program, _, ok := transaction.programs.get(location)
	return program, ok
}

func (transaction *DerivedTransactionData) AddInvalidator(
	invalidator TransactionInvalidator,
) {
	if invalidator == nil {
		return
	}

	transaction.programs.AddInvalidator(invalidator.ProgramInvalidator())
	transaction.executionParameters.AddInvalidator(
		invalidator.ExecutionParametersInvalidator())
}

func (transaction *DerivedTransactionData) GetStateExecutionParameters(
	txnState state.NestedTransactionPreparer,
	getMeterParamOverrides ValueComputer[struct{}, StateExecutionParameters],
) (
	StateExecutionParameters,
	*snapshot.ExecutionSnapshot,
	error,
) {
	return transaction.executionParameters.GetWithStateOrCompute(
		txnState,
		struct{}{},
		getMeterParamOverrides)
}

func (transaction *DerivedTransactionData) Validate() error {
	err := transaction.programs.Validate()
	if err != nil {
		return fmt.Errorf("programs validate failed: %w", err)
	}

	err = transaction.executionParameters.Validate()
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

	err = transaction.executionParameters.Commit()
	if err != nil {
		return fmt.Errorf("meter param overrides commit failed: %w", err)
	}

	return nil
}
