package fvm

import (
	"context"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/handler"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

var _ runtime.Interface = &ScriptEnv{}
var _ Environment = &ScriptEnv{}

// ScriptEnv is a read-only mostly used for executing scripts.
type ScriptEnv struct {
	commonEnv
}

func NewScriptEnvironment(
	reqContext context.Context,
	fvmContext Context,
	vm *VirtualMachine,
	sth *state.StateHolder,
	programs *programs.Programs,
) *ScriptEnv {

	accounts := state.NewAccounts(sth)
	programsHandler := handler.NewProgramsHandler(programs, sth)
	accountKeys := handler.NewAccountKeyHandler(accounts)
	tracer := environment.NewTracer(fvmContext.Tracer, nil, fvmContext.ExtensiveTracing)
	meter := environment.NewCancellableMeter(reqContext, sth)

	env := &ScriptEnv{
		commonEnv: commonEnv{
			Tracer: tracer,
			Meter:  meter,
			ProgramLogger: environment.NewProgramLogger(
				tracer,
				fvmContext.Logger,
				fvmContext.Metrics,
				fvmContext.CadenceLoggingEnabled,
			),
			UUIDGenerator: environment.NewUUIDGenerator(tracer, meter, sth),
			UnsafeRandomGenerator: environment.NewUnsafeRandomGenerator(
				tracer,
				fvmContext.BlockHeader,
			),
			CryptoLibrary: environment.NewCryptoLibrary(tracer, meter),
			BlockInfo: environment.NewBlockInfo(
				tracer,
				meter,
				fvmContext.BlockHeader,
				fvmContext.Blocks,
			),
			TransactionInfo: environment.NoTransactionInfo{},
			ctx:             fvmContext,
			sth:             sth,
			vm:              vm,
			programs:        programsHandler,
			accounts:        accounts,
			accountKeys:     accountKeys,
			frozenAccounts:  nil,
		},
	}

	// TODO(patrick): remove this hack
	env.AccountInterface = env
	env.fullEnv = env

	env.contracts = handler.NewContractHandler(
		accounts,
		func() bool { return true },
		func() bool { return true },
		func() []common.Address { return []common.Address{} },
		func() []common.Address { return []common.Address{} },
		func(address runtime.Address, code []byte) (bool, error) { return false, nil })

	return env
}

func (e *ScriptEnv) EmitEvent(_ cadence.Event) error {
	return errors.NewOperationNotSupportedError("EmitEvent")
}

func (e *ScriptEnv) Events() []flow.Event {
	return []flow.Event{}
}

// Block Environment Functions

func (e *ScriptEnv) CreateAccount(_ runtime.Address) (address runtime.Address, err error) {
	return runtime.Address{}, errors.NewOperationNotSupportedError("CreateAccount")
}

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

func (e *ScriptEnv) UpdateAccountContractCode(_ runtime.Address, _ string, _ []byte) (err error) {
	return errors.NewOperationNotSupportedError("UpdateAccountContractCode")
}

func (e *ScriptEnv) RemoveAccountContractCode(_ runtime.Address, _ string) (err error) {
	return errors.NewOperationNotSupportedError("RemoveAccountContractCode")
}
