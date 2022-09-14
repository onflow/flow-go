package fvm

import (
	"context"

	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/handler"
	"github.com/onflow/flow-go/fvm/state"
)

var _ runtime.Interface = &ScriptEnv{}
var _ environment.Environment = &ScriptEnv{}

// ScriptEnv is a read-only mostly used for executing scripts.
type ScriptEnv struct {
	commonEnv
}

func NewScriptEnvironment(
	reqContext context.Context,
	fvmContext Context,
	vm *VirtualMachine,
	sth *state.StateHolder,
	programs handler.TransactionPrograms,
) *ScriptEnv {

	tracer := environment.NewTracer(fvmContext.Tracer, nil, fvmContext.ExtensiveTracing)
	meter := environment.NewCancellableMeter(reqContext, sth)

	env := &ScriptEnv{
		commonEnv: newCommonEnv(
			fvmContext,
			sth,
			programs,
			tracer,
			meter,
		),
	}

	env.TransactionInfo = environment.NoTransactionInfo{}
	env.EventEmitter = environment.NoEventEmitter{}
	env.AccountCreator = environment.NoAccountCreator{}
	env.AccountFreezer = environment.NoAccountFreezer{}
	env.SystemContracts.SetEnvironment(env)

	env.ContractUpdater = handler.NoContractUpdater{}
	env.AccountKeyUpdater = handler.NoAccountKeyUpdater{}

	// TODO(patrick): remove this hack
	env.fullEnv = env

	return env
}
