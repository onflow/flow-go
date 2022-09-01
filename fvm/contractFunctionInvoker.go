package fvm

import (
	"go.opentelemetry.io/otel/attribute"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/module/trace"
)

type ContractFunctionInvoker struct {
	contractLocation common.AddressLocation
	functionName     string
	arguments        []cadence.Value
	argumentTypes    []sema.Type
	logSpanAttrs     []attribute.KeyValue
}

func NewContractFunctionInvoker(
	contractLocation common.AddressLocation,
	functionName string,
	arguments []cadence.Value,
	argumentTypes []sema.Type,
) *ContractFunctionInvoker {
	return &ContractFunctionInvoker{
		contractLocation: contractLocation,
		functionName:     functionName,
		arguments:        arguments,
		argumentTypes:    argumentTypes,
		logSpanAttrs: []attribute.KeyValue{
			attribute.String("transaction.ContractFunctionCall", contractLocation.String()+"."+functionName),
		},
	}
}

func (i *ContractFunctionInvoker) Invoke(env Environment) (cadence.Value, error) {

	span := env.StartSpanFromRoot(trace.FVMInvokeContractFunction)
	span.SetAttributes(i.logSpanAttrs...)
	defer span.End()

	runtimeEnv := env.BorrowCadenceRuntime()
	defer env.ReturnCadenceRuntime(runtimeEnv)

	value, err := env.VM().Runtime.InvokeContractFunction(
		i.contractLocation,
		i.functionName,
		i.arguments,
		i.argumentTypes,
		runtime.Context{
			Interface:   env,
			Environment: runtimeEnv,
		},
	)

	if err != nil {
		// this is an error coming from Cadendce runtime, so it must be handled first.
		err = errors.HandleRuntimeError(err)
		env.Context().Logger.
			Info().
			Err(err).
			Str("contract", i.contractLocation.String()).
			Str("function", i.functionName).
			Msg("Contract function call executed with error")
	}
	return value, err
}
