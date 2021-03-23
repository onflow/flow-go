package fvm

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/hash"
)

func Script(code []byte) *ScriptProcedure {
	scriptHash := hash.DefaultHasher.ComputeHash(code)

	return &ScriptProcedure{
		Script: code,
		ID:     flow.HashToID(scriptHash),
	}
}

type ScriptProcedure struct {
	ID        flow.Identifier
	Script    []byte
	Arguments [][]byte
	Value     cadence.Value
	Logs      []string
	Events    []flow.Event
	// TODO: report gas consumption: https://github.com/dapperlabs/flow-go/issues/4139
	GasUsed uint64
	Err     errors.TransactionError
}

type ScriptProcessor interface {
	Process(*VirtualMachine, Context, *ScriptProcedure, *state.StateHolder, *programs.Programs) (txError errors.TransactionError, vmError errors.VMError)
}

func (proc *ScriptProcedure) WithArguments(args ...[]byte) *ScriptProcedure {
	return &ScriptProcedure{
		ID:        proc.ID,
		Script:    proc.Script,
		Arguments: args,
	}
}

func (proc *ScriptProcedure) Run(vm *VirtualMachine, ctx Context, sth *state.StateHolder, programs *programs.Programs) error {
	for _, p := range ctx.ScriptProcessors {
		txError, vmError := p.Process(vm, ctx, proc, sth, programs)
		if vmError != nil {
			return vmError
		}
		if txError != nil {
			proc.Err = txError
			return nil
		}

	}

	return nil
}

type ScriptInvocator struct{}

func NewScriptInvocator() ScriptInvocator {
	return ScriptInvocator{}
}

func (i ScriptInvocator) Process(
	vm *VirtualMachine,
	ctx Context,
	proc *ScriptProcedure,
	sth *state.StateHolder,
	programs *programs.Programs,
) (txError errors.TransactionError, vmError errors.VMError) {
	env := newEnvironment(ctx, vm, sth, programs)
	location := common.ScriptLocation(proc.ID[:])
	value, err := vm.Runtime.ExecuteScript(
		runtime.Script{
			Source:    proc.Script,
			Arguments: proc.Arguments,
		},
		runtime.Context{
			Interface: env,
			Location:  location,
		},
	)

	if err != nil {
		txError, vmError := i.handleRuntimeError(err)

		if txError != nil || vmError != nil {
			return txError, vmError
		}
	}

	proc.Value = value
	proc.Logs = env.getLogs()
	proc.Events = env.Events()
	return nil, nil
}

func (i *ScriptInvocator) handleRuntimeError(err error) (txError errors.TransactionError, vmErr errors.VMError) {
	var runErr runtime.Error
	var ok bool
	// if not a runtime error return as vm error
	if runErr, ok = err.(runtime.Error); !ok {
		return nil, &errors.UnknownFailure{Err: runErr}
	}
	innerErr := runErr.Err

	// External errors are reported by the runtime but originate from the VM.
	//
	// External errors may be fatal or non-fatal, so additional handling
	// is required.
	if externalErr, ok := innerErr.(interpreter.ExternalError); ok {
		if recoveredErr, ok := externalErr.Recovered.(error); ok {
			// If the recovered value is an error, pass it to the original
			// error handler to distinguish between fatal and non-fatal errors.
			switch typedErr := recoveredErr.(type) {
			// TODO change this type to Env Error types
			case errors.TransactionError:
				// If the error is an fvm.Error, return as is
				return typedErr, nil
			default:
				// All other errors are considered fatal
				return nil, &errors.UnknownFailure{Err: runErr}
			}
		}
		// TODO revisit this
		// if not recovered return
		return nil, &errors.UnknownFailure{Err: externalErr}
	}

	// All other errors are non-fatal Cadence errors.
	return &errors.CadenceRuntimeError{Err: runErr}, nil
}
