package fvm

import (
	"fmt"
	"math"

	"github.com/onflow/cadence/runtime"

	errors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

// An Procedure is an operation (or set of operations) that reads or writes ledger state.
type Procedure interface {
	Run(vm *VirtualMachine, ctx Context, sth *state.StateHolder, txnProgs *programs.TransactionPrograms) error
	ComputationLimit(ctx Context) uint64
	MemoryLimit(ctx Context) uint64
	ShouldDisableMemoryAndInteractionLimits(ctx Context) bool

	IsSnapshotReadTransaction() bool
	IsBootstrapping() bool

	// The initial snapshot time.  Note that once we start supporting parallel
	// preprocessing/execution, a transaction may operation on mutliple
	// snapshots.  For scripts, the snapshot time is EndOfBlockExecutionTime.
	InitialSnapshotTime() programs.LogicalTime

	// For transactions, the execution time is TxIndex.  For scripts, the
	// execution time is EndOfBlockExecutionTime.
	ExecutionTime() programs.LogicalTime
}

func NewInterpreterRuntime(config runtime.Config) runtime.Runtime {
	return runtime.NewInterpreterRuntime(config)
}

// A VirtualMachine augments the Cadence runtime with Flow host functionality.
type VirtualMachine struct {
	// TODO(patrick): move this into ReusableCadenceRuntime
	Runtime runtime.Runtime
}

// NewVirtualMachine creates a new virtual machine instance with the provided runtime.
func NewVirtualMachine(rt runtime.Runtime) *VirtualMachine {
	return &VirtualMachine{
		Runtime: rt,
	}
}

// Run runs a procedure against a ledger in the given context.
func (vm *VirtualMachine) Run(ctx Context, proc Procedure, v state.View, blockProgs *programs.BlockPrograms) (err error) {
	meterParams := meter.DefaultParameters().
		WithComputationLimit(uint(proc.ComputationLimit(ctx))).
		WithMemoryLimit(proc.MemoryLimit(ctx))

	meterParams, err = getEnvironmentMeterParameters(
		vm,
		ctx,
		v,
		blockProgs,
		proc.InitialSnapshotTime(),
		proc.ExecutionTime(),
		meterParams,
	)
	if err != nil {
		return fmt.Errorf("error gettng environment meter parameters: %w", err)
	}

	interactionLimit := ctx.MaxStateInteractionSize
	if proc.ShouldDisableMemoryAndInteractionLimits(ctx) {
		meterParams = meterParams.WithMemoryLimit(math.MaxUint64)
		interactionLimit = math.MaxUint64
	}

	stTxn := state.NewStateTransaction(
		v,
		state.DefaultParameters().
			WithMeterParameters(meterParams).
			WithMaxKeySizeAllowed(ctx.MaxStateKeySize).
			WithMaxValueSizeAllowed(ctx.MaxStateValueSize).
			WithMaxInteractionSizeAllowed(interactionLimit),
	)

	newTxnProgs := blockProgs.NewTransactionPrograms
	if proc.IsSnapshotReadTransaction() {
		newTxnProgs = blockProgs.NewSnapshotReadTransactionPrograms
	}

	txnProgs, err := newTxnProgs(
		proc.InitialSnapshotTime(),
		proc.ExecutionTime())
	if err != nil {
		return err
	}

	if !proc.IsBootstrapping() {
		defer func() {
			commitErr := txnProgs.Commit()
			if commitErr != nil {
				// NOTE: This does not impact correctness.
				ctx.Logger.Err(commitErr).Msg(
					"failed to commit transaction programs")
			}
		}()
	}

	err = proc.Run(vm, ctx, stTxn, txnProgs)
	if err != nil {
		return err
	}

	return nil
}

// GetAccount returns an account by address or an error if none exists.
func (vm *VirtualMachine) GetAccount(ctx Context, address flow.Address, v state.View, blockProgs *programs.BlockPrograms) (*flow.Account, error) {
	stTxn := state.NewStateTransaction(
		v,
		state.DefaultParameters().
			WithMaxKeySizeAllowed(ctx.MaxStateKeySize).
			WithMaxValueSizeAllowed(ctx.MaxStateValueSize).
			WithMaxInteractionSizeAllowed(ctx.MaxStateInteractionSize),
	)

	txnProgs, err := blockProgs.NewSnapshotReadTransactionPrograms(
		programs.EndOfBlockExecutionTime,
		programs.EndOfBlockExecutionTime)
	if err != nil {
		return nil, err
	}

	defer func() {
		commitErr := txnProgs.Commit()
		if commitErr != nil {
			// NOTE: This does not impact correctness.
			ctx.Logger.Err(commitErr).Msg(
				"failed to commit transaction programs")
		}
	}()

	account, err := getAccount(vm, ctx, stTxn, txnProgs, address)
	if err != nil {
		if errors.IsALedgerFailure(err) {
			return nil, fmt.Errorf("cannot get account, this error usually happens if the reference block for this query is not set to a recent block: %w", err)
		}
		return nil, fmt.Errorf("cannot get account: %w", err)
	}
	return account, nil
}

// invokeMetaTransaction invokes a meta transaction inside the context of an outer transaction.
//
// Errors that occur in a meta transaction are propagated as a single error that can be
// captured by the Cadence runtime and eventually disambiguated by the parent context.
func (vm *VirtualMachine) invokeMetaTransaction(parentCtx Context, tx *TransactionProcedure, stTxn *state.StateHolder, txnProgs *programs.TransactionPrograms) (errors.Error, error) {
	invoker := NewTransactionInvoker()

	// do not deduct fees or check storage in meta transactions
	ctx := NewContextFromParent(parentCtx,
		WithAccountStorageLimit(false),
		WithTransactionFeesEnabled(false),
	)

	err := invoker.Process(vm, &ctx, tx, stTxn, txnProgs)
	txErr, fatalErr := errors.SplitErrorTypes(err)
	return txErr, fatalErr
}
