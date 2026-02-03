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

type cadenceRuntimeProvider struct {
	RuntimeParams

	env         Environment
	runtimeType CadenceRuntimeType
}

// CadenceRuntimeProvider exposes the cadence runtime to the rest of the environment package.
type CadenceRuntimeProvider interface {
	// SetEnvironment sets the fvm Environment that will be injected into a ReusableCadenceRuntime
	// when it is borrowed
	SetEnvironment(env Environment)
	// BorrowCadenceRuntime borrows a runtime from the ReusableCadenceRuntimePool moving it so its only used in
	// this procedure until returned
	BorrowCadenceRuntime() ReusableCadenceRuntime
	// ReturnCadenceRuntime returns a runtime from the ReusableCadenceRuntimePool so that it can be reused
	// for a different procedure later.
	ReturnCadenceRuntime(reusable ReusableCadenceRuntime)
}

// CadenceRuntimeType is used to specify if a runtime will be used for scripts or transactions
type CadenceRuntimeType int

const (
	// CadenceScriptRuntime is a marker to indicate to use the cadence runtime set up for scripts
	CadenceScriptRuntime CadenceRuntimeType = iota
	// CadenceTransactionRuntime is a marker to indicate to use the cadence runtime set up for transactions
	CadenceTransactionRuntime
)

func NewRuntime(params RuntimeParams, runtimeType CadenceRuntimeType) *cadenceRuntimeProvider {
	return &cadenceRuntimeProvider{
		RuntimeParams: params,
		runtimeType:   runtimeType,
	}
}

func (runtime *cadenceRuntimeProvider) SetEnvironment(env Environment) {
	runtime.env = env
}

func (runtime *cadenceRuntimeProvider) BorrowCadenceRuntime() ReusableCadenceRuntime {
	return runtime.ReusableCadenceRuntimePool.Borrow(runtime.env, runtime.runtimeType)
}

func (runtime *cadenceRuntimeProvider) ReturnCadenceRuntime(
	reusable ReusableCadenceRuntime,
) {
	runtime.ReusableCadenceRuntimePool.Return(reusable)
}
