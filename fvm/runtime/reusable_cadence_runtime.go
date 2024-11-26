package runtime

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	cadenceErrors "github.com/onflow/cadence/errors"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/sema"
	"github.com/onflow/cadence/stdlib"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

// Note: this is a subset of environment.Environment, redeclared to handle
// circular dependency.
type Environment interface {
	runtime.Interface

	RandomSourceHistory() ([]byte, error)
}

// randomSourceFunctionType is the type of the `randomSource` function.
// This defies the signature as `func (): [UInt8]`
var randomSourceFunctionType = &sema.FunctionType{
	Parameters:           []sema.Parameter{},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.ByteArrayType),
}

// scheduleAccountV2MigrationType is the type of the `scheduleAccountV2Migration` function.
// This defies the signature as `func (address: Address): Bool`
var scheduleAccountV2MigrationType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Identifier:     "address",
			TypeAnnotation: sema.AddressTypeAnnotation,
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.BoolType),
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
	reusable.declareScheduleAccountV2Migration(chainID)

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
	blockRandomSource := stdlib.StandardLibraryValue{
		Name: "randomSourceHistory",
		Type: randomSourceFunctionType,
		Kind: common.DeclarationKindFunction,
		Value: interpreter.NewUnmeteredStaticHostFunctionValue(
			randomSourceFunctionType,
			func(invocation interpreter.Invocation) interpreter.Value {
				if len(invocation.Arguments) != 0 {
					panic(errors.NewInvalidArgumentErrorf(
						"randomSourceHistory should be called without arguments"))
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

func (reusable *ReusableCadenceRuntime) declareScheduleAccountV2Migration(chainID flow.ChainID) {

	serviceAccount := systemcontracts.SystemContractsForChain(chainID).FlowServiceAccount

	blockRandomSource := stdlib.StandardLibraryValue{
		Name: "scheduleAccountV2Migration",
		Type: scheduleAccountV2MigrationType,
		Kind: common.DeclarationKindFunction,
		Value: interpreter.NewUnmeteredStaticHostFunctionValue(
			scheduleAccountV2MigrationType,
			func(invocation interpreter.Invocation) interpreter.Value {
				if len(invocation.Arguments) != 1 {
					panic(errors.NewInvalidArgumentErrorf(
						"scheduleAccountV2Migration should be called with exactly one argument of type Address",
					))
				}

				addressValue, ok := invocation.Arguments[0].(interpreter.AddressValue)
				if !ok {
					panic(errors.NewInvalidArgumentErrorf(
						"scheduleAccountV2Migration should be called with exactly one argument of type Address",
					))
				}

				storage := invocation.Interpreter.Storage()

				runtimeStorage, ok := storage.(*runtime.Storage)
				if !ok {
					panic(cadenceErrors.NewUnexpectedError("interpreter storage is not a runtime.Storage"))
				}

				result := runtimeStorage.ScheduleV2Migration(common.Address(addressValue))

				return interpreter.AsBoolValue(result)
			},
		),
	}

	accountV2MigrationLocation := common.NewAddressLocation(
		nil,
		common.Address(serviceAccount.Address),
		"AccountV2Migration",
	)

	reusable.TxRuntimeEnv.DeclareValue(
		blockRandomSource,
		accountV2MigrationLocation,
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
