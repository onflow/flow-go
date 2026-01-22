package runtime

import (
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/sema"
	"github.com/onflow/cadence/stdlib"

	"github.com/onflow/flow-go/fvm/errors"
)

// randomSourceFunctionType is the type of the `randomSource` function.
// This defines the signature as `func(): [UInt8]`
var randomSourceFunctionType = &sema.FunctionType{
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.ByteArrayType),
}

func blockRandomSourceDeclaration(renv *ReusableCadenceRuntime) stdlib.StandardLibraryValue {
	// Declare the `randomSourceHistory` function. This function is **only** used by the
	// System transaction, to fill the `RandomBeaconHistory` contract via the heartbeat
	// resource. This allows the `RandomBeaconHistory` contract to be a standard contract,
	// without any special parts.
	// Since the `randomSourceHistory` function is only used by the System transaction,
	// it is not part of the cadence standard library, and can just be injected from here.
	// It also doesn't need user documentation, since it is not (and should not)
	// be called by the user. If it is called by the user it will panic.
	return stdlib.StandardLibraryValue{
		Name: "randomSourceHistory",
		Type: randomSourceFunctionType,
		Kind: common.DeclarationKindFunction,
		Value: interpreter.NewUnmeteredStaticHostFunctionValue(
			randomSourceFunctionType,
			func(invocation interpreter.Invocation) interpreter.Value {
				var err error
				var source []byte
				env := renv.fvmEnv
				if env != nil {
					source, err = env.RandomSourceHistory()
				} else {
					err = errors.NewOperationNotSupportedError("randomSourceHistory")
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
}

// transactionIndexFunctionType is the type of the `getTransactionIndex` function.
// This defines the signature as `func(): UInt32`
var transactionIndexFunctionType = &sema.FunctionType{
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.UInt32Type),
}

func transactionIndexDeclaration(renv *ReusableCadenceRuntime) stdlib.StandardLibraryValue {
	return stdlib.StandardLibraryValue{
		Name:      "getTransactionIndex",
		DocString: `Returns the transaction index in the current block, i.e. first transaction in a block has index of 0, second has index of 1...`,
		Type:      transactionIndexFunctionType,
		Kind:      common.DeclarationKindFunction,
		Value: interpreter.NewUnmeteredStaticHostFunctionValue(
			transactionIndexFunctionType,
			func(invocation interpreter.Invocation) interpreter.Value {
				return interpreter.NewUInt32Value(
					invocation.InvocationContext,
					func() uint32 {
						env := renv.fvmEnv
						if env == nil {
							panic(errors.NewOperationNotSupportedError("transactionIndex"))
						}
						return env.TxIndex()
					})
			},
		),
	}
}
