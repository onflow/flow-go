package fvm

import (
	"fmt"

	"github.com/rs/zerolog"

	errors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
)

// An Procedure is an operation (or set of operations) that reads or writes ledger state.
type Procedure interface {
	Run(vm *VirtualMachine, ctx Context, sth *state.StateHolder, programs *programs.Programs) error
}

func NewInterpreterRuntime() runtime.Runtime {
	return runtime.NewInterpreterRuntime(
		runtime.WithContractUpdateValidationEnabled(true),
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
		state.WithMaxKeySizeAllowed(ctx.MaxStateKeySize),
		state.WithMaxValueSizeAllowed(ctx.MaxStateValueSize),
		state.WithMaxInteractionSizeAllowed(ctx.MaxStateInteractionSize))
	sth := state.NewStateHolder(st)

	defer func() {
		if r := recover(); r != nil {

			// Cadence may fail to encode certain values.
			// Return an error for now, which will cause transactions to revert.
			//
			if encodingErr, ok := r.(interpreter.EncodingUnsupportedValueError); ok {
				err = errors.NewEncodingUnsupportedValueError(encodingErr.Value, encodingErr.Path)
				return
			}

			panic(r)
		}
	}()

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
		return nil, fmt.Errorf("cannot get account: %w", err)
	}
	return account, nil
}

func (vm *VirtualMachine) invokeMetaTransaction(ctx Context, tx *TransactionProcedure, sth *state.StateHolder, programs *programs.Programs) (errors.Error, error) {
	invocator := NewTransactionInvocator(zerolog.Nop())
	err := invocator.Process(vm, &ctx, tx, sth, programs)
	txErr, fatalErr := errors.SplitErrorTypes(err)
	return txErr, fatalErr
}

func (vm *VirtualMachine) invokeContractFunction(
	contractLocation common.AddressLocation,
	functionName string,
	arguments []interpreter.Value,
	argumentTypes []sema.Type,
	ctx *Context,
	proc *TransactionProcedure,
	sth *state.StateHolder,
	programs *programs.Programs,
) (fvmErr errors.Error, processErr error) {

	invocator := NewTransactionContractFunctionInvocator(
		contractLocation,
		functionName,
		arguments,
		argumentTypes,
		zerolog.Nop(),
	)
	_, err := invocator.Invoke(vm, ctx, proc, sth, programs)

	return errors.SplitErrorTypes(err)
}
