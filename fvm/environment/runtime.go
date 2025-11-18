package environment

import (
	cadenceRuntime "github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/runtime"
)

type RuntimeParams struct {
	runtime.ReusableCadenceRuntimePool
	ConfigureCadenceRuntime func(
		reusableCadenceRuntime *runtime.ReusableCadenceRuntime,
		env Environment,
	)
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
	env := runtime.env

	reusableCadenceRuntime := runtime.ReusableCadenceRuntimePool.Borrow(env)

	configure := runtime.ConfigureCadenceRuntime
	if configure != nil {
		configure(reusableCadenceRuntime, env)
	}

	return reusableCadenceRuntime
}

func (runtime *Runtime) ReturnCadenceRuntime(
	reusable *runtime.ReusableCadenceRuntime,
) {
	runtime.ReusableCadenceRuntimePool.Return(reusable)
}
