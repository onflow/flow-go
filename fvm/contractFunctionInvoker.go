package fvm

import (
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"

	"github.com/onflow/flow-go/module/trace"
)

type ContractFunctionInvoker struct {
	contractLocation common.AddressLocation
	functionName     string
	arguments        []cadence.Value
	argumentTypes    []sema.Type
	logger           zerolog.Logger
	logSpanAttrs     []attribute.KeyValue
}

func NewContractFunctionInvoker(
	contractLocation common.AddressLocation,
	functionName string,
	arguments []cadence.Value,
	argumentTypes []sema.Type,
	logger zerolog.Logger) *ContractFunctionInvoker {
	return &ContractFunctionInvoker{
		contractLocation: contractLocation,
		functionName:     functionName,
		arguments:        arguments,
		argumentTypes:    argumentTypes,
		logger:           logger,
		logSpanAttrs: []attribute.KeyValue{
			attribute.String("transaction.ContractFunctionCall", contractLocation.String()+"."+functionName),
		},
	}
}

func (i *ContractFunctionInvoker) Invoke(env Environment) (cadence.Value, error) {

	span := env.StartSpanFromRoot(trace.FVMInvokeContractFunction)
	span.SetAttributes(i.logSpanAttrs...)
	defer span.End()

	predeclaredValues := valueDeclarations(env)

	ctx := env.Context()

	var blockHeight uint64
	if ctx.BlockHeader != nil {
		blockHeight = ctx.BlockHeader.Height
	}

	chainID := ctx.Chain.ChainID()

	value, err := env.VM().Runtime.InvokeContractFunction(
		i.contractLocation,
		i.functionName,
		i.arguments,
		i.argumentTypes,
		runtime.Context{
			Interface:         env,
			PredeclaredValues: predeclaredValues,
			CheckerOptions: []sema.Option{
				sema.WithAllowResourceInvalidationAfterPotentialJump(
					allowResourceInvalidationAfterPotentialJump(chainID, blockHeight),
				),
			},
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
