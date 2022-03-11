package fvm

import (
	"fmt"

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
	Err       errors.Error
}

type ScriptProcessor interface {
	Process(*VirtualMachine, Context, *ScriptProcedure, *state.StateHolder, *programs.Programs) error
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
		err := p.Process(vm, ctx, proc, sth, programs)
		txError, failure := errors.SplitErrorTypes(err)
		if failure != nil {
			if errors.IsALedgerFailure(failure) {
				return fmt.Errorf("cannot execute the script, this error usually happens if the reference block for this script is not set to a recent block: %w", failure)
			}
			return failure
		}
		if txError != nil {
			proc.Err = txError
			return nil
		}
	}

	return nil
}

func (proc *ScriptProcedure) ComputationLimit(ctx Context) uint64 {
	computationLimit := ctx.ComputationLimit
	// if ctx.ComputationLimit is also zero, fallback to the default computation limit
	if computationLimit == 0 {
		computationLimit = DefaultComputationLimit
	}
	return computationLimit
}

func (proc *ScriptProcedure) MemoryLimit(ctx Context) uint64 {
	memoryLimit := ctx.MemoryLimit
	// if ctx.MemoryLimit is also zero, fallback to the default memory limit
	if memoryLimit == 0 {
		memoryLimit = DefaultMemoryLimit
	}
	return memoryLimit
}

type ScriptInvoker struct{}

func NewScriptInvoker() ScriptInvoker {
	return ScriptInvoker{}
}

func (i ScriptInvoker) Process(
	vm *VirtualMachine,
	ctx Context,
	proc *ScriptProcedure,
	sth *state.StateHolder,
	programs *programs.Programs,
) error {
	env := NewScriptEnvironment(ctx, vm, sth, programs)
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
		return errors.HandleRuntimeError(err)
	}

	proc.Value = value
	proc.Logs = env.Logs()
	proc.Events = env.Events()
	proc.GasUsed = env.ComputationUsed()
	return nil
}
