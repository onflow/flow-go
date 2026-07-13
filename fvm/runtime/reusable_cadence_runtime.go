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
// There are four cadence runtime.Environments, one for each combination of
// {transaction, script} x {interpreter, VM}. The wrapper struct selected by the pool determines
// which environment is used (see reusableCadence{Transaction,Script}{,VM}Runtime).
type reusableCadenceRuntime struct {
	runtime.Runtime

	chain flow.Chain

	TxRuntimeEnv       runtime.Environment
	ScriptRuntimeEnv   runtime.Environment
	VMTxRuntimeEnv     runtime.Environment
	VMScriptRuntimeEnv runtime.Environment

	fvmEnv *SwappableEnvironment
}

// SwappableEnvironment is a wrapper type that extends the functionality of environment.Environment.
// It is designed to allow dynamic replacement of the underlying environment implementation.
type SwappableEnvironment struct {
	environment.Environment
	onSwap []func() // Callbacks triggered on every SetFvmEnvironment call.
}

// RegisterOnSwapCallback registers a callback that is invoked whenever
// the underlying Environment is swapped (during pool Borrow and Return).
// This enables long-lived, reusable objects to clear per-transaction
// caches at transaction boundaries.
func (se *SwappableEnvironment) RegisterOnSwapCallback(cb func()) {
	se.onSwap = append(se.onSwap, cb)
}

func newReusableCadenceRuntime(
	rt runtime.Runtime,
	chain flow.Chain,
	config runtime.Config,
) *reusableCadenceRuntime {
	reusable := &reusableCadenceRuntime{
		Runtime:            rt,
		chain:              chain,
		TxRuntimeEnv:       runtime.NewBaseInterpreterEnvironment(config),
		ScriptRuntimeEnv:   runtime.NewScriptInterpreterEnvironment(config),
		VMTxRuntimeEnv:     runtime.NewBaseVMEnvironment(config),
		VMScriptRuntimeEnv: runtime.NewScriptVMEnvironment(config),
		fvmEnv:             &SwappableEnvironment{},
	}

	reusable.declareStandardLibraryFunctions()

	return reusable
}

func (reusable *reusableCadenceRuntime) declareStandardLibraryFunctions() {
	fvmEnv := reusable.fvmEnv

	// random source for transactions (system transaction only)
	reusable.TxRuntimeEnv.DeclareValue(InterpreterBlockRandomSourceDeclaration(fvmEnv), nil)
	reusable.VMTxRuntimeEnv.DeclareValue(VMBlockRandomSourceDeclaration(fvmEnv), nil)

	// transaction index (transactions and scripts)
	reusable.TxRuntimeEnv.DeclareValue(InterpreterTransactionIndexDeclaration(fvmEnv), nil)
	reusable.ScriptRuntimeEnv.DeclareValue(InterpreterTransactionIndexDeclaration(fvmEnv), nil)
	reusable.VMTxRuntimeEnv.DeclareValue(VMTransactionIndexDeclaration(fvmEnv), nil)
	reusable.VMScriptRuntimeEnv.DeclareValue(VMTransactionIndexDeclaration(fvmEnv), nil)

	// EVM: the interpreter contract value and the VM functions share a single contract handler.
	evmInternalContractValue, evmInternalFunctions := EVMInternalEVMContract(reusable.chain.ChainID(), fvmEnv)
	evmContractAddress := evm.ContractAccountAddress(reusable.chain.ChainID())

	for _, env := range []runtime.Environment{
		reusable.TxRuntimeEnv,
		reusable.ScriptRuntimeEnv,
		reusable.VMTxRuntimeEnv,
		reusable.VMScriptRuntimeEnv,
	} {
		stdlib.SetupEnvironment(
			env,
			evmInternalContractValue,
			evmInternalFunctions,
			evmContractAddress,
		)
	}
}

func (reusable *reusableCadenceRuntime) SetFvmEnvironment(fvmEnv environment.Environment) {
	for _, fn := range reusable.fvmEnv.onSwap {
		fn()
	}
	reusable.fvmEnv.Environment = fvmEnv
}

func (reusable *reusableCadenceRuntime) CadenceTXEnv() runtime.Environment {
	return reusable.TxRuntimeEnv
}

func (reusable *reusableCadenceRuntime) CadenceScriptEnv() runtime.Environment {
	return reusable.ScriptRuntimeEnv
}

// newTransactionExecutor returns a cadence transaction executor using the given environment.
// When useVM is true, the cadence VM is used to execute the transaction.
func (reusable *reusableCadenceRuntime) newTransactionExecutor(
	script runtime.Script,
	location common.Location,
	env runtime.Environment,
	useVM bool,
) runtime.Executor {
	return reusable.Runtime.NewTransactionExecutor(
		script,
		runtime.Context{
			Interface:        reusable.fvmEnv,
			Location:         location,
			Environment:      env,
			UseVM:            useVM,
			MemoryGauge:      reusable.fvmEnv,
			ComputationGauge: reusable.fvmEnv,
		},
	)
}

// executeScript executes the given script using the given environment.
// When useVM is true, the cadence VM is used to execute the script.
func (reusable *reusableCadenceRuntime) executeScript(
	script runtime.Script,
	location common.Location,
	env runtime.Environment,
	useVM bool,
) (
	cadence.Value,
	error,
) {
	return reusable.Runtime.ExecuteScript(
		script,
		runtime.Context{
			Interface:        reusable.fvmEnv,
			Location:         location,
			Environment:      env,
			UseVM:            useVM,
			MemoryGauge:      reusable.fvmEnv,
			ComputationGauge: reusable.fvmEnv,
		},
	)
}

// readStored reads the stored value at the given path using the given environment.
// Reading stored values does not differ between the interpreter and the VM, so UseVM is not set.
func (reusable *reusableCadenceRuntime) readStored(
	address common.Address,
	path cadence.Path,
	env runtime.Environment,
) (
	cadence.Value,
	error,
) {
	return reusable.Runtime.ReadStored(
		address,
		path,
		runtime.Context{
			Interface:        reusable.fvmEnv,
			Environment:      env,
			MemoryGauge:      reusable.fvmEnv,
			ComputationGauge: reusable.fvmEnv,
		},
	)
}

// invokeContractFunction invokes the given contract function using the given environment.
// When useVM is true, the cadence VM is used to invoke the function.
func (reusable *reusableCadenceRuntime) invokeContractFunction(
	contractLocation common.AddressLocation,
	functionName string,
	arguments []cadence.Value,
	argumentTypes []sema.Type,
	env runtime.Environment,
	useVM bool,
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
			Environment:      env,
			UseVM:            useVM,
			MemoryGauge:      reusable.fvmEnv,
			ComputationGauge: reusable.fvmEnv,
		},
	)
}

// reusableCadenceTransactionRuntime is a wrapper around reusableCadenceRuntime
// that is meant to be used in transactions executed by the cadence interpreter.
// see: reusableCadenceRuntime
type reusableCadenceTransactionRuntime struct {
	*reusableCadenceRuntime
}

var _ environment.ReusableCadenceRuntime = reusableCadenceTransactionRuntime{}

func (reusable reusableCadenceTransactionRuntime) NewTransactionExecutor(
	script runtime.Script,
	location common.Location,
) runtime.Executor {
	return reusable.newTransactionExecutor(script, location, reusable.TxRuntimeEnv, false)
}

func (reusable reusableCadenceTransactionRuntime) ExecuteScript(
	script runtime.Script,
	location common.Location,
) (
	cadence.Value,
	error,
) {
	return reusable.executeScript(script, location, reusable.ScriptRuntimeEnv, false)
}

func (reusable reusableCadenceTransactionRuntime) ReadStored(
	address common.Address,
	path cadence.Path,
) (
	cadence.Value,
	error,
) {
	return reusable.readStored(address, path, reusable.TxRuntimeEnv)
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
	return reusable.invokeContractFunction(
		contractLocation,
		functionName,
		arguments,
		argumentTypes,
		reusable.TxRuntimeEnv,
		false,
	)
}

// reusableCadenceScriptRuntime is a wrapper around reusableCadenceRuntime
// that is meant to be used in scripts executed by the cadence interpreter.
// see: reusableCadenceRuntime
type reusableCadenceScriptRuntime struct {
	*reusableCadenceRuntime
}

var _ environment.ReusableCadenceRuntime = reusableCadenceScriptRuntime{}

func (reusable reusableCadenceScriptRuntime) NewTransactionExecutor(
	script runtime.Script,
	location common.Location,
) runtime.Executor {
	return reusable.newTransactionExecutor(script, location, reusable.TxRuntimeEnv, false)
}

func (reusable reusableCadenceScriptRuntime) ExecuteScript(
	script runtime.Script,
	location common.Location,
) (
	cadence.Value,
	error,
) {
	return reusable.executeScript(script, location, reusable.ScriptRuntimeEnv, false)
}

func (reusable reusableCadenceScriptRuntime) ReadStored(
	address common.Address,
	path cadence.Path,
) (
	cadence.Value,
	error,
) {
	return reusable.readStored(address, path, reusable.ScriptRuntimeEnv)
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
	return reusable.invokeContractFunction(
		contractLocation,
		functionName,
		arguments,
		argumentTypes,
		reusable.ScriptRuntimeEnv,
		false,
	)
}

// reusableCadenceTransactionVMRuntime is a wrapper around reusableCadenceRuntime
// that is meant to be used in transactions executed by the cadence VM.
// see: reusableCadenceRuntime
type reusableCadenceTransactionVMRuntime struct {
	*reusableCadenceRuntime
}

var _ environment.ReusableCadenceRuntime = reusableCadenceTransactionVMRuntime{}

func (reusable reusableCadenceTransactionVMRuntime) NewTransactionExecutor(
	script runtime.Script,
	location common.Location,
) runtime.Executor {
	return reusable.newTransactionExecutor(script, location, reusable.VMTxRuntimeEnv, true)
}

func (reusable reusableCadenceTransactionVMRuntime) ExecuteScript(
	script runtime.Script,
	location common.Location,
) (
	cadence.Value,
	error,
) {
	return reusable.executeScript(script, location, reusable.VMScriptRuntimeEnv, true)
}

func (reusable reusableCadenceTransactionVMRuntime) ReadStored(
	address common.Address,
	path cadence.Path,
) (
	cadence.Value,
	error,
) {
	// Reading stored values does not differ between the interpreter and the VM.
	return reusable.readStored(address, path, reusable.TxRuntimeEnv)
}

func (reusable reusableCadenceTransactionVMRuntime) InvokeContractFunction(
	contractLocation common.AddressLocation,
	functionName string,
	arguments []cadence.Value,
	argumentTypes []sema.Type,
) (
	cadence.Value,
	error,
) {
	return reusable.invokeContractFunction(
		contractLocation,
		functionName,
		arguments,
		argumentTypes,
		reusable.VMTxRuntimeEnv,
		true,
	)
}

// reusableCadenceScriptVMRuntime is a wrapper around reusableCadenceRuntime
// that is meant to be used in scripts executed by the cadence VM.
// see: reusableCadenceRuntime
type reusableCadenceScriptVMRuntime struct {
	*reusableCadenceRuntime
}

var _ environment.ReusableCadenceRuntime = reusableCadenceScriptVMRuntime{}

func (reusable reusableCadenceScriptVMRuntime) NewTransactionExecutor(
	script runtime.Script,
	location common.Location,
) runtime.Executor {
	return reusable.newTransactionExecutor(script, location, reusable.VMTxRuntimeEnv, true)
}

func (reusable reusableCadenceScriptVMRuntime) ExecuteScript(
	script runtime.Script,
	location common.Location,
) (
	cadence.Value,
	error,
) {
	return reusable.executeScript(script, location, reusable.VMScriptRuntimeEnv, true)
}

func (reusable reusableCadenceScriptVMRuntime) ReadStored(
	address common.Address,
	path cadence.Path,
) (
	cadence.Value,
	error,
) {
	// Reading stored values does not differ between the interpreter and the VM.
	return reusable.readStored(address, path, reusable.ScriptRuntimeEnv)
}

func (reusable reusableCadenceScriptVMRuntime) InvokeContractFunction(
	contractLocation common.AddressLocation,
	functionName string,
	arguments []cadence.Value,
	argumentTypes []sema.Type,
) (
	cadence.Value,
	error,
) {
	return reusable.invokeContractFunction(
		contractLocation,
		functionName,
		arguments,
		argumentTypes,
		reusable.VMScriptRuntimeEnv,
		true,
	)
}
