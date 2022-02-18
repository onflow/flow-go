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

	"github.com/onflow/flow-go/module/trace"
)

type TransactionContractFunctionInvoker struct {
	contractLocation common.AddressLocation
	functionName     string
	arguments        []interpreter.Value
	argumentTypes    []sema.Type
	logger           zerolog.Logger
	logSpanFields    []traceLog.Field
}

func NewTransactionContractFunctionInvoker(
	contractLocation common.AddressLocation,
	functionName string,
	arguments []interpreter.Value,
	argumentTypes []sema.Type,
	logger zerolog.Logger) *TransactionContractFunctionInvoker {
	return &TransactionContractFunctionInvoker{
		contractLocation: contractLocation,
		functionName:     functionName,
		arguments:        arguments,
		argumentTypes:    argumentTypes,
		logger:           logger,
		logSpanFields:    []traceLog.Field{traceLog.String("transaction.ContractFunctionCall", fmt.Sprintf("%s.%s", contractLocation.String(), functionName))},
	}
}

func (i *TransactionContractFunctionInvoker) Invoke(env Environment, parentTraceSpan opentracing.Span) (cadence.Value, error) {
	var span opentracing.Span

	ctx := env.Context()
	if ctx.Tracer != nil && parentTraceSpan != nil {
		span = ctx.Tracer.StartSpanFromParent(parentTraceSpan, trace.FVMInvokeContractFunction)
		span.LogFields(
			i.logSpanFields...,
		)
		defer span.Finish()
	}

	predeclaredValues := valueDeclarations(ctx, env)

	value, err := env.VM().Runtime.InvokeContractFunction(
		i.contractLocation,
		i.functionName,
		i.arguments,
		i.argumentTypes,
		runtime.Context{
			Interface:         env,
			PredeclaredValues: predeclaredValues,
		},
	)

	if err != nil {
		i.logger.
			Info().
			Err(err).
			Str("contract", i.contractLocation.String()).
			Str("function", i.functionName).
			Msg("Contract function call executed with error")
	}
	return value, err
}
