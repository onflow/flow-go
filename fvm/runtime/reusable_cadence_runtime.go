package runtime

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/sema"
	"github.com/onflow/cadence/stdlib"

	"github.com/onflow/flow-go/fvm/accountV2Migration"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
)

// Note: this is a subset of environment.Environment, redeclared to handle
// circular dependency.
type Environment interface {
	runtime.Interface

	RandomSourceHistory() ([]byte, error)
}

// randomSourceFunctionType is the type of the `randomSource` function.
// This defines the signature as `func(): [UInt8]`
var randomSourceFunctionType = &sema.FunctionType{
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.ByteArrayType),
}

type ReusableCadenceRuntime struct {
	runtime.Runtime
	TxRuntimeEnv     runtime.Environment
	ScriptRuntimeEnv runtime.Environment

	fvmEnv Environment
}

func NewReusableCadenceRuntime(
	rt runtime.Runtime,
	config runtime.Config,
	chainID flow.ChainID,
) *ReusableCadenceRuntime {
	reusable := &ReusableCadenceRuntime{
		Runtime:          rt,
		TxRuntimeEnv:     runtime.NewBaseInterpreterEnvironment(config),
		ScriptRuntimeEnv: runtime.NewScriptInterpreterEnvironment(config),
	}

	reusable.declareRandomSourceHistory()
	accountV2Migration.DeclareFunctions(reusable.TxRuntimeEnv, chainID)

	return reusable
}

func (reusable *ReusableCadenceRuntime) declareRandomSourceHistory() {

	// Declare the `randomSourceHistory` function. This function is **only** used by the
	// System transaction, to fill the `RandomBeaconHistory` contract via the heartbeat
	// resource. This allows the `RandomBeaconHistory` contract to be a standard contract,
	// without any special parts.
	// Since the `randomSourceHistory` function is only used by the System transaction,
	// it is not part of the cadence standard library, and can just be injected from here.
	// It also doesnt need user documentation, since it is not (and should not)
	// be called by the user. If it is called by the user it will panic.
	functionType := randomSourceFunctionType

	blockRandomSource := stdlib.StandardLibraryValue{
		Name: "randomSourceHistory",
		Type: functionType,
		Kind: common.DeclarationKindFunction,
		Value: interpreter.NewUnmeteredStaticHostFunctionValue(
			functionType,
			func(invocation interpreter.Invocation) interpreter.Value {

				actualArgumentCount := len(invocation.Arguments)
				expectedArgumentCount := len(functionType.Parameters)

				if actualArgumentCount != expectedArgumentCount {
					panic(errors.NewInvalidArgumentErrorf(
						"incorrect number of arguments: got %d, expected %d",
						actualArgumentCount,
						expectedArgumentCount,
					))
				}

				var err error
				var source []byte
				fvmEnv := reusable.fvmEnv
				if fvmEnv != nil {
					source, err = fvmEnv.RandomSourceHistory()
				} else {
					err = errors.NewOperationNotSupportedError("randomSourceHistory")
				}

				if err != nil {
					panic(err)
				}

				return interpreter.ByteSliceToByteArrayValue(
					invocation.Interpreter,
					source)
			},
		),
	}

	reusable.TxRuntimeEnv.DeclareValue(blockRandomSource, nil)
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
			Interface:   reusable.fvmEnv,
			Environment: reusable.TxRuntimeEnv,
		},
	)
}

func (reusable *ReusableCadenceRuntime) InvokeContractFunction(
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
			Interface:   reusable.fvmEnv,
			Environment: reusable.TxRuntimeEnv,
		},
	)
}

func (reusable *ReusableCadenceRuntime) NewTransactionExecutor(
	script runtime.Script,
	location common.Location,
) runtime.Executor {
	return reusable.Runtime.NewTransactionExecutor(
		script,
		runtime.Context{
			Interface:   reusable.fvmEnv,
			Location:    location,
			Environment: reusable.TxRuntimeEnv,
		},
	)
}

func (reusable *ReusableCadenceRuntime) ExecuteScript(
	script runtime.Script,
	location common.Location,
) (
	cadence.Value,
	error,
) {
	return reusable.Runtime.ExecuteScript(
		script,
		runtime.Context{
			Interface:   reusable.fvmEnv,
			Location:    location,
			Environment: reusable.ScriptRuntimeEnv,
		},
	)
}

type CadenceRuntimeConstructor func(config runtime.Config) runtime.Runtime

type ReusableCadenceRuntimePool struct {
	pool chan *ReusableCadenceRuntime

	config runtime.Config

	chainID flow.ChainID

	// When newCustomRuntime is nil, the pool will create standard cadence
	// interpreter runtimes via runtime.NewInterpreterRuntime.  Otherwise, the
	// pool will create runtimes using this function.
	//
	// Note that this is primarily used for testing.
	newCustomRuntime CadenceRuntimeConstructor
}

func newReusableCadenceRuntimePool(
	poolSize int,
	config runtime.Config,
	chainID flow.ChainID,
	newCustomRuntime CadenceRuntimeConstructor,
) ReusableCadenceRuntimePool {
	var pool chan *ReusableCadenceRuntime
	if poolSize > 0 {
		pool = make(chan *ReusableCadenceRuntime, poolSize)
	}

	return ReusableCadenceRuntimePool{
		pool:             pool,
		config:           config,
		chainID:          chainID,
		newCustomRuntime: newCustomRuntime,
	}
}

func NewReusableCadenceRuntimePool(
	poolSize int,
	config runtime.Config,
	chainID flow.ChainID,
) ReusableCadenceRuntimePool {
	return newReusableCadenceRuntimePool(
		poolSize,
		config,
		chainID,
		nil,
	)
}

func NewCustomReusableCadenceRuntimePool(
	poolSize int,
	config runtime.Config,
	chainID flow.ChainID,
	newCustomRuntime CadenceRuntimeConstructor,
) ReusableCadenceRuntimePool {
	return newReusableCadenceRuntimePool(
		poolSize,
		config,
		chainID,
		newCustomRuntime,
	)
}

func (pool ReusableCadenceRuntimePool) newRuntime() runtime.Runtime {
	if pool.newCustomRuntime != nil {
		return pool.newCustomRuntime(pool.config)
	}
	return runtime.NewInterpreterRuntime(pool.config)
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
			pool.chainID,
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
