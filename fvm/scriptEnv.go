package fvm

import (
	"context"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/state"
)

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
