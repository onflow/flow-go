package environment

// ReusableCadenceRuntimePool is the pool that holds ReusableCadenceRuntime-s so that they
// can be reused between procedures
type ReusableCadenceRuntimePool interface {
	Borrow(
		fvmEnv Environment,
	) ReusableCadenceRuntime
	Return(
		reusable ReusableCadenceRuntime,
	)
}

type RuntimeParams struct {
	ReusableCadenceRuntimePool
}

// Runtime expose the cadence runtime to the rest of the environment package.
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

func (runtime *Runtime) BorrowCadenceRuntime() ReusableCadenceRuntime {
	return runtime.ReusableCadenceRuntimePool.Borrow(runtime.env)
}

func (runtime *Runtime) ReturnCadenceRuntime(
	reusable ReusableCadenceRuntime,
) {
	runtime.ReusableCadenceRuntimePool.Return(reusable)
}
