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

func (i *TransactionContractFunctionInvoker) Invoke(env Environment, parentTraceSpan opentracing.Span) (value cadence.Value, err error) {
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

	// Recover panic coming from InvokeContractFunction and return it as an error.
	// This error will get properly handled down the line.
	// If this panic is not caught the node will crash.
	// Ideally ony user errors would be caught here, or better, no panics would reach this point.
	defer func() {
		if r := recover(); r != nil {
			if recoveredError, ok := r.(error); ok {
				err = recoveredError
				i.logger.Error().Err(recoveredError).Msgf("InvokeContractFunction panic recovered")
				return
			}

			panic(r)
		}
	}()

	value, err = env.VM().Runtime.InvokeContractFunction(
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
		i.logger.Warn().
			Msg("Contract function call executed with error")
	}
	return value, err
}
