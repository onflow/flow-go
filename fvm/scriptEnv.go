package fvm

import (
	"context"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/state"
)

// DEPRECATED.  DO NOT USE
//
// TODO(patrick): rm after emulator is updated.
func NewScriptEnvironment(
	reqContext context.Context,
	fvmContext Context,
	vm *VirtualMachine,
	sth *state.StateHolder,
	programs environment.TransactionPrograms,
) environment.Environment {
	return NewScriptEnv(
		reqContext,
		fvmContext,
		sth,
		programs)
}

func NewScriptEnv(
	reqContext context.Context,
	fvmContext Context,
	sth *state.StateHolder,
	programs environment.TransactionPrograms,
) environment.Environment {
	return newFacadeEnvironment(
		fvmContext,
		sth,
		programs,
		environment.NewTracer(fvmContext.TracerParams),
		environment.NewCancellableMeter(reqContext, sth),
	)
}
