package fvm

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/fvm/state"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/hash"
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
	Events    []cadence.Event
	// TODO: report gas consumption: https://github.com/dapperlabs/flow-go/issues/4139
	GasUsed uint64
	Err     Error
}

type ScriptProcessor interface {
	Process(*VirtualMachine, Context, *ScriptProcedure, state.Ledger) error
}

func (proc *ScriptProcedure) WithArguments(args [][]byte) *ScriptProcedure {
	return &ScriptProcedure{
		ID:        proc.ID,
		Script:    proc.Script,
		Arguments: args,
	}
}

func (proc *ScriptProcedure) Run(vm *VirtualMachine, ctx Context, ledger state.Ledger) error {
	for _, p := range ctx.ScriptProcessors {
		err := p.Process(vm, ctx, proc, ledger)
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
	ledger state.Ledger,
) error {
	env := newEnvironment(ctx, ledger)

	location := runtime.ScriptLocation(proc.ID[:])

	value, err := vm.Runtime.ExecuteScript(proc.Script, proc.Arguments, env, location)
	if err != nil {
		return err
	}

	proc.Value = value
	proc.Logs = env.getLogs()
	proc.Events = env.getEvents()

	return nil
}
