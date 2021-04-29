package procedures

import (
	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/fvm/context"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

type ScriptProcedure struct {
	ID        flow.Identifier
	Script    []byte
	Arguments [][]byte
	Value     cadence.Value
	Logs      []string
	GasUsed   uint64
	Err       errors.Error
}

func (proc *ScriptProcedure) WithArguments(args ...[]byte) *ScriptProcedure {
	return &ScriptProcedure{
		ID:        proc.ID,
		Script:    proc.Script,
		Arguments: args,
	}
}

func (proc *ScriptProcedure) Run(vm *context.VirtualMachine, ctx *context.Context, sth *state.StateHolder, programs *programs.Programs) error {
	for _, p := range ctx.ScriptProcessors {
		err := p.Process(vm, ctx, proc, sth, programs)
		txError, failure := errors.SplitErrorTypes(err)
		if failure != nil {
			return failure
		}
		if txError != nil {
			proc.Err = txError
			return nil
		}
	}

	return nil
}
