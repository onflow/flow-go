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
	derivedTxnData environment.DerivedTransactionData,
) environment.Environment {
	return environment.NewScriptEnvironment(
		reqContext,
		fvmContext.EnvironmentParams,
		txnState,
		derivedTxnData)
}
