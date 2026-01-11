package runtime

import (
	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/model/flow"
)

type CadenceRuntimeConstructor func(config runtime.Config) runtime.Runtime

type ReusableCadenceRuntimePool struct {
	pool chan *ReusableCadenceRuntime

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

func newReusableCadenceRuntimePool(
	poolSize int,
	chain flow.Chain,
	config runtime.Config,
	newCustomRuntime CadenceRuntimeConstructor,
) ReusableCadenceRuntimePool {
	var pool chan *ReusableCadenceRuntime
	if poolSize > 0 {
		pool = make(chan *ReusableCadenceRuntime, poolSize)
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
			pool.runtimeConfig,
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
