package environment

import (
	"github.com/onflow/flow-go/fvm/runtime"
)

// Runtime expose the cadence runtime to the rest of the envionment package.
type Runtime struct {
	pool runtime.ReusableCadenceRuntimePool

	env Environment
}

func NewRuntime(pool runtime.ReusableCadenceRuntimePool) *Runtime {
	return &Runtime{
		pool: pool,
	}
}

func (runtime *Runtime) SetEnvironment(env Environment) {
	runtime.env = env
}

func (runtime *Runtime) BorrowCadenceRuntime() *runtime.ReusableCadenceRuntime {
	return runtime.pool.Borrow(runtime.env)
}

func (runtime *Runtime) ReturnCadenceRuntime(
	reusable *runtime.ReusableCadenceRuntime,
) {
	runtime.pool.Return(reusable)
}
