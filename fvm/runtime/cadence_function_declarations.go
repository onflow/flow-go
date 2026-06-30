package runtime

import (
	"github.com/onflow/cadence/bbq/vm"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/sema"
	"github.com/onflow/cadence/stdlib"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/evm/backends"
	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/impl"
	evmstdlib "github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

// randomSourceHistoryFunctionName is the name of the `randomSourceHistory` function.
const randomSourceHistoryFunctionName = "randomSourceHistory"

// getTransactionIndexFunctionName is the name of the `getTransactionIndex` function.
const getTransactionIndexFunctionName = "getTransactionIndex"

// randomSourceFunctionType is the type of the `randomSourceHistory` function.
// This defines the signature as `func(): [UInt8]`
var randomSourceFunctionType = &sema.FunctionType{
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.ByteArrayType),
}

// transactionIndexFunctionType is the type of the `getTransactionIndex` function.
// This defines the signature as `func(): UInt32`
var transactionIndexFunctionType = &sema.FunctionType{
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.UInt32Type),
}

// randomSourceHistoryNativeFunction returns the native function backing the
// `randomSourceHistory` declaration. It is usable in both the interpreter and the VM.
//
// The `randomSourceHistory` function is **only** used by the System transaction,
// to fill the `RandomBeaconHistory` contract via the heartbeat resource.
// This allows the `RandomBeaconHistory` contract to be a standard contract,
// without any special parts.
// Since the `randomSourceHistory` function is only used by the System transaction,
// it is not part of the cadence standard library, and can just be injected from here.
// It also doesn't need user documentation, since it is not (and should not)
// be called by the user. If it is called by the user it will panic.
func randomSourceHistoryNativeFunction(fvmEnv environment.Environment) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		_ []interpreter.Value,
	) interpreter.Value {
		source, err := fvmEnv.RandomSourceHistory()
		if err != nil {
			panic(err)
		}

		return interpreter.ByteSliceToByteArrayValue(context, source)
	}
}

// InterpreterBlockRandomSourceDeclaration returns the interpreter declaration for the
// `randomSourceHistory` function.
// If the environment is a SwappableEnvironment the underlying environment can be swapped without
// causing issues.
func InterpreterBlockRandomSourceDeclaration(fvmEnv environment.Environment) stdlib.StandardLibraryValue {
	return stdlib.StandardLibraryValue{
		Name: randomSourceHistoryFunctionName,
		Type: randomSourceFunctionType,
		Kind: common.DeclarationKindFunction,
		Value: interpreter.NewUnmeteredStaticHostFunctionValueFromNativeFunction(
			randomSourceFunctionType,
			randomSourceHistoryNativeFunction(fvmEnv),
		),
	}
}

// VMBlockRandomSourceDeclaration returns the VM declaration for the `randomSourceHistory` function.
// If the environment is a SwappableEnvironment the underlying environment can be swapped without
// causing issues.
func VMBlockRandomSourceDeclaration(fvmEnv environment.Environment) stdlib.StandardLibraryValue {
	return stdlib.StandardLibraryValue{
		Name: randomSourceHistoryFunctionName,
		Type: randomSourceFunctionType,
		Kind: common.DeclarationKindFunction,
		Value: vm.NewNativeFunctionValue(
			randomSourceHistoryFunctionName,
			randomSourceFunctionType,
			randomSourceHistoryNativeFunction(fvmEnv),
		),
	}
}

// transactionIndexNativeFunction returns the native function backing the `getTransactionIndex`
// declaration. It is usable in both the interpreter and the VM.
func transactionIndexNativeFunction(fvmEnv environment.Environment) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		_ []interpreter.Value,
	) interpreter.Value {
		return interpreter.NewUInt32Value(
			context,
			func() uint32 {
				return fvmEnv.TxIndex()
			},
		)
	}
}

// transactionIndexDocString documents the `getTransactionIndex` function.
const transactionIndexDocString = `Returns the transaction index in the current block, i.e. first transaction in a block has index of 0, second has index of 1...`

// InterpreterTransactionIndexDeclaration returns the interpreter declaration for the
// `getTransactionIndex` function.
// If the environment is a SwappableEnvironment the underlying environment can be swapped without
// causing issues.
func InterpreterTransactionIndexDeclaration(fvmEnv environment.Environment) stdlib.StandardLibraryValue {
	return stdlib.StandardLibraryValue{
		Name:      getTransactionIndexFunctionName,
		DocString: transactionIndexDocString,
		Type:      transactionIndexFunctionType,
		Kind:      common.DeclarationKindFunction,
		Value: interpreter.NewUnmeteredStaticHostFunctionValueFromNativeFunction(
			transactionIndexFunctionType,
			transactionIndexNativeFunction(fvmEnv),
		),
	}
}

// VMTransactionIndexDeclaration returns the VM declaration for the `getTransactionIndex` function.
// If the environment is a SwappableEnvironment the underlying environment can be swapped without
// causing issues.
func VMTransactionIndexDeclaration(fvmEnv environment.Environment) stdlib.StandardLibraryValue {
	return stdlib.StandardLibraryValue{
		Name:      getTransactionIndexFunctionName,
		DocString: transactionIndexDocString,
		Type:      transactionIndexFunctionType,
		Kind:      common.DeclarationKindFunction,
		Value: vm.NewNativeFunctionValue(
			getTransactionIndexFunctionName,
			transactionIndexFunctionType,
			transactionIndexNativeFunction(fvmEnv),
		),
	}
}

// EVMInternalEVMContract creates the internal EVM contract value (used by the interpreter) and the
// internal EVM functions (used by the VM) for the specified ChainID and environment. Both share a
// single underlying contract handler so that EVM state and caches are consistent regardless of
// whether a procedure executes via the interpreter or the VM.
//
// If the environment is a SwappableEnvironment the underlying environment can be swapped without
// causing issues.
//
// Returns a nil contract value and a zero-valued functions struct if fvmEnv is nil.
func EVMInternalEVMContract(
	chainID flow.ChainID,
	fvmEnv environment.Environment,
) (*interpreter.SimpleCompositeValue, evmstdlib.InternalEVMFunctions) {
	if fvmEnv == nil {
		return nil, evmstdlib.InternalEVMFunctions{}
	}
	sc := systemcontracts.SystemContractsForChain(chainID)
	randomBeaconAddress := sc.RandomBeaconHistory.Address
	flowTokenAddress := sc.FlowToken.Address

	evmBackend := backends.NewWrappedEnvironment(fvmEnv)
	evmEmulator := emulator.NewEmulator(evmBackend, evm.StorageAccountAddress(chainID))
	addressAllocator := handler.NewAddressAllocator()

	evmContractAddress := evm.ContractAccountAddress(chainID)

	contractHandler := handler.NewContractHandler(
		chainID,
		evmContractAddress,
		common.Address(flowTokenAddress),
		randomBeaconAddress,
		addressAllocator,
		evmBackend,
		evmEmulator,
	)

	// Register cache cleanup callback on the SwappableEnvironment.
	// This ensures the cache is cleared whenever the runtime is
	// borrowed for a new transaction or returned to the pool.
	if se, ok := fvmEnv.(*SwappableEnvironment); ok {
		se.RegisterOnSwapCallback(contractHandler.ResetCaches)
	}

	internalEVMValue := impl.NewInternalEVMContractValue(
		nil,
		contractHandler,
		evmContractAddress,
	)

	internalEVMFunctions := impl.NewInternalEVMFunctions(
		contractHandler,
		evmContractAddress,
	)

	return internalEVMValue, internalEVMFunctions
}
