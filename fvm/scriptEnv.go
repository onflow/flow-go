package fvm

import (
	"context"

	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
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

	// TODO(patrick): remove this hack
	env.accountKeys = handler.NewAccountKeyHandler(env.accounts)
	env.fullEnv = env

	return env
}

// Block Environment Functions

func (e *ScriptEnv) AddEncodedAccountKey(_ runtime.Address, _ []byte) error {
	return errors.NewOperationNotSupportedError("AddEncodedAccountKey")
}

func (e *ScriptEnv) RevokeEncodedAccountKey(_ runtime.Address, _ int) (publicKey []byte, err error) {
	return nil, errors.NewOperationNotSupportedError("RevokeEncodedAccountKey")
}

func (e *ScriptEnv) AddAccountKey(_ runtime.Address, _ *runtime.PublicKey, _ runtime.HashAlgorithm, _ int) (*runtime.AccountKey, error) {
	return nil, errors.NewOperationNotSupportedError("AddAccountKey")
}

func (e *ScriptEnv) RevokeAccountKey(_ runtime.Address, _ int) (*runtime.AccountKey, error) {
	return nil, errors.NewOperationNotSupportedError("RevokeAccountKey")
}
