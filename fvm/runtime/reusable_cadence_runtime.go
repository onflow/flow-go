package runtime

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/sema"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/model/flow"
)

// TODO(JanezP): unexport all types in this file
// They are just used by some test

// reusableCadenceRuntime is a wrapper around cadence Runtime and cadence Environment
// with pre-injected cadence context for: EVM, getTransactionIndex, ...
// it can be reused by changing the fvmEnv. The reuse happens accross blocks and between scripts and transactions
//
// This exists because creating and setting up a cadence runtime and environment is a costly operation.
// This assumes that the following objects are safe to reuse across blocks:
// - cadence runtime
// - cadence environment
// - evm emulator (and related objects)
//
// because the cadence environment differs between scripts and transactions there are 2
// wrapper structs for reusableCadenceRuntime that change what env ReadStored
// and InvokeContractFunction use.
type reusableCadenceRuntime struct {
	runtime.Runtime

	chain flow.Chain

	TxRuntimeEnv     runtime.Environment
	ScriptRuntimeEnv runtime.Environment

	fvmEnv *SwappableEnvironment
}

// SwappableEnvironment is a wrapper type that extends the functionality of environment.Environment.
// It is designed to allow dynamic replacement of the underlying environment implementation.
type SwappableEnvironment struct {
	environment.Environment
}

func newReusableCadenceRuntime(
	rt runtime.Runtime,
	chain flow.Chain,
	config runtime.Config,
) *reusableCadenceRuntime {
	reusable := &reusableCadenceRuntime{
		Runtime:          rt,
		chain:            chain,
		TxRuntimeEnv:     runtime.NewBaseInterpreterEnvironment(config),
		ScriptRuntimeEnv: runtime.NewScriptInterpreterEnvironment(config),
		fvmEnv:           &SwappableEnvironment{},
	}

	reusable.declareStandardLibraryFunctions()

	return reusable
}

func (reusable *reusableCadenceRuntime) declareStandardLibraryFunctions() {
	// random source for transactions
	declaration := BlockRandomSourceDeclaration(reusable.fvmEnv)
	reusable.TxRuntimeEnv.DeclareValue(declaration, nil)

	// transaction index
	declaration = TransactionIndexDeclaration(reusable.fvmEnv)
	reusable.TxRuntimeEnv.DeclareValue(declaration, nil)
	reusable.ScriptRuntimeEnv.DeclareValue(declaration, nil)

	evmInternalContractValue := EVMInternalEVMContractValue(reusable.chain.ChainID(), reusable.fvmEnv)
	evmContractAddress := evm.ContractAccountAddress(reusable.chain.ChainID())

	stdlib.SetupEnvironment(
		reusable.TxRuntimeEnv,
		evmInternalContractValue,
		evmContractAddress,
	)

	stdlib.SetupEnvironment(
		reusable.ScriptRuntimeEnv,
		evmInternalContractValue,
		evmContractAddress,
	)
}

func (reusable *reusableCadenceRuntime) SetFvmEnvironment(fvmEnv environment.Environment) {
	reusable.fvmEnv.Environment = fvmEnv
}

func (reusable *reusableCadenceRuntime) CadenceTXEnv() runtime.Environment {
	return reusable.TxRuntimeEnv
}

func (reusable *reusableCadenceRuntime) CadenceScriptEnv() runtime.Environment {
	return reusable.ScriptRuntimeEnv
}

func (reusable *reusableCadenceRuntime) NewTransactionExecutor(
	script runtime.Script,
	location common.Location,
) runtime.Executor {
	return reusable.Runtime.NewTransactionExecutor(
		script,
		runtime.Context{
			Interface:        reusable.fvmEnv,
			Location:         location,
			Environment:      reusable.TxRuntimeEnv,
			MemoryGauge:      reusable.fvmEnv,
			ComputationGauge: reusable.fvmEnv,
		},
	)
}

func (reusable *reusableCadenceRuntime) ExecuteScript(
	script runtime.Script,
	location common.Location,
) (
	cadence.Value,
	error,
) {
	return reusable.Runtime.ExecuteScript(
		script,
		runtime.Context{
			Interface:        reusable.fvmEnv,
			Location:         location,
			Environment:      reusable.ScriptRuntimeEnv,
			MemoryGauge:      reusable.fvmEnv,
			ComputationGauge: reusable.fvmEnv,
		},
	)
}

// reusableCadenceTransactionRuntime is a wrapper around reusableCadenceRuntime
// that is meant to be used in transactions.
// see: reusableCadenceRuntime
type reusableCadenceTransactionRuntime struct {
	*reusableCadenceRuntime
}

var _ environment.ReusableCadenceRuntime = reusableCadenceTransactionRuntime{}

func (reusable reusableCadenceTransactionRuntime) ReadStored(
	address common.Address,
	path cadence.Path,
) (
	cadence.Value,
	error,
) {
	return reusable.Runtime.ReadStored(
		address,
		path,
		runtime.Context{
			Interface:        reusable.fvmEnv,
			Environment:      reusable.TxRuntimeEnv,
			MemoryGauge:      reusable.fvmEnv,
			ComputationGauge: reusable.fvmEnv,
		},
	)
}

func (reusable reusableCadenceTransactionRuntime) InvokeContractFunction(
	contractLocation common.AddressLocation,
	functionName string,
	arguments []cadence.Value,
	argumentTypes []sema.Type,
) (
	cadence.Value,
	error,
) {
	return reusable.Runtime.InvokeContractFunction(
		contractLocation,
		functionName,
		arguments,
		argumentTypes,
		runtime.Context{
			Interface:        reusable.fvmEnv,
			Environment:      reusable.TxRuntimeEnv,
			MemoryGauge:      reusable.fvmEnv,
			ComputationGauge: reusable.fvmEnv,
		},
	)
}

// reusableCadenceScriptRuntime is a wrapper around reusableCadenceRuntime
// that is meant to be used in scripts.
// see: reusableCadenceRuntime
type reusableCadenceScriptRuntime struct {
	*reusableCadenceRuntime
}

var _ environment.ReusableCadenceRuntime = reusableCadenceScriptRuntime{}

func (reusable reusableCadenceScriptRuntime) ReadStored(
	address common.Address,
	path cadence.Path,
) (
	cadence.Value,
	error,
) {
	return reusable.Runtime.ReadStored(
		address,
		path,
		runtime.Context{
			Interface:        reusable.fvmEnv,
			Environment:      reusable.ScriptRuntimeEnv,
			MemoryGauge:      reusable.fvmEnv,
			ComputationGauge: reusable.fvmEnv,
		},
	)
}

func (reusable reusableCadenceScriptRuntime) InvokeContractFunction(
	contractLocation common.AddressLocation,
	functionName string,
	arguments []cadence.Value,
	argumentTypes []sema.Type,
) (
	cadence.Value,
	error,
) {
	return reusable.Runtime.InvokeContractFunction(
		contractLocation,
		functionName,
		arguments,
		argumentTypes,
		runtime.Context{
			Interface:        reusable.fvmEnv,
			Environment:      reusable.ScriptRuntimeEnv,
			MemoryGauge:      reusable.fvmEnv,
			ComputationGauge: reusable.fvmEnv,
		},
	)
}
