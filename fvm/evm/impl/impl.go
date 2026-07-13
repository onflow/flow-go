package impl

import (
	"fmt"
	"math/big"

	"github.com/holiman/uint256"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/bbq/vm"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/errors"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/sema"

	"github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	gethCrypto "github.com/ethereum/go-ethereum/crypto"
)

var internalEVMContractStaticType = interpreter.ConvertSemaCompositeTypeToStaticCompositeType(
	nil,
	stdlib.InternalEVMContractType,
)

// NewInternalEVMContractValue creates the interpreter-side value for the InternalEVM contract.
// Its methods are the interpreter forms of the internal EVM functions.
func NewInternalEVMContractValue(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
	contractAddress flow.Address,
) *interpreter.SimpleCompositeValue {
	location := common.NewAddressLocation(nil, common.Address(contractAddress), stdlib.ContractName)

	methods := map[string]interpreter.FunctionValue{}

	computeLazyStoredMethod := func(name string) interpreter.FunctionValue {
		switch name {
		case stdlib.InternalEVMTypeRunFunctionName:
			return newInterpreterInternalEVMTypeRunFunction(gauge, handler)
		case stdlib.InternalEVMTypeBatchRunFunctionName:
			return newInterpreterInternalEVMTypeBatchRunFunction(gauge, handler)
		case stdlib.InternalEVMTypeCreateCadenceOwnedAccountFunctionName:
			return newInterpreterInternalEVMTypeCreateCadenceOwnedAccountFunction(gauge, handler)
		case stdlib.InternalEVMTypeCallFunctionName:
			return newInterpreterInternalEVMTypeCallFunction(gauge, handler)
		case stdlib.InternalEVMTypeCallWithSigAndArgsFunctionName:
			return newInterpreterInternalEVMTypeCallWithSigAndArgsFunction(gauge, handler, location)
		case stdlib.InternalEVMTypeDepositFunctionName:
			return newInterpreterInternalEVMTypeDepositFunction(gauge, handler)
		case stdlib.InternalEVMTypeWithdrawFunctionName:
			return newInterpreterInternalEVMTypeWithdrawFunction(gauge, handler)
		case stdlib.InternalEVMTypeDeployFunctionName:
			return newInterpreterInternalEVMTypeDeployFunction(gauge, handler)
		case stdlib.InternalEVMTypeBalanceFunctionName:
			return newInterpreterInternalEVMTypeBalanceFunction(gauge, handler)
		case stdlib.InternalEVMTypeNonceFunctionName:
			return newInterpreterInternalEVMTypeNonceFunction(gauge, handler)
		case stdlib.InternalEVMTypeCodeFunctionName:
			return newInterpreterInternalEVMTypeCodeFunction(gauge, handler)
		case stdlib.InternalEVMTypeCodeHashFunctionName:
			return newInterpreterInternalEVMTypeCodeHashFunction(gauge, handler)
		case stdlib.InternalEVMTypeEncodeABIFunctionName:
			return newInterpreterInternalEVMTypeEncodeABIFunction(gauge, location)
		case stdlib.InternalEVMTypeDecodeABIFunctionName:
			return newInterpreterInternalEVMTypeDecodeABIFunction(gauge, location)
		case stdlib.InternalEVMTypeCastToAttoFLOWFunctionName:
			return newInterpreterInternalEVMTypeCastToAttoFLOWFunction(gauge)
		case stdlib.InternalEVMTypeCastToFLOWFunctionName:
			return newInterpreterInternalEVMTypeCastToFLOWFunction(gauge)
		case stdlib.InternalEVMTypeGetLatestBlockFunctionName:
			return newInterpreterInternalEVMTypeGetLatestBlockFunction(gauge, handler)
		case stdlib.InternalEVMTypeDryRunFunctionName:
			return newInterpreterInternalEVMTypeDryRunFunction(gauge, handler)
		case stdlib.InternalEVMTypeDryCallFunctionName:
			return newInterpreterInternalEVMTypeDryCallFunction(gauge, handler)
		case stdlib.InternalEVMTypeDryCallWithSigAndArgsFunctionName:
			return newInterpreterInternalEVMTypeDryCallWithSigAndArgsFunction(gauge, handler, location)
		case stdlib.InternalEVMTypeCommitBlockProposalFunctionName:
			return newInterpreterInternalEVMTypeCommitBlockProposalFunction(gauge, handler)
		case stdlib.InternalEVMTypeStoreFunctionName:
			return newInterpreterInternalEVMTypeStoreFunction(gauge, handler)
		case stdlib.InternalEVMTypeLoadFunctionName:
			return newInterpreterInternalEVMTypeLoadFunction(gauge, handler)
		case stdlib.InternalEVMTypeRunTxAsFunctionName:
			return newInterpreterInternalEVMTypeRunTxAsFunction(gauge, handler)
		}

		return nil
	}

	// Given all methods of InternalEVM are essentially "static" functions,
	// we can cache them and avoid recomputing them on every access.

	methodGetter := func(
		name string,
		_ interpreter.MemberAccessibleContext,
		_ interpreter.ReferenceValue,
	) interpreter.FunctionValue {
		method, ok := methods[name]
		if !ok {
			method = computeLazyStoredMethod(name)
			if method != nil {
				methods[name] = method
			}
		}

		return method
	}

	return interpreter.NewSimpleCompositeValue(
		gauge,
		stdlib.InternalEVMContractType.ID(),
		internalEVMContractStaticType,
		stdlib.InternalEVMContractType.Fields,
		nil,
		nil,
		methodGetter,
		nil,
		nil,
	)
}

// NewInternalEVMFunctions creates the VM-side native function values for the InternalEVM contract.
func NewInternalEVMFunctions(
	handler types.ContractHandler,
	contractAddress flow.Address,
) stdlib.InternalEVMFunctions {
	location := common.NewAddressLocation(nil, common.Address(contractAddress), stdlib.ContractName)

	return stdlib.InternalEVMFunctions{
		Run:                       newVMInternalEVMTypeRunFunction(handler),
		DryRun:                    newVMInternalEVMTypeDryRunFunction(handler),
		BatchRun:                  newVMInternalEVMTypeBatchRunFunction(handler),
		CreateCadenceOwnedAccount: newVMInternalEVMTypeCreateCadenceOwnedAccountFunction(handler),
		Call:                      newVMInternalEVMTypeCallFunction(handler),
		CallWithSigAndArgs:        newVMInternalEVMTypeCallWithSigAndArgsFunction(handler, location),
		RunTxAs:                   newVMInternalEVMTypeRunTxAsFunction(handler),
		DryCall:                   newVMInternalEVMTypeDryCallFunction(handler),
		DryCallWithSigAndArgs:     newVMInternalEVMTypeDryCallWithSigAndArgsFunction(handler, location),
		Deposit:                   newVMInternalEVMTypeDepositFunction(handler),
		Withdraw:                  newVMInternalEVMTypeWithdrawFunction(handler),
		Deploy:                    newVMInternalEVMTypeDeployFunction(handler),
		Balance:                   newVMInternalEVMTypeBalanceFunction(handler),
		Nonce:                     newVMInternalEVMTypeNonceFunction(handler),
		Code:                      newVMInternalEVMTypeCodeFunction(handler),
		CodeHash:                  newVMInternalEVMTypeCodeHashFunction(handler),
		EncodeABI:                 newVMInternalEVMTypeEncodeABIFunction(location),
		DecodeABI:                 newVMInternalEVMTypeDecodeABIFunction(location),
		CastToAttoFLOW:            vmInternalEVMTypeCastToAttoFLOWFunction,
		CastToFLOW:                vmInternalEVMTypeCastToFLOWFunction,
		GetLatestBlock:            newVMInternalEVMTypeGetLatestBlockFunction(handler),
		CommitBlockProposal:       newVMInternalEVMTypeCommitBlockProposalFunction(handler),
		Store:                     newVMInternalEVMTypeStoreFunction(handler),
		Load:                      newVMInternalEVMTypeLoadFunction(handler),
	}
}

// getLatestBlock

func newInterpreterInternalEVMTypeGetLatestBlockFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValueFromNativeFunction(
		gauge,
		stdlib.InternalEVMTypeGetLatestBlockFunctionType,
		newInternalEVMTypeGetLatestBlockFunction(handler),
	)
}

func newInternalEVMTypeGetLatestBlockFunction(
	handler types.ContractHandler,
) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		_ []interpreter.Value,
	) interpreter.Value {

		latestBlock := handler.LastExecutedBlock()

		return NewEVMBlockValue(
			handler,
			context,
			context,
			latestBlock,
		)
	}
}

func newVMInternalEVMTypeGetLatestBlockFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeGetLatestBlockFunctionName,
		stdlib.InternalEVMTypeGetLatestBlockFunctionType,
		newInternalEVMTypeGetLatestBlockFunction(handler),
	)
}

func NewEVMBlockValue(
	handler types.ContractHandler,
	gauge common.MemoryGauge,
	context interpreter.MemberAccessibleContext,
	block *types.Block,
) *interpreter.CompositeValue {
	loc := common.NewAddressLocation(gauge, handler.EVMContractAddress(), stdlib.ContractName)
	hash, err := block.Hash()
	if err != nil {
		panic(err)
	}

	return interpreter.NewCompositeValue(
		context,
		loc,
		stdlib.EVMBlockTypeQualifiedIdentifier,
		common.CompositeKindStructure,
		[]interpreter.CompositeField{
			{
				Name:  "height",
				Value: interpreter.UInt64Value(block.Height),
			},
			{
				Name: "hash",
				Value: interpreter.NewStringValue(
					context,
					common.NewStringMemoryUsage(len(hash)),
					func() string {
						return hash.Hex()
					},
				),
			},
			{
				Name: "totalSupply",
				Value: interpreter.NewIntValueFromBigInt(
					context,
					common.NewBigIntMemoryUsage(common.BigIntByteLength(block.TotalSupply)),
					func() *big.Int {
						return block.TotalSupply
					},
				),
			},
			{
				Name:  "timestamp",
				Value: interpreter.UInt64Value(block.Timestamp),
			},
		},
		common.ZeroAddress,
	)
}

func NewEVMAddress(
	context interpreter.MemberAccessibleContext,
	location common.AddressLocation,
	address types.Address,
) *interpreter.CompositeValue {
	return interpreter.NewCompositeValue(
		context,
		location,
		stdlib.EVMAddressTypeQualifiedIdentifier,
		common.CompositeKindStructure,
		[]interpreter.CompositeField{
			{
				Name:  stdlib.EVMAddressTypeBytesFieldName,
				Value: EVMAddressToAddressBytesArrayValue(context, address),
			},
		},
		common.ZeroAddress,
	)
}

func NewEVMBytes(
	context memberAccessibleArrayCreationContext,
	location common.AddressLocation,
	bytes []byte,
) *interpreter.CompositeValue {
	return interpreter.NewCompositeValue(
		context,
		location,
		stdlib.EVMBytesTypeQualifiedIdentifier,
		common.CompositeKindStructure,
		[]interpreter.CompositeField{
			{
				Name:  stdlib.EVMBytesTypeValueFieldName,
				Value: EVMBytesToBytesArrayValue(context, bytes),
			},
		},
		common.ZeroAddress,
	)
}

func NewEVMBytes4(
	context memberAccessibleArrayCreationContext,
	location common.AddressLocation,
	bytes [4]byte,
) *interpreter.CompositeValue {
	return interpreter.NewCompositeValue(
		context,
		location,
		stdlib.EVMBytes4TypeQualifiedIdentifier,
		common.CompositeKindStructure,
		[]interpreter.CompositeField{
			{
				Name:  stdlib.EVMBytesTypeValueFieldName,
				Value: EVMBytes4ToBytesArrayValue(context, bytes),
			},
		},
		common.ZeroAddress,
	)
}

func NewEVMBytes32(
	context memberAccessibleArrayCreationContext,
	location common.AddressLocation,
	bytes [32]byte,
) *interpreter.CompositeValue {
	return interpreter.NewCompositeValue(
		context,
		location,
		stdlib.EVMBytes32TypeQualifiedIdentifier,
		common.CompositeKindStructure,
		[]interpreter.CompositeField{
			{
				Name:  stdlib.EVMBytesTypeValueFieldName,
				Value: EVMBytes32ToBytesArrayValue(context, bytes),
			},
		},
		common.ZeroAddress,
	)
}

func AddressBytesArrayValueToEVMAddress(
	context interpreter.ContainerMutationContext,
	addressBytesValue *interpreter.ArrayValue,
) (
	result types.Address,
	err error,
) {
	// Convert

	var bytes []byte
	bytes, err = interpreter.ByteArrayValueToByteSlice(context, addressBytesValue)
	if err != nil {
		return result, err
	}

	// Check length

	length := len(bytes)
	const expectedLength = types.AddressLength
	if length != expectedLength {
		return result, errors.NewDefaultUserError(
			"invalid address length: got %d, expected %d",
			length,
			expectedLength,
		)
	}

	copy(result[:], bytes)

	return result, nil
}

func EVMAddressToAddressBytesArrayValue(
	context interpreter.ArrayCreationContext,
	address types.Address,
) *interpreter.ArrayValue {
	return interpreter.ByteSliceToByteArrayValueWithType(
		context,
		stdlib.EVMAddressBytesStaticType,
		address[:],
	)
}

func EVMBytesToBytesArrayValue(
	context interpreter.ArrayCreationContext,
	bytes []byte,
) *interpreter.ArrayValue {
	return interpreter.ByteSliceToByteArrayValueWithType(
		context,
		stdlib.EVMBytesValueStaticType,
		bytes,
	)
}

func EVMBytes4ToBytesArrayValue(
	context interpreter.ArrayCreationContext,
	bytes [4]byte,
) *interpreter.ArrayValue {
	return interpreter.ByteSliceToByteArrayValueWithType(
		context,
		stdlib.EVMBytes4ValueStaticType,
		bytes[:],
	)
}

func EVMBytes32ToBytesArrayValue(
	context interpreter.ArrayCreationContext,
	bytes [32]byte,
) *interpreter.ArrayValue {
	return interpreter.ByteSliceToByteArrayValueWithType(
		context,
		stdlib.EVMBytes32ValueStaticType,
		bytes[:],
	)
}

// createCadenceOwnedAccount

func newInterpreterInternalEVMTypeCreateCadenceOwnedAccountFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValueFromNativeFunction(
		gauge,
		stdlib.InternalEVMTypeCreateCadenceOwnedAccountFunctionType,
		newInternalEVMTypeCreateCadenceOwnedAccountFunction(handler),
	)
}

func newInternalEVMTypeCreateCadenceOwnedAccountFunction(
	handler types.ContractHandler,
) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		args []interpreter.Value,
	) interpreter.Value {

		uuid, ok := args[0].(interpreter.UInt64Value)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		address := handler.DeployCOA(uint64(uuid))

		return EVMAddressToAddressBytesArrayValue(context, address)
	}
}

func newVMInternalEVMTypeCreateCadenceOwnedAccountFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeCreateCadenceOwnedAccountFunctionName,
		stdlib.InternalEVMTypeCreateCadenceOwnedAccountFunctionType,
		newInternalEVMTypeCreateCadenceOwnedAccountFunction(handler),
	)
}

// code

// newInterpreterInternalEVMTypeCodeFunction returns the code of the account
func newInterpreterInternalEVMTypeCodeFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValueFromNativeFunction(
		gauge,
		stdlib.InternalEVMTypeCodeFunctionType,
		newInternalEVMTypeCodeFunction(handler),
	)
}

func newInternalEVMTypeCodeFunction(handler types.ContractHandler) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		args []interpreter.Value,
	) interpreter.Value {

		addressValue, ok := args[0].(*interpreter.ArrayValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		address, err := AddressBytesArrayValueToEVMAddress(context, addressValue)
		if err != nil {
			panic(err)
		}

		const isAuthorized = false
		account := handler.AccountByAddress(address, isAuthorized)

		return interpreter.ByteSliceToByteArrayValue(context, account.Code())
	}
}

func newVMInternalEVMTypeCodeFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeCodeFunctionName,
		stdlib.InternalEVMTypeCodeFunctionType,
		newInternalEVMTypeCodeFunction(handler),
	)
}

// nonce

// newInterpreterInternalEVMTypeNonceFunction returns the nonce of the account
func newInterpreterInternalEVMTypeNonceFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValueFromNativeFunction(
		gauge,
		stdlib.InternalEVMTypeNonceFunctionType,
		newInternalEVMTypeNonceFunction(handler),
	)
}

func newInternalEVMTypeNonceFunction(handler types.ContractHandler) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		args []interpreter.Value,
	) interpreter.Value {
		addressValue, ok := args[0].(*interpreter.ArrayValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		address, err := AddressBytesArrayValueToEVMAddress(context, addressValue)
		if err != nil {
			panic(err)
		}

		const isAuthorized = false
		account := handler.AccountByAddress(address, isAuthorized)

		return interpreter.UInt64Value(account.Nonce())
	}
}

func newVMInternalEVMTypeNonceFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeNonceFunctionName,
		stdlib.InternalEVMTypeNonceFunctionType,
		newInternalEVMTypeNonceFunction(handler),
	)
}

// call

func newInterpreterInternalEVMTypeCallFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValueFromNativeFunction(
		gauge,
		stdlib.InternalEVMTypeCallFunctionType,
		newInternalEVMTypeCallFunction(handler),
	)
}

func newInternalEVMTypeCallFunction(handler types.ContractHandler) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		args []interpreter.Value,
	) interpreter.Value {

		callArgs, err := parseCallArguments(context, args)
		if err != nil {
			panic(err)
		}

		// Call

		const isAuthorized = true
		account := handler.AccountByAddress(callArgs.from, isAuthorized)
		result := account.Call(callArgs.to, callArgs.data, callArgs.gasLimit, callArgs.balance)

		return NewResultValue(
			handler,
			context,
			context,
			result,
		)
	}
}

func newVMInternalEVMTypeCallFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeCallFunctionName,
		stdlib.InternalEVMTypeCallFunctionType,
		newInternalEVMTypeCallFunction(handler),
	)
}

// runTxAs

func newInterpreterInternalEVMTypeRunTxAsFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValueFromNativeFunction(
		gauge,
		stdlib.InternalEVMTypeRunTxAsFunctionType,
		newInternalEVMTypeRunTxAsFunction(handler),
	)
}

func newInternalEVMTypeRunTxAsFunction(handler types.ContractHandler) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		args []interpreter.Value,
	) interpreter.Value {

		callArgs, err := parseCallArguments(context, args)
		if err != nil {
			panic(err)
		}

		// Call

		result := handler.RunTxAs(
			callArgs.from,
			callArgs.to,
			callArgs.data,
			callArgs.gasLimit,
			callArgs.balance,
		)

		return NewResultValue(
			handler,
			context,
			context,
			result,
		)
	}
}

func newVMInternalEVMTypeRunTxAsFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeRunTxAsFunctionName,
		stdlib.InternalEVMTypeRunTxAsFunctionType,
		newInternalEVMTypeRunTxAsFunction(handler),
	)
}

// callWithSigAndArgs

func newInterpreterInternalEVMTypeCallWithSigAndArgsFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
	location common.AddressLocation,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValueFromNativeFunction(
		gauge,
		stdlib.InternalEVMTypeCallWithSigAndArgsFunctionType,
		newInternalEVMTypeCallWithSigAndArgsFunction(handler, location),
	)
}

func newInternalEVMTypeCallWithSigAndArgsFunction(
	handler types.ContractHandler,
	location common.AddressLocation,
) interpreter.NativeFunction {
	evmSpecialTypeIDs := NewEVMSpecialTypeIDs(nil, location)

	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		args []interpreter.Value,
	) interpreter.Value {

		// Parse arguments

		callArgs, err := parseCallArgumentsWithSigAndArgs(context, args)
		if err != nil {
			panic(err)
		}

		// Encode signature and arguments

		data, err := encodeABIWithSigAndArgs(context, evmSpecialTypeIDs, callArgs.signature, callArgs.args)
		if err != nil {
			panic(err)
		}

		// Call

		const isAuthorized = true
		account := handler.AccountByAddress(callArgs.from, isAuthorized)
		result := account.Call(callArgs.to, data, callArgs.gasLimit, callArgs.value)

		return NewResultDecodedValue(
			handler,
			context,
			context,
			location,
			evmSpecialTypeIDs,
			callArgs.resultTypes,
			result,
		)
	}
}

func newVMInternalEVMTypeCallWithSigAndArgsFunction(
	handler types.ContractHandler,
	location common.AddressLocation,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeCallWithSigAndArgsFunctionName,
		stdlib.InternalEVMTypeCallWithSigAndArgsFunctionType,
		newInternalEVMTypeCallWithSigAndArgsFunction(handler, location),
	)
}

// dryCall

func newInterpreterInternalEVMTypeDryCallFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValueFromNativeFunction(
		gauge,
		stdlib.InternalEVMTypeDryCallFunctionType,
		newInternalEVMTypeDryCallFunction(handler),
	)
}

func newInternalEVMTypeDryCallFunction(
	handler types.ContractHandler,
) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		args []interpreter.Value,
	) interpreter.Value {

		callArgs, err := parseCallArguments(context, args)
		if err != nil {
			panic(err)
		}
		to := callArgs.to.ToCommon()

		txData := &gethTypes.LegacyTx{
			Nonce:    0,
			To:       &to,
			Gas:      uint64(callArgs.gasLimit),
			Data:     callArgs.data,
			GasPrice: big.NewInt(0),
			Value:    callArgs.balance,
		}

		// call contract function

		res := handler.DryRunWithTxData(txData, callArgs.from)

		return NewResultValue(handler, context, context, res)
	}
}

func newVMInternalEVMTypeDryCallFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeDryCallFunctionName,
		stdlib.InternalEVMTypeDryCallFunctionType,
		newInternalEVMTypeDryCallFunction(handler),
	)
}

// dryCallWithSigAndArgs

func newInterpreterInternalEVMTypeDryCallWithSigAndArgsFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
	location common.AddressLocation,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValueFromNativeFunction(
		gauge,
		stdlib.InternalEVMTypeDryCallWithSigAndArgsFunctionType,
		newInternalEVMTypeDryCallWithSigAndArgsFunction(handler, location),
	)
}

func newInternalEVMTypeDryCallWithSigAndArgsFunction(
	handler types.ContractHandler,
	location common.AddressLocation,
) interpreter.NativeFunction {
	evmSpecialTypeIDs := NewEVMSpecialTypeIDs(nil, location)

	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		args []interpreter.Value,
	) interpreter.Value {

		// Parse arguments

		callArgs, err := parseCallArgumentsWithSigAndArgs(context, args)
		if err != nil {
			panic(err)
		}
		to := callArgs.to.ToCommon()

		data, err := encodeABIWithSigAndArgs(context, evmSpecialTypeIDs, callArgs.signature, callArgs.args)
		if err != nil {
			panic(err)
		}

		txData := &gethTypes.LegacyTx{
			Nonce:    0,
			To:       &to,
			Gas:      uint64(callArgs.gasLimit),
			Data:     data,
			GasPrice: big.NewInt(0),
			Value:    callArgs.value,
		}

		// Call contract function

		res := handler.DryRunWithTxData(txData, callArgs.from)

		return NewResultDecodedValue(
			handler,
			context,
			context,
			location,
			evmSpecialTypeIDs,
			callArgs.resultTypes,
			res,
		)
	}
}

func newVMInternalEVMTypeDryCallWithSigAndArgsFunction(
	handler types.ContractHandler,
	location common.AddressLocation,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeDryCallWithSigAndArgsFunctionName,
		stdlib.InternalEVMTypeDryCallWithSigAndArgsFunctionType,
		newInternalEVMTypeDryCallWithSigAndArgsFunction(handler, location),
	)
}

const fungibleTokenVaultTypeBalanceFieldName = "balance"

// deposit

func newInterpreterInternalEVMTypeDepositFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValueFromNativeFunction(
		gauge,
		stdlib.InternalEVMTypeDepositFunctionType,
		newInternalEVMTypeDepositFunction(handler),
	)
}

func newInternalEVMTypeDepositFunction(handler types.ContractHandler) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		args []interpreter.Value,
	) interpreter.Value {
		// Get from vault

		fromValue, ok := args[0].(*interpreter.CompositeValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		amountValue, ok := fromValue.GetField(
			context,
			fungibleTokenVaultTypeBalanceFieldName,
		).(interpreter.UFix64Value)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		amount := types.NewBalanceFromUFix64(cadence.UFix64(amountValue.UFix64Value))

		// Get to address

		toAddressValue, ok := args[1].(*interpreter.ArrayValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		toAddress, err := AddressBytesArrayValueToEVMAddress(context, toAddressValue)
		if err != nil {
			panic(err)
		}

		// NOTE: We're intentionally not destroying the vault here,
		// because the value of it is supposed to be "kept alive".
		// Destroying would incorrectly be equivalent to a burn and decrease the total supply,
		// and a withdrawal would then have to perform an actual mint of new tokens.

		// Deposit

		const isAuthorized = false
		account := handler.AccountByAddress(toAddress, isAuthorized)
		account.Deposit(types.NewFlowTokenVault(amount))

		return interpreter.Void
	}
}

func newVMInternalEVMTypeDepositFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeDepositFunctionName,
		stdlib.InternalEVMTypeDepositFunctionType,
		newInternalEVMTypeDepositFunction(handler),
	)
}

// balance

// newInterpreterInternalEVMTypeBalanceFunction returns the Flow balance of the account
func newInterpreterInternalEVMTypeBalanceFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValueFromNativeFunction(
		gauge,
		stdlib.InternalEVMTypeBalanceFunctionType,
		newInternalEVMTypeBalanceFunction(handler),
	)
}

func newInternalEVMTypeBalanceFunction(handler types.ContractHandler) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		args []interpreter.Value,
	) interpreter.Value {

		addressValue, ok := args[0].(*interpreter.ArrayValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		address, err := AddressBytesArrayValueToEVMAddress(context, addressValue)
		if err != nil {
			panic(err)
		}

		const isAuthorized = false
		account := handler.AccountByAddress(address, isAuthorized)

		return interpreter.UIntValue{BigInt: account.Balance()}
	}
}

func newVMInternalEVMTypeBalanceFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeBalanceFunctionName,
		stdlib.InternalEVMTypeBalanceFunctionType,
		newInternalEVMTypeBalanceFunction(handler),
	)
}

// codeHash

// newInterpreterInternalEVMTypeCodeHashFunction returns the code hash of the account
func newInterpreterInternalEVMTypeCodeHashFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValueFromNativeFunction(
		gauge,
		stdlib.InternalEVMTypeCodeHashFunctionType,
		newInternalEVMTypeCodeHashFunction(handler),
	)
}

func newInternalEVMTypeCodeHashFunction(handler types.ContractHandler) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		args []interpreter.Value,
	) interpreter.Value {

		addressValue, ok := args[0].(*interpreter.ArrayValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		address, err := AddressBytesArrayValueToEVMAddress(context, addressValue)
		if err != nil {
			panic(err)
		}

		const isAuthorized = false
		account := handler.AccountByAddress(address, isAuthorized)

		return interpreter.ByteSliceToByteArrayValue(context, account.CodeHash())
	}
}

func newVMInternalEVMTypeCodeHashFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeCodeHashFunctionName,
		stdlib.InternalEVMTypeCodeHashFunctionType,
		newInternalEVMTypeCodeHashFunction(handler),
	)
}

// withdraw

func newInterpreterInternalEVMTypeWithdrawFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValueFromNativeFunction(
		gauge,
		stdlib.InternalEVMTypeWithdrawFunctionType,
		newInternalEVMTypeWithdrawFunction(handler),
	)
}

func newInternalEVMTypeWithdrawFunction(handler types.ContractHandler) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		args []interpreter.Value,
	) interpreter.Value {

		// Get from address

		fromAddressValue, ok := args[0].(*interpreter.ArrayValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		fromAddress, err := AddressBytesArrayValueToEVMAddress(context, fromAddressValue)
		if err != nil {
			panic(err)
		}

		// Get amount

		amountValue, ok := args[1].(interpreter.UIntValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		_, overflow := uint256.FromBig(amountValue.BigInt)
		if overflow {
			panic(types.ErrInvalidBalance)
		}

		// check balance is not prone to rounding error
		if !types.AttoFlowBalanceIsValidForFlowVault(amountValue.BigInt) {
			panic(types.ErrWithdrawBalanceRounding)
		}

		// this is where rounding from Atto scale to UFix scale happens.
		value := new(big.Int).Div(amountValue.BigInt, types.UFixToAttoConversionMultiplier)
		amount := types.NewBalanceFromUFix64(cadence.UFix64(value.Uint64()))

		// Withdraw

		const isAuthorized = true
		account := handler.AccountByAddress(fromAddress, isAuthorized)
		vault := account.Withdraw(amount)

		ufix, roundedOff, err := types.ConvertBalanceToUFix64(vault.Balance())
		if err != nil {
			panic(err)
		}
		// We have already truncated the remainder above, but we still leave
		// the rounding check in as a redundancy.
		if roundedOff {
			panic(types.ErrWithdrawBalanceRounding)
		}

		// TODO: improve: maybe call actual constructor
		return interpreter.NewCompositeValue(
			context,
			common.NewAddressLocation(context, handler.FlowTokenAddress(), "FlowToken"),
			"FlowToken.Vault",
			common.CompositeKindResource,
			[]interpreter.CompositeField{
				{
					Name: "balance",
					Value: interpreter.NewUFix64Value(context, func() uint64 {
						return uint64(ufix)
					}),
				},
				{
					Name: sema.ResourceUUIDFieldName,
					Value: interpreter.NewUInt64Value(context, func() uint64 {
						return handler.GenerateResourceUUID()
					}),
				},
			},
			common.ZeroAddress,
		)
	}
}

func newVMInternalEVMTypeWithdrawFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeWithdrawFunctionName,
		stdlib.InternalEVMTypeWithdrawFunctionType,
		newInternalEVMTypeWithdrawFunction(handler),
	)
}

// deploy

func newInterpreterInternalEVMTypeDeployFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValueFromNativeFunction(
		gauge,
		stdlib.InternalEVMTypeDeployFunctionType,
		newInternalEVMTypeDeployFunction(handler),
	)
}

func newInternalEVMTypeDeployFunction(
	handler types.ContractHandler,
) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		args []interpreter.Value,
	) interpreter.Value {

		// Get from address

		fromAddressValue, ok := args[0].(*interpreter.ArrayValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		fromAddress, err := AddressBytesArrayValueToEVMAddress(context, fromAddressValue)
		if err != nil {
			panic(err)
		}

		// Get code

		codeValue, ok := args[1].(*interpreter.ArrayValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		code, err := interpreter.ByteArrayValueToByteSlice(context, codeValue)
		if err != nil {
			panic(err)
		}

		// Get gas limit

		gasLimitValue, ok := args[2].(interpreter.UInt64Value)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		gasLimit := types.GasLimit(gasLimitValue)

		// Get value

		amountValue, ok := args[3].(interpreter.UIntValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		amount := types.NewBalance(amountValue.BigInt)

		// Deploy

		const isAuthorized = true
		account := handler.AccountByAddress(fromAddress, isAuthorized)
		result := account.Deploy(code, gasLimit, amount)

		return NewResultValue(
			handler,
			context,
			context,
			result,
		)
	}
}

func newVMInternalEVMTypeDeployFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeDeployFunctionName,
		stdlib.InternalEVMTypeDeployFunctionType,
		newInternalEVMTypeDeployFunction(handler),
	)
}

// castToAttoFLOW

func newInterpreterInternalEVMTypeCastToAttoFLOWFunction(
	gauge common.MemoryGauge,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValueFromNativeFunction(
		gauge,
		stdlib.InternalEVMTypeCastToAttoFLOWFunctionType,
		internalEVMTypeCastToAttoFLOWFunction,
	)
}

var internalEVMTypeCastToAttoFLOWFunction = interpreter.NativeFunction(
	func(
		_ interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		args []interpreter.Value,
	) interpreter.Value {
		balanceValue, ok := args[0].(interpreter.UFix64Value)
		if !ok {
			panic(errors.NewUnreachableError())
		}
		balance := types.NewBalanceFromUFix64(cadence.UFix64(balanceValue.UFix64Value))
		return interpreter.UIntValue{BigInt: balance}
	},
)

var vmInternalEVMTypeCastToAttoFLOWFunction = vm.NewNativeFunctionValue(
	stdlib.InternalEVMTypeCastToAttoFLOWFunctionName,
	stdlib.InternalEVMTypeCastToAttoFLOWFunctionType,
	internalEVMTypeCastToAttoFLOWFunction,
)

// castToFLOW

func newInterpreterInternalEVMTypeCastToFLOWFunction(
	gauge common.MemoryGauge,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValueFromNativeFunction(
		gauge,
		stdlib.InternalEVMTypeCastToFLOWFunctionType,
		internalEVMTypeCastToFLOWFunction,
	)
}

var internalEVMTypeCastToFLOWFunction = interpreter.NativeFunction(
	func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		args []interpreter.Value,
	) interpreter.Value {

		balanceValue, ok := args[0].(interpreter.UIntValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		balance := types.NewBalance(balanceValue.BigInt)
		// ignoring the rounding error and let user handle it
		v, _, err := types.ConvertBalanceToUFix64(balance)
		if err != nil {
			panic(err)
		}

		return interpreter.NewUFix64Value(context, func() uint64 {
			return uint64(v)
		})
	},
)

var vmInternalEVMTypeCastToFLOWFunction = vm.NewNativeFunctionValue(
	stdlib.InternalEVMTypeCastToFLOWFunctionName,
	stdlib.InternalEVMTypeCastToFLOWFunctionType,
	internalEVMTypeCastToFLOWFunction,
)

// commitBlockProposal

func newInterpreterInternalEVMTypeCommitBlockProposalFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValueFromNativeFunction(
		gauge,
		stdlib.InternalEVMTypeCommitBlockProposalFunctionType,
		newInternalEVMTypeCommitBlockProposalFunction(handler),
	)
}

func newInternalEVMTypeCommitBlockProposalFunction(
	handler types.ContractHandler,
) interpreter.NativeFunction {
	return func(
		_ interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		_ []interpreter.Value,
	) interpreter.Value {
		handler.CommitBlockProposal()
		return interpreter.Void
	}
}

func newVMInternalEVMTypeCommitBlockProposalFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeCommitBlockProposalFunctionName,
		stdlib.InternalEVMTypeCommitBlockProposalFunctionType,
		newInternalEVMTypeCommitBlockProposalFunction(handler),
	)
}

// load

func newInterpreterInternalEVMTypeLoadFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValueFromNativeFunction(
		gauge,
		stdlib.InternalEVMTypeLoadFunctionType,
		newInternalEVMTypeLoadFunction(handler),
	)
}

func newInternalEVMTypeLoadFunction(handler types.ContractHandler) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		args []interpreter.Value,
	) interpreter.Value {

		// Get target argument
		targetValue, ok := args[0].(*interpreter.ArrayValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		addr, err := AddressBytesArrayValueToEVMAddress(context, targetValue)
		if err != nil {
			panic(err)
		}

		// Get slot argument
		slotValue, ok := args[1].(*interpreter.StringValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		if !isHexHash(slotValue.Str) {
			panic(fmt.Errorf("invalid input: slot is not a valid hex-encoded Ethereum hash"))
		}
		slot := gethCommon.HexToHash(slotValue.Str)

		value := handler.GetState(addr, slot)
		return interpreter.ByteSliceToByteArrayValue(context, value.Bytes())
	}
}

func newVMInternalEVMTypeLoadFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeLoadFunctionName,
		stdlib.InternalEVMTypeLoadFunctionType,
		newInternalEVMTypeLoadFunction(handler),
	)
}

// store

func newInterpreterInternalEVMTypeStoreFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValueFromNativeFunction(
		gauge,
		stdlib.InternalEVMTypeStoreFunctionType,
		newInternalEVMTypeStoreFunction(handler),
	)
}

func newInternalEVMTypeStoreFunction(handler types.ContractHandler) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		args []interpreter.Value,
	) interpreter.Value {

		// Get target argument
		targetValue, ok := args[0].(*interpreter.ArrayValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		addr, err := AddressBytesArrayValueToEVMAddress(context, targetValue)
		if err != nil {
			panic(err)
		}

		// Get slot argument
		slotValue, ok := args[1].(*interpreter.StringValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		if !isHexHash(slotValue.Str) {
			panic(fmt.Errorf("invalid input: slot is not a valid hex-encoded Ethereum hash"))
		}
		slot := gethCommon.HexToHash(slotValue.Str)

		// Get value argument
		valueValue, ok := args[2].(*interpreter.StringValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		if !isHexHash(valueValue.Str) {
			panic(fmt.Errorf("invalid input: value is not a valid hex-encoded Ethereum hash"))
		}
		value := gethCommon.HexToHash(valueValue.Str)

		handler.SetState(addr, slot, value)

		return interpreter.Void
	}
}

func newVMInternalEVMTypeStoreFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeStoreFunctionName,
		stdlib.InternalEVMTypeStoreFunctionType,
		newInternalEVMTypeStoreFunction(handler),
	)
}

// run

func newInterpreterInternalEVMTypeRunFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValueFromNativeFunction(
		gauge,
		stdlib.InternalEVMTypeRunFunctionType,
		newInternalEVMTypeRunFunction(handler),
	)
}

func newInternalEVMTypeRunFunction(
	handler types.ContractHandler,
) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		args []interpreter.Value,
	) interpreter.Value {

		// Get transaction argument

		transactionValue, ok := args[0].(*interpreter.ArrayValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		transaction, err := interpreter.ByteArrayValueToByteSlice(context, transactionValue)
		if err != nil {
			panic(err)
		}

		// Get gas fee collector argument

		gasFeeCollectorValue, ok := args[1].(*interpreter.ArrayValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		gasFeeCollector, err := interpreter.ByteArrayValueToByteSlice(context, gasFeeCollectorValue)
		if err != nil {
			panic(err)
		}

		// run transaction
		result := handler.Run(transaction, types.NewAddressFromBytes(gasFeeCollector))

		return NewResultValue(
			handler,
			context,
			context,
			result,
		)
	}
}

func newVMInternalEVMTypeRunFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeRunFunctionName,
		stdlib.InternalEVMTypeRunFunctionType,
		newInternalEVMTypeRunFunction(handler),
	)
}

// dryRun

func newInterpreterInternalEVMTypeDryRunFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValueFromNativeFunction(
		gauge,
		stdlib.InternalEVMTypeDryRunFunctionType,
		newInternalEVMTypeDryRunFunction(handler),
	)
}

func newInternalEVMTypeDryRunFunction(
	handler types.ContractHandler,
) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		args []interpreter.Value,
	) interpreter.Value {

		// Get transaction argument

		transactionValue, ok := args[0].(*interpreter.ArrayValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		transaction, err := interpreter.ByteArrayValueToByteSlice(context, transactionValue)
		if err != nil {
			panic(err)
		}

		// Get from argument

		fromValue, ok := args[1].(*interpreter.ArrayValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		from, err := interpreter.ByteArrayValueToByteSlice(context, fromValue)
		if err != nil {
			panic(err)
		}

		// call estimate

		res := handler.DryRun(transaction, types.NewAddressFromBytes(from))
		return NewResultValue(
			handler,
			context,
			context,
			res,
		)
	}
}

func newVMInternalEVMTypeDryRunFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeDryRunFunctionName,
		stdlib.InternalEVMTypeDryRunFunctionType,
		newInternalEVMTypeDryRunFunction(handler),
	)
}

// batchRun

func newInterpreterInternalEVMTypeBatchRunFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValueFromNativeFunction(
		gauge,
		stdlib.InternalEVMTypeBatchRunFunctionType,
		newInternalEVMTypeBatchRunFunction(handler),
	)
}

func newInternalEVMTypeBatchRunFunction(
	handler types.ContractHandler,
) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.ArgumentTypesIterator,
		_ interpreter.Value,
		args []interpreter.Value,
	) interpreter.Value {

		// Get transactions batch argument
		transactionsBatchValue, ok := args[0].(*interpreter.ArrayValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		batchCount := transactionsBatchValue.Count()
		var transactionBatch [][]byte
		if batchCount > 0 {
			transactionBatch = make([][]byte, batchCount)
			i := 0
			transactionsBatchValue.Iterate(
				context,
				func(transactionValue interpreter.Value) (resume bool) {
					t, err := interpreter.ByteArrayValueToByteSlice(context, transactionValue)
					if err != nil {
						panic(err)
					}
					transactionBatch[i] = t
					i++
					return true
				},
				false,
			)
		}

		// Get gas fee collector argument
		gasFeeCollectorValue, ok := args[1].(*interpreter.ArrayValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		gasFeeCollector, err := interpreter.ByteArrayValueToByteSlice(context, gasFeeCollectorValue)
		if err != nil {
			panic(err)
		}

		// Batch run
		batchResults := handler.BatchRun(transactionBatch, types.NewAddressFromBytes(gasFeeCollector))

		values := newResultValues(handler, context, context, batchResults)

		loc := common.NewAddressLocation(context, handler.EVMContractAddress(), stdlib.ContractName)
		evmResultType := interpreter.NewVariableSizedStaticType(
			context,
			interpreter.NewCompositeStaticType(
				nil,
				loc,
				stdlib.EVMResultTypeQualifiedIdentifier,
				common.NewTypeIDFromQualifiedName(
					nil,
					loc,
					stdlib.EVMResultTypeQualifiedIdentifier,
				),
			),
		)

		return interpreter.NewArrayValue(
			context,
			evmResultType,
			common.ZeroAddress,
			values...,
		)
	}
}

func newVMInternalEVMTypeBatchRunFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeBatchRunFunctionName,
		stdlib.InternalEVMTypeBatchRunFunctionType,
		newInternalEVMTypeBatchRunFunction(handler),
	)
}

// newResultValues converts batch run result summary type to cadence array of structs
func newResultValues(
	handler types.ContractHandler,
	gauge common.MemoryGauge,
	context interpreter.MemberAccessibleContext,
	results []*types.ResultSummary,
) []interpreter.Value {
	var values []interpreter.Value
	if len(results) > 0 {
		values = make([]interpreter.Value, 0, len(results))
		for _, result := range results {
			res := NewResultValue(
				handler,
				gauge,
				context,
				result,
			)
			values = append(values, res)
		}
	}
	return values
}

func NewResultValue(
	handler types.ContractHandler,
	gauge common.MemoryGauge,
	context interpreter.MemberAccessibleContext,
	result *types.ResultSummary,
) *interpreter.CompositeValue {

	evmContractLocation := common.NewAddressLocation(
		gauge,
		handler.EVMContractAddress(),
		stdlib.ContractName,
	)

	deployedContractAddress := result.DeployedContractAddress
	deployedContractValue := interpreter.NilOptionalValue
	if deployedContractAddress != nil {
		deployedContractValue = interpreter.NewSomeValueNonCopying(
			context,
			NewEVMAddress(
				context,
				evmContractLocation,
				*deployedContractAddress,
			),
		)
	}

	fields := []interpreter.CompositeField{
		{
			Name: "status",
			Value: interpreter.NewEnumCaseValue(
				context,
				&sema.CompositeType{
					Location:   evmContractLocation,
					Identifier: stdlib.EVMStatusTypeQualifiedIdentifier,
					Kind:       common.CompositeKindEnum,
				},
				interpreter.NewUInt8Value(gauge, func() uint8 {
					return uint8(result.Status)
				}),
				nil,
			),
		},
		{
			Name: "errorCode",
			Value: interpreter.NewUInt64Value(gauge, func() uint64 {
				return uint64(result.ErrorCode)
			}),
		},
		{
			Name: "errorMessage",
			Value: interpreter.NewStringValue(
				context,
				common.NewStringMemoryUsage(len(result.ErrorMessage)),
				func() string {
					return result.ErrorMessage
				},
			),
		},
		{
			Name: "gasUsed",
			Value: interpreter.NewUInt64Value(gauge, func() uint64 {
				return result.GasConsumed
			}),
		},
		{
			Name:  "data",
			Value: interpreter.ByteSliceToByteArrayValue(context, result.ReturnedData),
		},
		{
			Name:  "deployedContract",
			Value: deployedContractValue,
		},
	}

	return interpreter.NewCompositeValue(
		context,
		evmContractLocation,
		stdlib.EVMResultTypeQualifiedIdentifier,
		common.CompositeKindStructure,
		fields,
		common.ZeroAddress,
	)
}

func NewResultDecodedValue(
	handler types.ContractHandler,
	gauge common.MemoryGauge,
	context interpreter.InvocationContext,
	location common.AddressLocation,
	evmSpecialTypeIDs *evmSpecialTypeIDs,
	resultTypes *interpreter.ArrayValue,
	result *types.ResultSummary,
) *interpreter.CompositeValue {

	evmContractLocation := common.NewAddressLocation(
		gauge,
		handler.EVMContractAddress(),
		stdlib.ContractName,
	)

	deployedContractAddress := result.DeployedContractAddress
	deployedContractValue := interpreter.NilOptionalValue
	if deployedContractAddress != nil {
		deployedContractValue = interpreter.NewSomeValueNonCopying(
			context,
			NewEVMAddress(
				context,
				evmContractLocation,
				*deployedContractAddress,
			),
		)
	}

	results, err := decodeResultData(context, location, evmSpecialTypeIDs, resultTypes, result)
	if err != nil {
		panic(err)
	}

	fields := []interpreter.CompositeField{
		{
			Name: "status",
			Value: interpreter.NewEnumCaseValue(
				context,
				&sema.CompositeType{
					Location:   evmContractLocation,
					Identifier: stdlib.EVMStatusTypeQualifiedIdentifier,
					Kind:       common.CompositeKindEnum,
				},
				interpreter.NewUInt8Value(gauge, func() uint8 {
					return uint8(result.Status)
				}),
				nil,
			),
		},
		{
			Name: "errorCode",
			Value: interpreter.NewUInt64Value(gauge, func() uint64 {
				return uint64(result.ErrorCode)
			}),
		},
		{
			Name: "errorMessage",
			Value: interpreter.NewStringValue(
				context,
				common.NewStringMemoryUsage(len(result.ErrorMessage)),
				func() string {
					return result.ErrorMessage
				},
			),
		},
		{
			Name: "gasUsed",
			Value: interpreter.NewUInt64Value(gauge, func() uint64 {
				return result.GasConsumed
			}),
		},
		{
			Name:  "results",
			Value: results,
		},
		{
			Name:  "deployedContract",
			Value: deployedContractValue,
		},
	}

	return interpreter.NewCompositeValue(
		context,
		evmContractLocation,
		stdlib.EVMResultDecodedTypeQualifiedIdentifier,
		common.CompositeKindStructure,
		fields,
		common.ZeroAddress,
	)
}

func decodeResultData(
	context interpreter.InvocationContext,
	location common.AddressLocation,
	evmSpecialTypeIDs *evmSpecialTypeIDs,
	resultTypes *interpreter.ArrayValue,
	result *types.ResultSummary,
) (interpreter.Value, error) {

	if result.Status != types.StatusSuccessful || resultTypes == nil || resultTypes.Count() == 0 {
		resultValue := interpreter.ByteSliceToByteArrayValue(context, result.ReturnedData)
		return resultValue, nil
	}

	resultValue := decodeABIs(context, location, evmSpecialTypeIDs, resultTypes, result.ReturnedData)
	return resultValue, nil
}

func ResultSummaryFromEVMResultValue(val cadence.Value) (*types.ResultSummary, error) {
	str, ok := val.(cadence.Struct)
	if !ok {
		return nil, fmt.Errorf("invalid input: unexpected value type")
	}

	fields := cadence.FieldsMappedByName(str)

	const expectedFieldCount = 6
	if len(fields) != expectedFieldCount {
		return nil, fmt.Errorf(
			"invalid input: field count mismatch: expected %d, got %d",
			expectedFieldCount,
			len(fields),
		)
	}

	statusEnum, ok := fields[stdlib.EVMResultTypeStatusFieldName].(cadence.Enum)
	if !ok {
		return nil, fmt.Errorf("invalid input: unexpected type for status field")
	}

	status, ok := cadence.FieldsMappedByName(statusEnum)[sema.EnumRawValueFieldName].(cadence.UInt8)
	if !ok {
		return nil, fmt.Errorf("invalid input: unexpected type for status field")
	}

	errorCode, ok := fields[stdlib.EVMResultTypeErrorCodeFieldName].(cadence.UInt64)
	if !ok {
		return nil, fmt.Errorf("invalid input: unexpected type for error code field")
	}

	errorMsg, ok := fields[stdlib.EVMResultTypeErrorMessageFieldName].(cadence.String)
	if !ok {
		return nil, fmt.Errorf("invalid input: unexpected type for error msg field")
	}

	gasUsed, ok := fields[stdlib.EVMResultTypeGasUsedFieldName].(cadence.UInt64)
	if !ok {
		return nil, fmt.Errorf("invalid input: unexpected type for gas field")
	}

	data, ok := fields[stdlib.EVMResultTypeDataFieldName].(cadence.Array)
	if !ok {
		return nil, fmt.Errorf("invalid input: unexpected type for data field")
	}

	convertedData := make([]byte, len(data.Values))
	for i, value := range data.Values {
		convertedData[i] = byte(value.(cadence.UInt8))
	}

	var convertedDeployedAddress *types.Address

	deployedAddressField, ok := fields[stdlib.EVMResultTypeDeployedContractFieldName].(cadence.Optional)
	if !ok {
		return nil, fmt.Errorf("invalid input: unexpected type for deployed contract field")
	}

	if deployedAddressField.Value != nil {
		evmAddress, ok := deployedAddressField.Value.(cadence.Struct)
		if !ok {
			return nil, fmt.Errorf("invalid input: unexpected type for deployed contract field")
		}

		bytes, ok := cadence.SearchFieldByName(evmAddress, stdlib.EVMAddressTypeBytesFieldName).(cadence.Array)
		if !ok {
			return nil, fmt.Errorf("invalid input: unexpected type for deployed contract field")
		}

		convertedAddress := make([]byte, len(bytes.Values))
		for i, value := range bytes.Values {
			convertedAddress[i] = byte(value.(cadence.UInt8))
		}
		addr := types.Address(convertedAddress)
		convertedDeployedAddress = &addr
	}

	return &types.ResultSummary{
		Status:                  types.Status(status),
		ErrorCode:               types.ErrorCode(errorCode),
		ErrorMessage:            string(errorMsg),
		GasConsumed:             uint64(gasUsed),
		MaxGasConsumed:          uint64(gasUsed),
		ReturnedData:            convertedData,
		DeployedContractAddress: convertedDeployedAddress,
	}, nil

}

type callArguments struct {
	from     types.Address
	to       types.Address
	data     []byte
	gasLimit types.GasLimit
	balance  types.Balance
}

func parseCallArguments(
	context interpreter.InvocationContext,
	args []interpreter.Value,
) (
	*callArguments,
	error,
) {
	// Get from address

	fromAddressValue, ok := args[0].(*interpreter.ArrayValue)
	if !ok {
		return nil, errors.NewUnreachableError()
	}

	fromAddress, err := AddressBytesArrayValueToEVMAddress(context, fromAddressValue)
	if err != nil {
		return nil, err
	}

	// Get to address

	toAddressValue, ok := args[1].(*interpreter.ArrayValue)
	if !ok {
		return nil, errors.NewUnreachableError()
	}

	toAddress, err := AddressBytesArrayValueToEVMAddress(context, toAddressValue)
	if err != nil {
		return nil, err
	}

	// Get data

	dataValue, ok := args[2].(*interpreter.ArrayValue)
	if !ok {
		return nil, errors.NewUnreachableError()
	}

	data, err := interpreter.ByteArrayValueToByteSlice(context, dataValue)
	if err != nil {
		return nil, err
	}

	// Get gas limit

	gasLimitValue, ok := args[3].(interpreter.UInt64Value)
	if !ok {
		return nil, errors.NewUnreachableError()
	}

	gasLimit := types.GasLimit(gasLimitValue)

	// Get balance

	balanceValue, ok := args[4].(interpreter.UIntValue)
	if !ok {
		return nil, errors.NewUnreachableError()
	}

	balance := types.NewBalance(balanceValue.BigInt)

	return &callArguments{
		from:     fromAddress,
		to:       toAddress,
		data:     data,
		gasLimit: gasLimit,
		balance:  balance,
	}, nil
}

type callArgumentsWithSigAndArgs struct {
	from        types.Address
	to          types.Address
	signature   string
	args        *interpreter.ArrayValue
	gasLimit    types.GasLimit
	value       types.Balance
	resultTypes *interpreter.ArrayValue
}

func parseCallArgumentsWithSigAndArgs(
	context interpreter.InvocationContext,
	args []interpreter.Value,
) (*callArgumentsWithSigAndArgs, error) {
	// Get from address

	fromAddressValue, ok := args[0].(*interpreter.ArrayValue)
	if !ok {
		return nil, errors.NewUnreachableError()
	}

	fromAddress, err := AddressBytesArrayValueToEVMAddress(context, fromAddressValue)
	if err != nil {
		return nil, err
	}

	// Get to address

	toAddressValue, ok := args[1].(*interpreter.ArrayValue)
	if !ok {
		return nil, errors.NewUnreachableError()
	}

	toAddress, err := AddressBytesArrayValueToEVMAddress(context, toAddressValue)
	if err != nil {
		return nil, err
	}

	// Get signature

	signature, ok := args[2].(*interpreter.StringValue)
	if !ok {
		return nil, errors.NewUnreachableError()
	}

	// Get arguments

	argsValue, ok := args[3].(*interpreter.ArrayValue)
	if !ok {
		return nil, errors.NewUnreachableError()
	}

	// Get gas limit

	gasLimitValue, ok := args[4].(interpreter.UInt64Value)
	if !ok {
		return nil, errors.NewUnreachableError()
	}

	gasLimit := types.GasLimit(gasLimitValue)

	// Get value

	valueValue, ok := args[5].(interpreter.UIntValue)
	if !ok {
		return nil, errors.NewUnreachableError()
	}

	value := types.NewBalance(valueValue.BigInt)

	// Get resultTypes

	var resultTypes *interpreter.ArrayValue
	switch resultTypesField := args[6].(type) {
	case *interpreter.SomeValue:
		if innerTypes := resultTypesField.InnerValue(); innerTypes != nil {
			resultTypes, ok = innerTypes.(*interpreter.ArrayValue)
			if !ok {
				return nil, errors.NewUnreachableError()
			}
		} else {
			return nil, errors.NewUnreachableError()
		}
	case interpreter.NilValue:
	default:
		return nil, errors.NewUnreachableError()
	}

	return &callArgumentsWithSigAndArgs{
		from:        fromAddress,
		to:          toAddress,
		signature:   signature.Str,
		args:        argsValue,
		gasLimit:    gasLimit,
		value:       value,
		resultTypes: resultTypes,
	}, nil
}

func encodeABIWithSigAndArgs(
	context interpreter.InvocationContext,
	evmSpecialTypeIDs *evmSpecialTypeIDs,
	signature string,
	args *interpreter.ArrayValue,
) ([]byte, error) {
	sig := gethCrypto.Keccak256([]byte(signature))

	if len(sig) < 4 {
		return nil, errors.NewUnreachableError()
	}

	sig = sig[:4]

	if args.Count() == 0 {
		return sig, nil
	}

	encodedArguments := encodeABIs(context, evmSpecialTypeIDs, args)

	data := make([]byte, len(sig)+len(encodedArguments))
	n := copy(data, sig)
	copy(data[n:], encodedArguments)

	return data, nil
}

// isHexHash verifies whether a string can represent a valid hex-encoded
// Ethereum hash or not.
func isHexHash(s string) bool {
	if has0xPrefix(s) {
		s = s[2:]
	}
	return len(s) == 2*gethCommon.HashLength && isHex(s)
}

// has0xPrefix validates str begins with '0x' or '0X'.
func has0xPrefix(str string) bool {
	return len(str) >= 2 && str[0] == '0' && (str[1] == 'x' || str[1] == 'X')
}

// isHexCharacter returns bool of c being a valid hexadecimal.
func isHexCharacter(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}

// isHex validates whether each byte is valid hexadecimal string.
func isHex(str string) bool {
	if len(str)%2 != 0 {
		return false
	}
	for _, c := range []byte(str) {
		if !isHexCharacter(c) {
			return false
		}
	}
	return true
}
