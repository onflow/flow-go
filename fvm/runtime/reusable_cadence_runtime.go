package runtime

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
)

// Note: this is a subset of environment.Environment, redeclared to handle
// circular dependency.
type Environment interface {
	runtime.Interface

	SetAccountFrozen(address flow.Address, frozen bool) error
}

var setAccountFrozenFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          sema.ArgumentLabelNotRequired,
			Identifier:     "account",
			TypeAnnotation: sema.NewTypeAnnotation(sema.TheAddressType),
		},
		{
			Label:          sema.ArgumentLabelNotRequired,
			Identifier:     "frozen",
			TypeAnnotation: sema.NewTypeAnnotation(sema.BoolType),
		},
	},
	ReturnTypeAnnotation: sema.TypeAnnotation{
		Type: sema.VoidType,
	},
}

type ReusableCadenceRuntime struct {
	runtime.Runtime
	runtime.Environment

	fvmEnv Environment
}

func NewReusableCadenceRuntime(rt runtime.Runtime, config runtime.Config) *ReusableCadenceRuntime {
	reusable := &ReusableCadenceRuntime{
		Runtime:     rt,
		Environment: runtime.NewBaseInterpreterEnvironment(config),
	}

	setAccountFrozen := stdlib.StandardLibraryValue{
		Name: "setAccountFrozen",
		Type: setAccountFrozenFunctionType,
		Kind: common.DeclarationKindFunction,
		Value: interpreter.NewUnmeteredHostFunctionValue(
			setAccountFrozenFunctionType,
			func(invocation interpreter.Invocation) interpreter.Value {
				address, ok := invocation.Arguments[0].(interpreter.AddressValue)
				if !ok {
					panic(errors.NewValueErrorf(invocation.Arguments[0].String(),
						"first argument of setAccountFrozen must be an address"))
				}

				frozen, ok := invocation.Arguments[1].(interpreter.BoolValue)
				if !ok {
					panic(errors.NewValueErrorf(invocation.Arguments[0].String(),
						"second argument of setAccountFrozen must be a boolean"))
				}

				var err error
				if reusable.fvmEnv != nil {
					err = reusable.fvmEnv.SetAccountFrozen(
						flow.ConvertAddress(address),
						bool(frozen))
				} else {
					err = errors.NewOperationNotSupportedError("SetAccountFrozen")
				}

				if err != nil {
					panic(err)
				}

				return interpreter.VoidValue{}
			},
		),
	}

	reusable.Declare(setAccountFrozen)
	return reusable
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
			Environment: reusable.Environment,
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
			Environment: reusable.Environment,
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
			Environment: reusable.Environment,
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
			Interface: reusable.fvmEnv,
			Location:  location,
		},
	)
}

type CadenceRuntimeConstructor func(config runtime.Config) runtime.Runtime

type ReusableCadenceRuntimePool struct {
	pool chan *ReusableCadenceRuntime

	config runtime.Config

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
	newCustomRuntime CadenceRuntimeConstructor,
) ReusableCadenceRuntimePool {
	return newReusableCadenceRuntimePool(
		poolSize,
		runtime.Config{},
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
