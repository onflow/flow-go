package fvm

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

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
	GasUsed   uint64
	Err       errors.TransactionError
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
		txError, vmError := errors.HandleRuntimeError(err)

		if txError != nil || vmError != nil {
			return txError, vmError
		}
	}

	proc.Value = value
	proc.Logs = env.getLogs()
	proc.Events = env.Events()
	proc.GasUsed = env.GetComputationUsed()
	return nil, nil
}
