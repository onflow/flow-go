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
	txnState *state.TransactionState,
	programs environment.TransactionPrograms,
) environment.Environment {
	return NewScriptEnv(
		reqContext,
		fvmContext,
		txnState,
		programs)
}

func NewScriptEnv(
	reqContext context.Context,
	fvmContext Context,
	txnState *state.TransactionState,
	programs environment.TransactionPrograms,
) environment.Environment {
	return environment.NewScriptEnvironment(
		reqContext,
		fvmContext.EnvironmentParams,
		txnState,
		programs)
}
