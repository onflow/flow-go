package processors

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/flow-go/fvm/context"
	"github.com/onflow/flow-go/fvm/env"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
)

type ScriptInvocator struct{}

func (i ScriptInvocator) Invocate(
	vm context.VirtualMachine,
	ctx context.Context,
	proc context.Runnable,
	sth *state.StateHolder,
	programs *programs.Programs,
) (err error, logs []string, value cadence.Value) {
	env := env.NewReadOnlyEnvironment(ctx, vm, sth, programs)
	procId := proc.ID()
	location := common.ScriptLocation(procId[:])
	value, err = vm.Runtime().ExecuteScript(
		runtime.Script{
			Source:    proc.Script(),
			Arguments: proc.Arguments(),
		},
		runtime.Context{
			Interface: env,
			Location:  location,
		},
	)

	if err != nil {
		return errors.HandleRuntimeError(err), nil, nil
	}

	return nil, env.Logs(), value
}
