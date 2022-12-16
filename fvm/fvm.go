package fvm

import (
	"context"
	"fmt"

	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/derived"
	"github.com/onflow/flow-go/fvm/environment"
	errors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

type ProcedureType string

const (
	BootstrapProcedureType   = ProcedureType("bootstrap")
	TransactionProcedureType = ProcedureType("transaction")
	ScriptProcedureType      = ProcedureType("script")
)

type ProcedureExecutor interface {
	Preprocess() error
	Execute() error
	Cleanup()
}

func Run(executor ProcedureExecutor) error {
	defer executor.Cleanup()

	err := executor.Preprocess()
	if err != nil {
		return err
	}

	return executor.Execute()
}

// An Procedure is an operation (or set of operations) that reads or writes ledger state.
type Procedure interface {
	NewExecutor(
		ctx Context,
		txnState *state.TransactionState,
		derivedTxnData *derived.DerivedTransactionData,
	) ProcedureExecutor

	ComputationLimit(ctx Context) uint64
	MemoryLimit(ctx Context) uint64
	ShouldDisableMemoryAndInteractionLimits(ctx Context) bool

	Type() ProcedureType

	// The initial snapshot time is used as part of OCC validation to ensure
	// there are no read-write conflict amongst transactions.  Note that once
	// we start supporting parallel preprocessing/execution, a transaction may
	// operation on mutliple snapshots.
	//
	// For scripts, since they can only be executed after the block has been
	// executed, the initial snapshot time is EndOfBlockExecutionTime.
	InitialSnapshotTime() derived.LogicalTime

	// For transactions, the execution time is TxIndex.  For scripts, the
	// execution time is EndOfBlockExecutionTime.
	ExecutionTime() derived.LogicalTime
}

func NewInterpreterRuntime(config runtime.Config) runtime.Runtime {
	return runtime.NewInterpreterRuntime(config)
}

// A VirtualMachine augments the Cadence runtime with Flow host functionality.
type VirtualMachine struct {
}

func NewVirtualMachine() *VirtualMachine {
	return &VirtualMachine{}
}

// Run runs a procedure against a ledger in the given context.
func (vm *VirtualMachine) Run(
	ctx Context,
	proc Procedure,
	v state.View,
) error {
	derivedBlockData := ctx.DerivedBlockData
	if derivedBlockData == nil {
		derivedBlockData = derived.NewEmptyDerivedBlockDataWithTransactionOffset(
			uint32(proc.ExecutionTime()))
	}

	var derivedTxnData *derived.DerivedTransactionData
	var err error
	switch proc.Type() {
	case ScriptProcedureType:
		derivedTxnData, err = derivedBlockData.NewSnapshotReadDerivedTransactionData(
			proc.InitialSnapshotTime(),
			proc.ExecutionTime())
	case TransactionProcedureType, BootstrapProcedureType:
		derivedTxnData, err = derivedBlockData.NewDerivedTransactionData(
			proc.InitialSnapshotTime(),
			proc.ExecutionTime())
	default:
		return fmt.Errorf("invalid proc type: %v", proc.Type())
	}

	if err != nil {
		return fmt.Errorf("error creating derived transaction data: %w", err)
	}

	txnState := state.NewTransactionState(
		v,
		state.DefaultParameters().
			WithMeterParameters(getBasicMeterParameters(ctx, proc)).
			WithMaxKeySizeAllowed(ctx.MaxStateKeySize).
			WithMaxValueSizeAllowed(ctx.MaxStateValueSize))

	err = Run(proc.NewExecutor(ctx, txnState, derivedTxnData))
	if err != nil {
		return err
	}

	// Note: it is safe to skip committing derived data for non-normal
	// transactions (i.e., bootstrap and script) since these do not invalidate
	// derived data entries.
	if proc.Type() == TransactionProcedureType {
		// NOTE: It is not safe to ignore derivedTxnData' commit error for
		// transactions that trigger derived data invalidation.
		return derivedTxnData.Commit()
	}

	return nil
}

// GetAccount returns an account by address or an error if none exists.
func (vm *VirtualMachine) GetAccount(
	ctx Context,
	address flow.Address,
	v state.View,
) (
	*flow.Account,
	error,
) {
	txnState := state.NewTransactionState(
		v,
		state.DefaultParameters().
			WithMaxKeySizeAllowed(ctx.MaxStateKeySize).
			WithMaxValueSizeAllowed(ctx.MaxStateValueSize).
			WithMeterParameters(
				meter.DefaultParameters().
					WithStorageInteractionLimit(ctx.MaxStateInteractionSize)))

	derivedBlockData := ctx.DerivedBlockData
	if derivedBlockData == nil {
		derivedBlockData = derived.NewEmptyDerivedBlockData()
	}

	derviedTxnData, err := derivedBlockData.NewSnapshotReadDerivedTransactionData(
		derived.EndOfBlockExecutionTime,
		derived.EndOfBlockExecutionTime)
	if err != nil {
		return nil, fmt.Errorf(
			"error creating derived transaction data for GetAccount: %w",
			err)
	}

	env := environment.NewScriptEnvironment(
		context.Background(),
		ctx.EnvironmentParams,
		txnState,
		derviedTxnData)
	account, err := env.GetAccount(address)
	if err != nil {
		if errors.IsALedgerFailure(err) {
			return nil, fmt.Errorf(
				"cannot get account, this error usually happens if the "+
					"reference block for this query is not set to a recent "+
					"block: %w",
				err)
		}
		return nil, fmt.Errorf("cannot get account: %w", err)
	}
	return account, nil
}
