package fvm

import (
	"context"
	"fmt"
	"math"

	"github.com/onflow/cadence/runtime"

	errors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

type ProcedureType string

const (
	BootstrapProcedureType   = ProcedureType("bootstrap")
	TransactionProcedureType = ProcedureType("transaction")
	ScriptProcedureType      = ProcedureType("script")
)

// An Procedure is an operation (or set of operations) that reads or writes ledger state.
type Procedure interface {
	Run(
		ctx Context,
		txnState *state.TransactionState,
		programs *programs.TransactionPrograms,
	) error

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
	blockPrograms := ctx.BlockPrograms
	if blockPrograms == nil {
		blockPrograms = programs.NewEmptyBlockProgramsWithTransactionOffset(
			uint32(proc.ExecutionTime()))
	}

	var txnPrograms *programs.TransactionPrograms
	var err error
	switch proc.Type() {
	case ScriptProcedureType:
		txnPrograms, err = blockPrograms.NewSnapshotReadTransactionPrograms(
			proc.InitialSnapshotTime(),
			proc.ExecutionTime())
	case TransactionProcedureType, BootstrapProcedureType:
		txnPrograms, err = blockPrograms.NewTransactionPrograms(
			proc.InitialSnapshotTime(),
			proc.ExecutionTime())
	default:
		return fmt.Errorf("invalid proc type: %v", proc.Type())
	}

	if err != nil {
		return fmt.Errorf("error creating transaction programs: %w", err)
	}

	// Note: it is safe to skip committing the parsed programs for non-normal
	// transactions (i.e., bootstrap and script) since this is only an
	// optimization.
	if proc.Type() == TransactionProcedureType {
		defer func() {
			commitErr := txnPrograms.Commit()
			if commitErr != nil {
				// NOTE: txnPrograms commit error does not impact correctness,
				// but may slow down execution since some programs may need to
				// be re-parsed.
				ctx.Logger.Warn().Err(commitErr).Msg(
					"failed to commit transaction programs")
			}
		}()
	}

	meterParams := meter.DefaultParameters().
		WithComputationLimit(uint(proc.ComputationLimit(ctx))).
		WithMemoryLimit(proc.MemoryLimit(ctx))

	meterParams, err = getEnvironmentMeterParameters(
		ctx,
		v,
		txnPrograms,
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

	eventSizeLimit := ctx.EventCollectionByteSizeLimit
	meterParams = meterParams.WithEventEmitByteLimit(eventSizeLimit)

	txnState := state.NewTransactionState(
		v,
		state.DefaultParameters().
			WithMeterParameters(meterParams).
			WithMaxKeySizeAllowed(ctx.MaxStateKeySize).
			WithMaxValueSizeAllowed(ctx.MaxStateValueSize).
			WithMaxInteractionSizeAllowed(interactionLimit),
	)

	return proc.Run(ctx, txnState, txnPrograms)
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
			WithMaxInteractionSizeAllowed(ctx.MaxStateInteractionSize),
	)

	blockPrograms := ctx.BlockPrograms
	if blockPrograms == nil {
		blockPrograms = programs.NewEmptyBlockPrograms()
	}

	txnPrograms, err := blockPrograms.NewSnapshotReadTransactionPrograms(
		programs.EndOfBlockExecutionTime,
		programs.EndOfBlockExecutionTime)
	if err != nil {
		return nil, fmt.Errorf(
			"error creating transaction programs for GetAccount: %w",
			err)
	}

	env := NewScriptEnv(context.Background(), ctx, txnState, txnPrograms)
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
