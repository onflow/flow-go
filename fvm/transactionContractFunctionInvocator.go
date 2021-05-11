package fvm

import (
	"fmt"

	"github.com/opentracing/opentracing-go"
	traceLog "github.com/opentracing/opentracing-go/log"
	"github.com/rs/zerolog"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"

	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/module/trace"
)

type TransactionContractFunctionInvocator struct {
	contractLocation common.AddressLocation
	functionName     string
	arguments        []interpreter.Value
	argumentTypes    []sema.Type
	logger           zerolog.Logger
}

func NewTransactionContractFunctionInvocator(
	contractLocation common.AddressLocation,
	functionName string,
	arguments []interpreter.Value,
	argumentTypes []sema.Type,
	logger zerolog.Logger) *TransactionContractFunctionInvocator {
	return &TransactionContractFunctionInvocator{
		contractLocation: contractLocation,
		functionName:     functionName,
		arguments:        arguments,
		argumentTypes:    argumentTypes,
		logger:           logger,
	}
}

func (i *TransactionContractFunctionInvocator) Invoke(vm *VirtualMachine, ctx *Context, parentSpan opentracing.Span, sth *state.StateHolder, programs *programs.Programs) (cadence.Value, error) {
	var span opentracing.Span
	if ctx.Tracer != nil && parentSpan != nil {
		span = ctx.Tracer.StartSpanFromParent(parentSpan, trace.FVMInvokeContractFunction)
		span.LogFields(
			traceLog.String("transaction.ContractFunctionCall", fmt.Sprintf("%s.%s", i.contractLocation.String(), i.functionName)),
		)
		defer span.Finish()
	}

	env := newEnvironment(*ctx, vm, sth, programs)
	predeclaredValues := valueDeclarations(ctx, env)

	env.setTraceSpan(span)
	location := common.StringLocation("ContractFunctionInvocation")

	value, err := vm.Runtime.InvokeContractFunction(
		i.contractLocation,
		i.functionName,
		i.arguments,
		i.argumentTypes,
		runtime.Context{
			Interface:         env,
			Location:          location,
			PredeclaredValues: predeclaredValues,
		},
	)

	if err != nil {
		i.logger.Info().
			Msg("Contract function call executed with error")
	}
	return value, err
}
