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
	Run(vm *VirtualMachine, ctx Context, sth *state.StateHolder, programs *programs.Programs) error
	ComputationLimit(ctx Context) uint64
	MemoryLimit(ctx Context) uint64
	ShouldDisableMemoryAndInteractionLimits(ctx Context) bool
}

func NewInterpreterRuntime(config runtime.Config) runtime.Runtime {
	return runtime.NewInterpreterRuntime(config)
}

// A VirtualMachine augments the Cadence runtime with Flow host functionality.
type VirtualMachine struct {
	Runtime runtime.Runtime // DEPRECATED.  DO NOT USE.
}

func NewVM() *VirtualMachine {
	return &VirtualMachine{}
}

// DEPRECATED.  DO NOT USE.
//
// TODO(patrick): remove after emulator is updated.
//
// Emulator is a special snowflake which prevents fvm from every changing its
// APIs (integration test uses a pinned version of the emulator, which in turn
// uses a pinned non-master version of flow-go).  This method is expose to break
// the ridiculous circular dependency between the two builds.
func NewVirtualMachine(rt runtime.Runtime) *VirtualMachine {
	return &VirtualMachine{
		Runtime: rt,
	}
}

// DEPRECATED.  DO NOT USE.
//
// TODO(patrick): remove after emulator is updated
//
// Run runs a procedure against a ledger in the given context.
func (vm *VirtualMachine) Run(ctx Context, proc Procedure, v state.View, _ *programs.Programs) (err error) {
	return vm.RunV2(ctx, proc, v)
}

// TODO(patrick): rename back to Run after emulator is fully updated (this
// takes at least 3 sporks ...).
//
// Run runs a procedure against a ledger in the given context.
func (vm *VirtualMachine) RunV2(
	ctx Context,
	proc Procedure,
	v state.View,
) error {
	blockPrograms := ctx.BlockPrograms
	if blockPrograms == nil {
		blockPrograms = programs.NewEmptyPrograms()
	}

	meterParams := meter.DefaultParameters().
		WithComputationLimit(uint(proc.ComputationLimit(ctx))).
		WithMemoryLimit(proc.MemoryLimit(ctx))

	meterParams, err := getEnvironmentMeterParameters(
		vm,
		ctx,
		v,
		blockPrograms,
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

	err = proc.Run(vm, ctx, stTxn, blockPrograms)
	if err != nil {
		return err
	}

	return nil
}

// GetAccount returns an account by address or an error if none exists.
func (vm *VirtualMachine) GetAccount(ctx Context, address flow.Address, v state.View, programs *programs.Programs) (*flow.Account, error) {
	stTxn := state.NewStateTransaction(
		v,
		state.DefaultParameters().
			WithMaxKeySizeAllowed(ctx.MaxStateKeySize).
			WithMaxValueSizeAllowed(ctx.MaxStateValueSize).
			WithMaxInteractionSizeAllowed(ctx.MaxStateInteractionSize),
	)

	account, err := getAccount(vm, ctx, stTxn, programs, address)
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
func (vm *VirtualMachine) invokeMetaTransaction(parentCtx Context, tx *TransactionProcedure, stTxn *state.StateHolder, programs *programs.Programs) (errors.Error, error) {
	invoker := NewTransactionInvoker()

	// do not deduct fees or check storage in meta transactions
	ctx := NewContextFromParent(parentCtx,
		WithAccountStorageLimit(false),
		WithTransactionFeesEnabled(false),
	)

	err := invoker.Process(vm, &ctx, tx, stTxn, programs)
	txErr, fatalErr := errors.SplitErrorTypes(err)
	return txErr, fatalErr
}
