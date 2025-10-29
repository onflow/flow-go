package runtime

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/bbq/vm"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/sema"
	"github.com/onflow/cadence/stdlib"

	"github.com/onflow/flow-go/fvm/errors"
)

// Note: this is a subset of environment.Environment, redeclared to handle
// circular dependency.
type Environment interface {
	runtime.Interface
	common.Gauge

	RandomSourceHistory() ([]byte, error)
}

const randomSourceHistoryFunctionName = "randomSourceHistory"

// randomSourceHistoryFunctionType is the type of the `randomSourceHistory` function.
// This defines the signature as `func(): [UInt8]`
var randomSourceHistoryFunctionType = &sema.FunctionType{
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.ByteArrayType),
}

type ReusableCadenceRuntime struct {
	runtime.Runtime

	TxRuntimeEnv       runtime.Environment
	ScriptRuntimeEnv   runtime.Environment
	VMTxRuntimeEnv     runtime.Environment
	VMScriptRuntimeEnv runtime.Environment

	fvmEnv Environment
}

func NewReusableCadenceRuntime(
	rt runtime.Runtime,
	config runtime.Config,
) *ReusableCadenceRuntime {
	reusable := &ReusableCadenceRuntime{
		Runtime:            rt,
		TxRuntimeEnv:       runtime.NewBaseInterpreterEnvironment(config),
		ScriptRuntimeEnv:   runtime.NewScriptInterpreterEnvironment(config),
		VMTxRuntimeEnv:     runtime.NewBaseVMEnvironment(config),
		VMScriptRuntimeEnv: runtime.NewScriptVMEnvironment(config),
	}

	reusable.declareRandomSourceHistory()

	return reusable
}

func (reusable *ReusableCadenceRuntime) declareRandomSourceHistory() {

	// Declare the `randomSourceHistory` function.
	// This function is **only** used by the System transaction,
	// to fill the `RandomBeaconHistory` contract via the heartbeat resource.
	// This allows the `RandomBeaconHistory` contract to be a standard contract,
	// without any special parts.
	//
	// Since the `randomSourceHistory` function is only used by the System transaction,
	// it is not part of the cadence standard library, and can just be injected from here.
	// It also doesn't need user documentation, since it is not (and should not)
	// be called by the user. If it is called by the user it will panic.

	randomSourceHistoryFunction := interpreter.NativeFunction(
		func(
			context interpreter.NativeFunctionContext,
			_ interpreter.TypeArgumentsIterator,
			_ interpreter.Value,
			_ []interpreter.Value,
		) interpreter.Value {
			return reusable.randomSourceHistory(context)
		},
	)

	reusable.TxRuntimeEnv.DeclareValue(
		newRandomSourceHistoryFunctionValue(
			interpreter.NewUnmeteredStaticHostFunctionValue(
				randomSourceHistoryFunctionType,
				interpreter.AdaptNativeFunctionForInterpreter(
					randomSourceHistoryFunction,
				),
			),
		),
		nil,
	)

	reusable.VMTxRuntimeEnv.DeclareValue(
		newRandomSourceHistoryFunctionValue(
			vm.NewNativeFunctionValue(
				randomSourceHistoryFunctionName,
				randomSourceHistoryFunctionType,
				randomSourceHistoryFunction,
			),
		),
		nil,
	)
}

func newRandomSourceHistoryFunctionValue(functionValue interpreter.FunctionValue) stdlib.StandardLibraryValue {
	return stdlib.StandardLibraryValue{
		Name:  randomSourceHistoryFunctionName,
		Type:  randomSourceHistoryFunctionType,
		Kind:  common.DeclarationKindFunction,
		Value: functionValue,
	}
}

func (reusable *ReusableCadenceRuntime) randomSourceHistory(context interpreter.InvocationContext) interpreter.Value {
	fvmEnv := reusable.fvmEnv
	if fvmEnv == nil {
		panic(errors.NewOperationNotSupportedError(randomSourceHistoryFunctionName))
	}

	source, err := fvmEnv.RandomSourceHistory()
	if err != nil {
		panic(err)
	}

	return interpreter.ByteSliceToByteArrayValue(
		context,
		source,
	)
}

func (reusable *ReusableCadenceRuntime) SetFvmEnvironment(fvmEnv Environment) {
	reusable.fvmEnv = fvmEnv
}

func (reusable *ReusableCadenceRuntime) ReadStored(
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
			Interface: reusable.fvmEnv,
			// No difference between VM and interpreter environments here,
			// and UseVM is ignored.
			Environment:      reusable.TxRuntimeEnv,
			MemoryGauge:      reusable.fvmEnv,
			ComputationGauge: reusable.fvmEnv,
		},
	)
}

func (reusable *ReusableCadenceRuntime) InvokeContractFunction(
	contractLocation common.AddressLocation,
	functionName string,
	arguments []cadence.Value,
	argumentTypes []sema.Type,
	useVM bool,
) (
	cadence.Value,
	error,
) {
	var environment runtime.Environment
	if useVM {
		environment = reusable.VMTxRuntimeEnv
	} else {
		environment = reusable.TxRuntimeEnv
	}

	return reusable.Runtime.InvokeContractFunction(
		contractLocation,
		functionName,
		arguments,
		argumentTypes,
		runtime.Context{
			Interface:        reusable.fvmEnv,
			Environment:      environment,
			UseVM:            useVM,
			MemoryGauge:      reusable.fvmEnv,
			ComputationGauge: reusable.fvmEnv,
		},
	)
}

func (reusable *ReusableCadenceRuntime) NewTransactionExecutor(
	script runtime.Script,
	location common.Location,
	useVM bool,
) runtime.Executor {
	var environment runtime.Environment
	if useVM {
		environment = reusable.VMTxRuntimeEnv
	} else {
		environment = reusable.TxRuntimeEnv
	}

	return reusable.Runtime.NewTransactionExecutor(
		script,
		runtime.Context{
			Interface:        reusable.fvmEnv,
			Location:         location,
			UseVM:            useVM,
			Environment:      environment,
			MemoryGauge:      reusable.fvmEnv,
			ComputationGauge: reusable.fvmEnv,
		},
	)
}

func (reusable *ReusableCadenceRuntime) ExecuteScript(
	script runtime.Script,
	location common.Location,
	useVM bool,
) (
	cadence.Value,
	error,
) {
	var environment runtime.Environment
	if useVM {
		environment = reusable.VMScriptRuntimeEnv
	} else {
		environment = reusable.ScriptRuntimeEnv
	}

	return reusable.Runtime.ExecuteScript(
		script,
		runtime.Context{
			Interface:        reusable.fvmEnv,
			Location:         location,
			Environment:      environment,
			UseVM:            useVM,
			MemoryGauge:      reusable.fvmEnv,
			ComputationGauge: reusable.fvmEnv,
		},
	)
}

type CadenceRuntimeConstructor func(config runtime.Config) runtime.Runtime

type ReusableCadenceRuntimePool struct {
	pool chan *ReusableCadenceRuntime

	config runtime.Config

	// When newCustomRuntime is nil, the pool will create standard cadence
	// interpreter runtimes via runtime.NewRuntime.  Otherwise, the
	// pool will create runtimes using this function.
	//
	// Note that this is primarily used for testing.
	newCustomRuntime CadenceRuntimeConstructor
}

func newReusableCadenceRuntimePool(
	poolSize int,
	config runtime.Config,
	newCustomRuntime CadenceRuntimeConstructor,
) ReusableCadenceRuntimePool {
	var pool chan *ReusableCadenceRuntime
	if poolSize > 0 {
		pool = make(chan *ReusableCadenceRuntime, poolSize)
	}

	return ReusableCadenceRuntimePool{
		pool:             pool,
		config:           config,
		newCustomRuntime: newCustomRuntime,
	}
}

func NewReusableCadenceRuntimePool(
	poolSize int,
	config runtime.Config,
) ReusableCadenceRuntimePool {
	return newReusableCadenceRuntimePool(
		poolSize,
		config,
		nil,
	)
}

func NewCustomReusableCadenceRuntimePool(
	poolSize int,
	config runtime.Config,
	newCustomRuntime CadenceRuntimeConstructor,
) ReusableCadenceRuntimePool {
	return newReusableCadenceRuntimePool(
		poolSize,
		config,
		newCustomRuntime,
	)
}

func (pool ReusableCadenceRuntimePool) newRuntime() runtime.Runtime {
	if pool.newCustomRuntime != nil {
		return pool.newCustomRuntime(pool.config)
	}
	return runtime.NewRuntime(pool.config)
}

func (pool ReusableCadenceRuntimePool) Borrow(
	fvmEnv Environment,
) *ReusableCadenceRuntime {
	var reusable *ReusableCadenceRuntime
	select {
	case reusable = <-pool.pool:
		// Do nothing.
	default:
		reusable = NewReusableCadenceRuntime(
			WrappedCadenceRuntime{
				pool.newRuntime(),
			},
			pool.config,
		)
	}

	reusable.SetFvmEnvironment(fvmEnv)
	return reusable
}

func (pool ReusableCadenceRuntimePool) Return(
	reusable *ReusableCadenceRuntime,
) {
	reusable.SetFvmEnvironment(nil)
	select {
	case pool.pool <- reusable:
		// Do nothing.
	default:
		// Do nothing.  Discard the overflow entry.
	}
}
