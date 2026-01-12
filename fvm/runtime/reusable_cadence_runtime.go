package runtime

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/sema"

	"github.com/onflow/flow-go/fvm/environment"
)

type ReusableCadenceRuntime struct {
	runtime.Runtime
	TxRuntimeEnv     runtime.Environment
	ScriptRuntimeEnv runtime.Environment

	fvmEnv environment.Environment
}

var _ environment.ReusableCadenceRuntime = (*ReusableCadenceRuntime)(nil)

func NewReusableCadenceRuntime(
	rt runtime.Runtime,
	config runtime.Config,
) *ReusableCadenceRuntime {
	reusable := &ReusableCadenceRuntime{
		Runtime:          rt,
		TxRuntimeEnv:     runtime.NewBaseInterpreterEnvironment(config),
		ScriptRuntimeEnv: runtime.NewScriptInterpreterEnvironment(config),
	}

	reusable.declareStandardLibraryFunctions()

	return reusable
}

func (reusable *ReusableCadenceRuntime) declareStandardLibraryFunctions() {
	reusable.TxRuntimeEnv.DeclareValue(blockRandomSourceDeclaration(reusable), nil)

	declaration := transactionIndexDeclaration(reusable)
	reusable.TxRuntimeEnv.DeclareValue(declaration, nil)
	reusable.ScriptRuntimeEnv.DeclareValue(declaration, nil)

}

func (reusable *ReusableCadenceRuntime) SetFvmEnvironment(fvmEnv environment.Environment) {
	reusable.fvmEnv = fvmEnv
}

func (reusable *ReusableCadenceRuntime) CadenceTXEnv() runtime.Environment {
	return reusable.TxRuntimeEnv
}

func (reusable *ReusableCadenceRuntime) CadenceScriptEnv() runtime.Environment {
	return reusable.ScriptRuntimeEnv
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
			Interface:        reusable.fvmEnv,
			Environment:      reusable.TxRuntimeEnv,
			MemoryGauge:      reusable.fvmEnv,
			ComputationGauge: reusable.fvmEnv,
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
			Interface:        reusable.fvmEnv,
			Environment:      reusable.TxRuntimeEnv,
			MemoryGauge:      reusable.fvmEnv,
			ComputationGauge: reusable.fvmEnv,
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
			Interface:        reusable.fvmEnv,
			Location:         location,
			Environment:      reusable.TxRuntimeEnv,
			MemoryGauge:      reusable.fvmEnv,
			ComputationGauge: reusable.fvmEnv,
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
			Interface:        reusable.fvmEnv,
			Location:         location,
			Environment:      reusable.ScriptRuntimeEnv,
			MemoryGauge:      reusable.fvmEnv,
			ComputationGauge: reusable.fvmEnv,
		},
	)
}
