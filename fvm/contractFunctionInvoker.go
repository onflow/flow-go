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
	LocationName  string
	FunctionName  string
	ArgumentTypes []sema.Type
}

type ContractFunctionInvoker struct {
	env Environment
}

func NewContractFunctionInvoker(
	env Environment,
) *ContractFunctionInvoker {
	return &ContractFunctionInvoker{
		env: env,
	}
}

func (i *ContractFunctionInvoker) Invoke(
	spec ContractFunctionSpec,
	address flow.Address,
	arguments []cadence.Value,
) (
	cadence.Value,
	error,
) {
	contractLocation := common.AddressLocation{
		Address: common.Address(address),
		Name:    spec.LocationName,
	}

	span := i.env.StartSpanFromRoot(trace.FVMInvokeContractFunction)
	span.SetAttributes(
		attribute.String(
			"transaction.ContractFunctionCall",
			contractLocation.String()+"."+spec.FunctionName))
	defer span.End()

	runtimeEnv := i.env.BorrowCadenceRuntime()
	defer i.env.ReturnCadenceRuntime(runtimeEnv)

	value, err := i.env.VM().Runtime.InvokeContractFunction(
		contractLocation,
		spec.FunctionName,
		arguments,
		spec.ArgumentTypes,
		runtime.Context{
			Interface:   i.env,
			Environment: runtimeEnv,
		},
	)

	if err != nil {
		// this is an error coming from Cadendce runtime, so it must be handled first.
		err = errors.HandleRuntimeError(err)
		i.env.Logger().
			Info().
			Err(err).
			Str("contract", contractLocation.String()).
			Str("function", spec.FunctionName).
			Msg("Contract function call executed with error")
	}
	return value, err
}
