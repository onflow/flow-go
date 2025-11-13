package environment

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	cadenceRuntime "github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/sema"

	"github.com/onflow/flow-go/fvm/runtime"
)

type CadenceRuntimeConstructor func(config cadenceRuntime.Config) cadenceRuntime.Runtime

type RuntimeParams struct {
	cadenceRuntime.Config
	NewRuntime CadenceRuntimeConstructor
}

func DefaultRuntimeParams() RuntimeParams {
	return RuntimeParams{}
}

// Runtime expose the Cadence runtime to the rest of the environment package.
type Runtime struct {
	CadenceRuntime cadenceRuntime.Runtime

	TxEnv     cadenceRuntime.Environment
	ScriptEnv cadenceRuntime.Environment

	FvmEnv Environment
}

func NewRuntime(params RuntimeParams) *Runtime {
	runtimeConfig := params.Config

	var cadenceRT cadenceRuntime.Runtime
	if params.NewRuntime != nil {
		cadenceRT = params.NewRuntime(runtimeConfig)
	} else {
		cadenceRT = runtime.WrappedCadenceRuntime{
			Runtime: cadenceRuntime.NewRuntime(runtimeConfig),
		}
	}

	txEnv := cadenceRuntime.NewBaseInterpreterEnvironment(runtimeConfig)
	scriptEnv := cadenceRuntime.NewScriptInterpreterEnvironment(runtimeConfig)

	rt := &Runtime{
		CadenceRuntime: cadenceRT,
		TxEnv:          txEnv,
		ScriptEnv:      scriptEnv,
	}

	declareRandomSourceHistory(txEnv, rt)

	return rt
}

func (runtime *Runtime) SetFvmEnvironment(fvmEnv Environment) {
	runtime.FvmEnv = fvmEnv
}

func (runtime *Runtime) ReadStored(
	address common.Address,
	path cadence.Path,
) (
	cadence.Value,
	error,
) {
	return runtime.CadenceRuntime.ReadStored(
		address,
		path,
		cadenceRuntime.Context{
			Interface:        runtime.FvmEnv,
			Environment:      runtime.TxEnv,
			MemoryGauge:      runtime.FvmEnv,
			ComputationGauge: runtime.FvmEnv,
		},
	)
}

func (runtime *Runtime) InvokeContractFunction(
	contractLocation common.AddressLocation,
	functionName string,
	arguments []cadence.Value,
	argumentTypes []sema.Type,
) (
	cadence.Value,
	error,
) {
	return runtime.CadenceRuntime.InvokeContractFunction(
		contractLocation,
		functionName,
		arguments,
		argumentTypes,
		cadenceRuntime.Context{
			Interface:        runtime.FvmEnv,
			Environment:      runtime.TxEnv,
			MemoryGauge:      runtime.FvmEnv,
			ComputationGauge: runtime.FvmEnv,
		},
	)
}

func (runtime *Runtime) NewTransactionExecutor(
	script cadenceRuntime.Script,
	location common.Location,
) cadenceRuntime.Executor {
	return runtime.CadenceRuntime.NewTransactionExecutor(
		script,
		cadenceRuntime.Context{
			Interface:        runtime.FvmEnv,
			Location:         location,
			Environment:      runtime.TxEnv,
			MemoryGauge:      runtime.FvmEnv,
			ComputationGauge: runtime.FvmEnv,
		},
	)
}

func (runtime *Runtime) ExecuteScript(
	script cadenceRuntime.Script,
	location common.Location,
) (
	cadence.Value,
	error,
) {
	return runtime.CadenceRuntime.ExecuteScript(
		script,
		cadenceRuntime.Context{
			Interface:        runtime.FvmEnv,
			Location:         location,
			Environment:      runtime.ScriptEnv,
			MemoryGauge:      runtime.FvmEnv,
			ComputationGauge: runtime.FvmEnv,
		},
	)
}

func (runtime *Runtime) RandomSourceHistory() ([]byte, error) {
	return runtime.FvmEnv.RandomSourceHistory()
}
