package environment

// ReusableCadenceRuntimePool is the pool that holds ReusableCadenceRuntime-s so that they
// can be reused between procedures
type ReusableCadenceRuntimePool interface {
	Borrow(
		fvmEnv Environment,
		runtimeType CadenceRuntimeType,
	) ReusableCadenceRuntime
	Return(
		reusable ReusableCadenceRuntime,
	)
}

type RuntimeParams struct {
	ReusableCadenceRuntimePool
}

type cadenceRuntime struct {
	RuntimeParams

	env         Environment
	runtimeType CadenceRuntimeType
}

// CadenceRuntime exposes the cadence runtime to the rest of the environment package.
type CadenceRuntime interface {
	SetEnvironment(env Environment)
	BorrowCadenceRuntime() ReusableCadenceRuntime
	ReturnCadenceRuntime(reusable ReusableCadenceRuntime)
}

// CadenceRuntimeType is used to specify if a runtime will be used for scripts or transactions
type CadenceRuntimeType int

const (
	// CadenceScriptRuntime is a marker to indicate to use the cadence runtime set up for scripts
	CadenceScriptRuntime = iota
	// CadenceTransactionRuntime is a marker to indicate to use the cadence runtime set up for transactions
	CadenceTransactionRuntime
)

func NewRuntime(params RuntimeParams, runtimeType CadenceRuntimeType) *cadenceRuntime {
	return &cadenceRuntime{
		RuntimeParams: params,
		runtimeType:   runtimeType,
	}
}

func (runtime *cadenceRuntime) SetEnvironment(env Environment) {
	runtime.env = env
}

func (runtime *cadenceRuntime) BorrowCadenceRuntime() ReusableCadenceRuntime {
	return runtime.ReusableCadenceRuntimePool.Borrow(runtime.env, runtime.runtimeType)
}

func (runtime *cadenceRuntime) ReturnCadenceRuntime(
	reusable ReusableCadenceRuntime,
) {
	runtime.ReusableCadenceRuntimePool.Return(reusable)
}
