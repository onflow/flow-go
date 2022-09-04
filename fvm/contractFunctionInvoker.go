package fvm

import (
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"
	"go.opentelemetry.io/otel/attribute"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// ContractFunctionSpec specify all the information, except the function's
// address and arguments, needed to invoke the contract function.
type ContractFunctionSpec struct {
	AddressFromChain func(flow.Chain) flow.Address
	LocationName     string
	FunctionName     string
	ArgumentTypes    []sema.Type
}

// SystemContracts provides methods for invoking system contract functions as
// service account.
type SystemContracts struct {
	env Environment
}

func NewSystemContracts() *SystemContracts {
	return &SystemContracts{}
}

func (sys *SystemContracts) SetEnvironment(env Environment) {
	sys.env = env
}

func (sys *SystemContracts) Invoke(
	spec ContractFunctionSpec,
	arguments []cadence.Value,
) (
	cadence.Value,
	error,
) {
	contractLocation := common.AddressLocation{
		Address: common.Address(spec.AddressFromChain(sys.env.Chain())),
		Name:    spec.LocationName,
	}

	span := sys.env.StartSpanFromRoot(trace.FVMInvokeContractFunction)
	span.SetAttributes(
		attribute.String(
			"transaction.ContractFunctionCall",
			contractLocation.String()+"."+spec.FunctionName))
	defer span.End()

	runtimeEnv := sys.env.BorrowCadenceRuntime()
	defer sys.env.ReturnCadenceRuntime(runtimeEnv)

	value, err := sys.env.VM().Runtime.InvokeContractFunction(
		contractLocation,
		spec.FunctionName,
		arguments,
		spec.ArgumentTypes,
		runtime.Context{
			Interface:   sys.env,
			Environment: runtimeEnv,
		},
	)

	if err != nil {
		// this is an error coming from Cadendce runtime, so it must be handled first.
		err = errors.HandleRuntimeError(err)
		sys.env.Logger().
			Info().
			Err(err).
			Str("contract", contractLocation.String()).
			Str("function", spec.FunctionName).
			Msg("Contract function call executed with error")
	}
	return value, err
}
