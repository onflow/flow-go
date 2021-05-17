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

const contractFunctionInvocationLocation = common.StringLocation("ContractFunctionInvocation")

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

func (i *TransactionContractFunctionInvocator) Invoke(env *hostEnv, proc *TransactionProcedure) (cadence.Value, error) {
	var span opentracing.Span
	if env.ctx.Tracer != nil && proc.TraceSpan != nil {
		span = env.ctx.Tracer.StartSpanFromParent(proc.TraceSpan, trace.FVMInvokeContractFunction)
		span.LogFields(
			traceLog.String("transaction.ContractFunctionCall", fmt.Sprintf("%s.%s", i.contractLocation.String(), i.functionName)),
		)
		defer span.Finish()
	}

	predeclaredValues := valueDeclarations(&env.ctx, env)

	value, err := env.vm.Runtime.InvokeContractFunction(
		i.contractLocation,
		i.functionName,
		i.arguments,
		i.argumentTypes,
		runtime.Context{
			Interface:         env,
			Location:          contractFunctionInvocationLocation,
			PredeclaredValues: predeclaredValues,
		},
	)

	if err != nil {
		i.logger.Warn().
			Msg("Contract function call executed with error")
	}
	return value, err
}
