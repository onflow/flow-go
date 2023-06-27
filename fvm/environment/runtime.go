package environment

import (
	cadenceRuntime "github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/runtime"
)

type RuntimeParams struct {
	runtime.ReusableCadenceRuntimePool
}

func DefaultRuntimeParams() RuntimeParams {
	return RuntimeParams{
		ReusableCadenceRuntimePool: runtime.NewReusableCadenceRuntimePool(
			0,
			cadenceRuntime.Config{},
		),
	}
}

// Runtime expose the cadence runtime to the rest of the envionment package.
type Runtime struct {
	RuntimeParams

	env Environment
}

func NewRuntime(params RuntimeParams) *Runtime {
	return &Runtime{
		RuntimeParams: params,
	}
}

func (runtime *Runtime) SetEnvironment(env Environment) {
	runtime.env = env
}

func (runtime *Runtime) BorrowCadenceRuntime() *runtime.ReusableCadenceRuntime {
	return runtime.ReusableCadenceRuntimePool.Borrow(runtime.env)
}

func (runtime *Runtime) ReturnCadenceRuntime(
	reusable *runtime.ReusableCadenceRuntime,
) {
	runtime.ReusableCadenceRuntimePool.Return(reusable)
}
