package fvm

import (
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"

	errors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter/basic"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

// An Procedure is an operation (or set of operations) that reads or writes ledger state.
type Procedure interface {
	Run(vm *VirtualMachine, ctx Context, sth *state.StateHolder, programs *programs.Programs) error
	ComputationLimit(ctx Context) uint64
	MemoryLimit(ctx Context) uint64
}

func NewInterpreterRuntime(options ...runtime.Option) runtime.Runtime {

	defaultOptions := []runtime.Option{
		runtime.WithContractUpdateValidationEnabled(true),
	}

	return runtime.NewInterpreterRuntime(
		append(defaultOptions, options...)...,
	)
}

// A VirtualMachine augments the Cadence runtime with Flow host functionality.
type VirtualMachine struct {
	Runtime runtime.Runtime
}

// NewVirtualMachine creates a new virtual machine instance with the provided runtime.
func NewVirtualMachine(rt runtime.Runtime) *VirtualMachine {
	return &VirtualMachine{
		Runtime: rt,
	}
}

// Run runs a procedure against a ledger in the given context.
func (vm *VirtualMachine) Run(ctx Context, proc Procedure, v state.View, programs *programs.Programs) (err error) {
	st := state.NewState(v,
		state.WithMeter(basic.NewMeter(
			uint(proc.ComputationLimit(ctx)),
			uint(proc.MemoryLimit(ctx)))),
		state.WithMaxKeySizeAllowed(ctx.MaxStateKeySize),
		state.WithMaxValueSizeAllowed(ctx.MaxStateValueSize),
		state.WithMaxInteractionSizeAllowed(ctx.MaxStateInteractionSize))
	sth := state.NewStateHolder(st)

	err = proc.Run(vm, ctx, sth, programs)
	if err != nil {
		return err
	}

	return nil
}

// GetAccount returns an account by address or an error if none exists.
func (vm *VirtualMachine) GetAccount(ctx Context, address flow.Address, v state.View, programs *programs.Programs) (*flow.Account, error) {
	st := state.NewState(v,
		state.WithMaxKeySizeAllowed(ctx.MaxStateKeySize),
		state.WithMaxValueSizeAllowed(ctx.MaxStateValueSize),
		state.WithMaxInteractionSizeAllowed(ctx.MaxStateInteractionSize))

	sth := state.NewStateHolder(st)
	account, err := getAccount(vm, ctx, sth, programs, address)
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
func (vm *VirtualMachine) invokeMetaTransaction(parentCtx Context, tx *TransactionProcedure, sth *state.StateHolder, programs *programs.Programs) (errors.Error, error) {
	invoker := NewTransactionInvoker(zerolog.Nop())

	// do not deduct fees or check storage in meta transactions
	ctx := NewContextFromParent(parentCtx,
		WithAccountStorageLimit(false),
		WithTransactionFeesEnabled(false),
	)

	err := invoker.Process(vm, &ctx, tx, sth, programs)
	txErr, fatalErr := errors.SplitErrorTypes(err)
	return txErr, fatalErr
}
