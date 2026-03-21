package runtime

import (
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
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

// randomSourceFunctionType is the type of the `randomSource` function.
// This defines the signature as `func(): [UInt8]`
var randomSourceFunctionType = &sema.FunctionType{
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.ByteArrayType),
}

// BlockRandomSourceDeclaration returns a declaration for the `randomSource` function.
// if the environment is a SwappableEnvironment the underlying environment can be swapped without causing issues.
func BlockRandomSourceDeclaration(fvmEnv environment.Environment) stdlib.StandardLibraryValue {
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
				source, err := fvmEnv.RandomSourceHistory()

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

// TransactionIndexDeclaration returns a declaration for the `getTransactionIndex` function.
// if the environment is a SwappableEnvironment the underlying environment can be swapped without causing issues.
func TransactionIndexDeclaration(fvmEnv environment.Environment) stdlib.StandardLibraryValue {
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
						return fvmEnv.TxIndex()
					})
			},
		),
	}
}

// EVMInternalEVMContractValue creates an internal EVM contract value based on the specified ChainID and environment.
// if the environment is a SwappableEnvironment the underlying environment can be swapped without causing issues.
func EVMInternalEVMContractValue(chainID flow.ChainID, fvmEnv environment.Environment) *interpreter.SimpleCompositeValue {
	if fvmEnv == nil {
		return nil
	}
	sc := systemcontracts.SystemContractsForChain(chainID)
	randomBeaconAddress := sc.RandomBeaconHistory.Address
	flowTokenAddress := sc.FlowToken.Address

	evmBackend := backends.NewWrappedEnvironment(fvmEnv)
	evmEmulator := emulator.NewEmulator(evmBackend, evm.StorageAccountAddress(chainID))
	blockStore := handler.NewBlockStore(chainID, evmBackend, evm.StorageAccountAddress(chainID))
	addressAllocator := handler.NewAddressAllocator()

	evmContractAddress := evm.ContractAccountAddress(chainID)

	contractHandler := handler.NewContractHandler(
		chainID,
		evmContractAddress,
		common.Address(flowTokenAddress),
		randomBeaconAddress,
		blockStore,
		addressAllocator,
		evmBackend,
		evmEmulator,
	)

	return impl.NewInternalEVMContractValue(
		nil,
		contractHandler,
		evmContractAddress,
	)
}
