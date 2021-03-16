package fvm

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

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
	Err     Error
}

type ScriptProcessor interface {
	Process(*VirtualMachine, Context, *ScriptProcedure, *state.State, *Programs) error
}

func (proc *ScriptProcedure) WithArguments(args ...[]byte) *ScriptProcedure {
	return &ScriptProcedure{
		ID:        proc.ID,
		Script:    proc.Script,
		Arguments: args,
	}
}

func (proc *ScriptProcedure) Run(vm *VirtualMachine, ctx Context, st *state.State, programs *Programs) error {
	for _, p := range ctx.ScriptProcessors {
		err := p.Process(vm, ctx, proc, st, programs)
		vmErr, fatalErr := handleError(err)
		if fatalErr != nil {
			return fatalErr
		}

		if vmErr != nil {
			proc.Err = vmErr
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
	st *state.State,
	programs *Programs,
) error {
	env, err := newEnvironment(ctx, vm, st, programs)
	if err != nil {
		return err
	}

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
		return err
	}

	proc.Value = value
	proc.Logs = env.getLogs()
	proc.Events = env.Events()

	return nil
}
