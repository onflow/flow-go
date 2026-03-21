package runtime

import (
	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
)

type CadenceRuntimeConstructor func(config runtime.Config) runtime.Runtime

type ReusableCadenceRuntimePool struct {
	pool chan *reusableCadenceRuntime

	runtimeConfig runtime.Config

	// When customRuntimeConstructor is nil, the pool will create standard cadence
	// interpreter runtimes via runtime.NewRuntime.  Otherwise, the
	// pool will create runtimes using this function.
	//
	// Note that this is primarily used for testing.
	customRuntimeConstructor CadenceRuntimeConstructor

	// chain is the chain the RuntimePool was made for
	// while cadence runtime is chain agnostic, some injected (into cadence) FVM definitions
	// are chain dependant. The pool should not be used cross chain.
	// Using the pool to execute procedures of a different chain will produce errors
	chain flow.Chain
}

var _ environment.ReusableCadenceRuntimePool = (*ReusableCadenceRuntimePool)(nil)

func newReusableCadenceRuntimePool(
	poolSize int,
	chain flow.Chain,
	config runtime.Config,
	newCustomRuntime CadenceRuntimeConstructor,
) ReusableCadenceRuntimePool {
	var pool chan *reusableCadenceRuntime
	if poolSize > 0 {
		pool = make(chan *reusableCadenceRuntime, poolSize)
	}

	return ReusableCadenceRuntimePool{
		pool:                     pool,
		chain:                    chain,
		runtimeConfig:            config,
		customRuntimeConstructor: newCustomRuntime,
	}
}

func NewReusableCadenceRuntimePool(
	poolSize int,
	chain flow.Chain,
	config runtime.Config,
) ReusableCadenceRuntimePool {
	return newReusableCadenceRuntimePool(
		poolSize,
		chain,
		config,
		nil,
	)
}

func NewCustomReusableCadenceRuntimePool(
	poolSize int,
	chain flow.Chain,
	config runtime.Config,
	newCustomRuntime CadenceRuntimeConstructor,
) ReusableCadenceRuntimePool {
	return newReusableCadenceRuntimePool(
		poolSize,
		chain,
		config,
		newCustomRuntime,
	)
}

func (pool ReusableCadenceRuntimePool) newRuntime() runtime.Runtime {
	if pool.customRuntimeConstructor != nil {
		return pool.customRuntimeConstructor(pool.runtimeConfig)
	}
	return runtime.NewRuntime(pool.runtimeConfig)
}

func (pool ReusableCadenceRuntimePool) Borrow(
	fvmEnv environment.Environment,
	runtimeType environment.CadenceRuntimeType,
) environment.ReusableCadenceRuntime {
	var reusable *reusableCadenceRuntime
	select {
	case reusable = <-pool.pool:
		// Do nothing.
	default:
		reusable = newReusableCadenceRuntime(
			WrappedCadenceRuntime{
				pool.newRuntime(),
			},
			pool.chain,
			pool.runtimeConfig,
		)
	}

	reusable.SetFvmEnvironment(fvmEnv)

	switch runtimeType {
	case environment.CadenceScriptRuntime:
		return reusableCadenceScriptRuntime{
			reusableCadenceRuntime: reusable,
		}
	case environment.CadenceTransactionRuntime:
		return reusableCadenceTransactionRuntime{
			reusableCadenceRuntime: reusable,
		}
	default:
		panic("unreachable")

	}
}

func (pool ReusableCadenceRuntimePool) Return(
	reusable environment.ReusableCadenceRuntime,
) {
	var inner *reusableCadenceRuntime
	switch v := reusable.(type) {
	case reusableCadenceScriptRuntime:
		inner = v.reusableCadenceRuntime
	case reusableCadenceTransactionRuntime:
		inner = v.reusableCadenceRuntime
	default:
		panic("unreachable")
	}
	inner.SetFvmEnvironment(nil)

	select {
	case pool.pool <- inner:
		// Do nothing.
	default:
		// Do nothing.  Discard the overflow entry.
	}
}

func DefaultRuntimeParams(chain flow.Chain) environment.RuntimeParams {
	return environment.RuntimeParams{
		ReusableCadenceRuntimePool: NewReusableCadenceRuntimePool(
			0,
			chain,
			runtime.Config{},
		),
	}
}
