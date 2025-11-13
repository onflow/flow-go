package environment

import (
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/sema"
	"github.com/onflow/cadence/stdlib"

	"github.com/onflow/flow-go/fvm/errors"
)

// randomSourceFunctionType is the type of the `randomSource` function.
// This defines the signature as `func(): [UInt8]`
var randomSourceFunctionType = &sema.FunctionType{
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.ByteArrayType),
}

func declareRandomSourceHistory(txRuntimeEnv runtime.Environment, provider RandomSourceHistoryProvider) {

	const functionName = "randomSourceHistory"

	// Declare the `randomSourceHistory` function. This function is **only** used by the
	// System transaction, to fill the `RandomBeaconHistory` contract via the heartbeat
	// resource. This allows the `RandomBeaconHistory` contract to be a standard contract,
	// without any special parts.
	// Since the `randomSourceHistory` function is only used by the System transaction,
	// it is not part of the cadence standard library, and can just be injected from here.
	// It also doesnt need user documentation, since it is not (and should not)
	// be called by the user. If it is called by the user it will panic.
	functionType := randomSourceFunctionType

	blockRandomSource := stdlib.StandardLibraryValue{
		Name: functionName,
		Type: functionType,
		Kind: common.DeclarationKindFunction,
		Value: interpreter.NewUnmeteredStaticHostFunctionValue(
			functionType,
			func(invocation interpreter.Invocation) interpreter.Value {

				actualArgumentCount := len(invocation.Arguments)
				expectedArgumentCount := len(functionType.Parameters)

				if actualArgumentCount != expectedArgumentCount {
					panic(errors.NewInvalidArgumentErrorf(
						"incorrect number of arguments: got %d, expected %d",
						actualArgumentCount,
						expectedArgumentCount,
					))
				}

				var (
					err    error
					source []byte
				)
				if provider != nil {
					source, err = provider.RandomSourceHistory()
				} else {
					err = errors.NewOperationNotSupportedError(functionName)
				}

				if err != nil {
					panic(err)
				}

				return interpreter.ByteSliceToByteArrayValue(
					invocation.InvocationContext,
					source)
			},
		),
	}

	txRuntimeEnv.DeclareValue(blockRandomSource, nil)
}
