package impl

import (
	"fmt"
	"math/big"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/bbq/vm"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/errors"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/sema"

	"github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"

	gethTypes "github.com/ethereum/go-ethereum/core/types"
)

var internalEVMContractStaticType = interpreter.ConvertSemaCompositeTypeToStaticCompositeType(
	nil,
	stdlib.InternalEVMContractType,
)

func NewInternalEVMContractValue(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
	contractAddress flow.Address,
) *interpreter.SimpleCompositeValue {
	location := common.NewAddressLocation(gauge, common.Address(contractAddress), stdlib.ContractName)

	return interpreter.NewSimpleCompositeValue(
		gauge,
		stdlib.InternalEVMContractType.ID(),
		internalEVMContractStaticType,
		stdlib.InternalEVMContractType.Fields,
		map[string]interpreter.Value{
			stdlib.InternalEVMTypeRunFunctionName:                       newInterpreterInternalEVMTypeRunFunction(gauge, handler),
			stdlib.InternalEVMTypeBatchRunFunctionName:                  newInterpreterInternalEVMTypeBatchRunFunction(gauge, handler),
			stdlib.InternalEVMTypeCreateCadenceOwnedAccountFunctionName: newInterpreterInternalEVMTypeCreateCadenceOwnedAccountFunction(gauge, handler),
			stdlib.InternalEVMTypeCallFunctionName:                      newInterpreterInternalEVMTypeCallFunction(gauge, handler),
			stdlib.InternalEVMTypeDepositFunctionName:                   newInterpreterInternalEVMTypeDepositFunction(gauge, handler),
			stdlib.InternalEVMTypeWithdrawFunctionName:                  newInterpreterInternalEVMTypeWithdrawFunction(gauge, handler),
			stdlib.InternalEVMTypeDeployFunctionName:                    newInterpreterInternalEVMTypeDeployFunction(gauge, handler),
			stdlib.InternalEVMTypeBalanceFunctionName:                   newInterpreterInternalEVMTypeBalanceFunction(gauge, handler),
			stdlib.InternalEVMTypeNonceFunctionName:                     newInterpreterInternalEVMTypeNonceFunction(gauge, handler),
			stdlib.InternalEVMTypeCodeFunctionName:                      newInterpreterInternalEVMTypeCodeFunction(gauge, handler),
			stdlib.InternalEVMTypeCodeHashFunctionName:                  newInterpreterInternalEVMTypeCodeHashFunction(gauge, handler),
			stdlib.InternalEVMTypeEncodeABIFunctionName:                 newInterpreterInternalEVMTypeEncodeABIFunction(gauge, location),
			stdlib.InternalEVMTypeDecodeABIFunctionName:                 newInterpreterInternalEVMTypeDecodeABIFunction(gauge, location),
			stdlib.InternalEVMTypeCastToAttoFLOWFunctionName:            newInterpreterInternalEVMTypeCastToAttoFLOWFunction(gauge),
			stdlib.InternalEVMTypeCastToFLOWFunctionName:                newInterpreterInternalEVMTypeCastToFLOWFunction(gauge),
			stdlib.InternalEVMTypeGetLatestBlockFunctionName:            newInterpreterInternalEVMTypeGetLatestBlockFunction(gauge, handler),
			stdlib.InternalEVMTypeDryRunFunctionName:                    newInterpreterInternalEVMTypeDryRunFunction(gauge, handler),
			stdlib.InternalEVMTypeDryCallFunctionName:                   newInterpreterInternalEVMTypeDryCallFunction(gauge, handler),
			stdlib.InternalEVMTypeCommitBlockProposalFunctionName:       newInterpreterInternalEVMTypeCommitBlockProposalFunction(gauge, handler),
		},
		nil,
		nil,
		nil,
		nil,
	)
}

func NewInternalEVMFunctions(
	handler types.ContractHandler,
	contractAddress flow.Address,
) stdlib.InternalEVMFunctions {
	location := common.NewAddressLocation(nil, common.Address(contractAddress), stdlib.ContractName)

	return stdlib.InternalEVMFunctions{
		Run:                       newVMInternalEVMTypeRunFunction(handler),
		BatchRun:                  newVMInternalEVMTypeBatchRunFunction(handler),
		CreateCadenceOwnedAccount: newVMInternalEVMTypeCreateCadenceOwnedAccountFunction(handler),
		Call:                      newVMInternalEVMTypeCallFunction(handler),
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
		DryRun:                    newVMInternalEVMTypeDryRunFunction(handler),
		DryCall:                   newVMInternalEVMTypeDryCallFunction(handler),
		CommitBlockProposal:       newVMInternalEVMTypeCommitBlockProposalFunction(handler),
	}

}

func newInterpreterInternalEVMTypeGetLatestBlockFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeGetLatestBlockFunctionType,
		interpreter.AdaptNativeFunctionForInterpreter(
			newInternalEVMTypeGetLatestBlockFunction(handler),
		),
	)
}

func newInternalEVMTypeGetLatestBlockFunction(
	handler types.ContractHandler,
) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
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
	var index int
	return interpreter.NewArrayValueWithIterator(
		context,
		stdlib.EVMAddressBytesStaticType,
		common.ZeroAddress,
		types.AddressLength,
		func() interpreter.Value {
			if index >= types.AddressLength {
				return nil
			}
			result := interpreter.NewUInt8Value(context, func() uint8 {
				return address[index]
			})
			index++
			return result
		},
	)
}

func EVMBytesToBytesArrayValue(
	context interpreter.ArrayCreationContext,
	bytes []byte,
) *interpreter.ArrayValue {
	var index int
	return interpreter.NewArrayValueWithIterator(
		context,
		stdlib.EVMBytesValueStaticType,
		common.ZeroAddress,
		uint64(len(bytes)),
		func() interpreter.Value {
			if index >= len(bytes) {
				return nil
			}
			result := interpreter.NewUInt8Value(context, func() uint8 {
				return bytes[index]
			})
			index++
			return result
		},
	)
}

func EVMBytes4ToBytesArrayValue(
	context interpreter.ArrayCreationContext,
	bytes [4]byte,
) *interpreter.ArrayValue {
	var index int
	return interpreter.NewArrayValueWithIterator(
		context,
		stdlib.EVMBytes4ValueStaticType,
		common.ZeroAddress,
		stdlib.EVMBytes4Length,
		func() interpreter.Value {
			if index >= stdlib.EVMBytes4Length {
				return nil
			}
			result := interpreter.NewUInt8Value(context, func() uint8 {
				return bytes[index]
			})
			index++
			return result
		},
	)
}

func EVMBytes32ToBytesArrayValue(
	context interpreter.ArrayCreationContext,
	bytes [32]byte,
) *interpreter.ArrayValue {
	var index int
	return interpreter.NewArrayValueWithIterator(
		context,
		stdlib.EVMBytes32ValueStaticType,
		common.ZeroAddress,
		stdlib.EVMBytes32Length,
		func() interpreter.Value {
			if index >= stdlib.EVMBytes32Length {
				return nil
			}
			result := interpreter.NewUInt8Value(context, func() uint8 {
				return bytes[index]
			})
			index++
			return result
		},
	)
}

func newInterpreterInternalEVMTypeCreateCadenceOwnedAccountFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeCreateCadenceOwnedAccountFunctionType,
		interpreter.AdaptNativeFunctionForInterpreter(
			newInternalEVMTypeCreateCadenceOwnedAccountFunction(handler),
		),
	)
}

func newInternalEVMTypeCreateCadenceOwnedAccountFunction(handler types.ContractHandler) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
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

// newInterpreterInternalEVMTypeCodeFunction returns the code of the account
func newInterpreterInternalEVMTypeCodeFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeCodeFunctionType,
		interpreter.AdaptNativeFunctionForInterpreter(
			newInternalEVMTypeCodeFunction(handler),
		),
	)
}

func newInternalEVMTypeCodeFunction(handler types.ContractHandler) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
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

// newInterpreterInternalEVMTypeNonceFunction returns the nonce of the account
func newInterpreterInternalEVMTypeNonceFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeNonceFunctionType,
		interpreter.AdaptNativeFunctionForInterpreter(
			newInternalEVMTypeNonceFunction(handler),
		),
	)
}

func newInternalEVMTypeNonceFunction(handler types.ContractHandler) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
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

func newInterpreterInternalEVMTypeCallFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeCallFunctionType,
		interpreter.AdaptNativeFunctionForInterpreter(
			newInternalEVMTypeCallFunction(handler),
		),
	)
}

func newInternalEVMTypeCallFunction(handler types.ContractHandler) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
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

func newInterpreterInternalEVMTypeDryCallFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeDryCallFunctionType,
		interpreter.AdaptNativeFunctionForInterpreter(
			newInternalEVMTypeDryCallFunction(handler),
		),
	)
}

func newInternalEVMTypeDryCallFunction(
	handler types.ContractHandler,
) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
		_ interpreter.Value,
		args []interpreter.Value,
	) interpreter.Value {

		callArgs, err := parseCallArguments(context, args)
		if err != nil {
			panic(err)
		}
		to := callArgs.to.ToCommon()

		tx := gethTypes.NewTx(&gethTypes.LegacyTx{
			Nonce:    0,
			To:       &to,
			Gas:      uint64(callArgs.gasLimit),
			Data:     callArgs.data,
			GasPrice: big.NewInt(0),
			Value:    callArgs.balance,
		})

		txPayload, err := tx.MarshalBinary()
		if err != nil {
			panic(err)
		}

		// call contract function

		res := handler.DryRun(txPayload, callArgs.from)

		return NewResultValue(
			handler,
			context,
			context,
			res,
		)
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

const fungibleTokenVaultTypeBalanceFieldName = "balance"

func newInterpreterInternalEVMTypeDepositFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeDepositFunctionType,
		interpreter.AdaptNativeFunctionForInterpreter(
			newInternalEVMTypeDepositFunction(handler),
		),
	)
}

func newInternalEVMTypeDepositFunction(handler types.ContractHandler) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
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

// newInterpreterInternalEVMTypeBalanceFunction returns the Flow balance of the account
func newInterpreterInternalEVMTypeBalanceFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeBalanceFunctionType,
		interpreter.AdaptNativeFunctionForInterpreter(
			newInternalEVMTypeBalanceFunction(handler),
		),
	)
}

func newInternalEVMTypeBalanceFunction(handler types.ContractHandler) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
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

// newInterpreterInternalEVMTypeCodeHashFunction returns the code hash of the account
func newInterpreterInternalEVMTypeCodeHashFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeCodeHashFunctionType,
		interpreter.AdaptNativeFunctionForInterpreter(
			newInternalEVMTypeCodeHashFunction(handler),
		),
	)
}

func newInternalEVMTypeCodeHashFunction(handler types.ContractHandler) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
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

func newInterpreterInternalEVMTypeWithdrawFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeWithdrawFunctionType,
		interpreter.AdaptNativeFunctionForInterpreter(
			newInternalEVMTypeWithdrawFunction(handler),
		),
	)
}

func newInternalEVMTypeWithdrawFunction(handler types.ContractHandler) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
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

		amount := types.NewBalance(amountValue.BigInt)

		// Withdraw

		const isAuthorized = true
		account := handler.AccountByAddress(fromAddress, isAuthorized)
		vault := account.Withdraw(amount)

		ufix, roundedOff, err := types.ConvertBalanceToUFix64(vault.Balance())
		if err != nil {
			panic(err)
		}
		if roundedOff {
			panic(types.ErrWithdrawBalanceRounding)
		}

		// TODO: improve: maybe call actual constructor
		return interpreter.NewCompositeValue(
			context,
			common.NewAddressLocation(
				context,
				handler.FlowTokenAddress(),
				"FlowToken",
			),
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

func newInterpreterInternalEVMTypeDeployFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeDeployFunctionType,
		interpreter.AdaptNativeFunctionForInterpreter(
			newInternalEVMTypeDeployFunction(handler),
		),
	)
}

func newInternalEVMTypeDeployFunction(
	handler types.ContractHandler,
) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
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

func newInterpreterInternalEVMTypeCastToAttoFLOWFunction(
	gauge common.MemoryGauge,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeCastToAttoFLOWFunctionType,
		interpreter.AdaptNativeFunctionForInterpreter(
			internalEVMTypeCastToAttoFLOWFunction,
		),
	)
}

var internalEVMTypeCastToAttoFLOWFunction = interpreter.NativeFunction(
	func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
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

func newInterpreterInternalEVMTypeCastToFLOWFunction(
	gauge common.MemoryGauge,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeCastToFLOWFunctionType,
		interpreter.AdaptNativeFunctionForInterpreter(
			internalEVMTypeCastToFLOWFunction,
		),
	)
}

var internalEVMTypeCastToFLOWFunction = interpreter.NativeFunction(
	func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
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

func newInterpreterInternalEVMTypeCommitBlockProposalFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeCommitBlockProposalFunctionType,
		interpreter.AdaptNativeFunctionForInterpreter(
			newInternalEVMTypeCommitBlockProposalFunction(handler),
		),
	)
}

func newInternalEVMTypeCommitBlockProposalFunction(
	handler types.ContractHandler,
) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
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

func newInterpreterInternalEVMTypeRunFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeRunFunctionType,
		interpreter.AdaptNativeFunctionForInterpreter(
			newInternalEVMTypeRunFunction(handler),
		),
	)
}

func newInternalEVMTypeRunFunction(
	handler types.ContractHandler,
) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
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

func newInterpreterInternalEVMTypeDryRunFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeDryRunFunctionType,
		interpreter.AdaptNativeFunctionForInterpreter(
			newInternalEVMTypeDryRunFunction(handler),
		),
	)
}

func newInternalEVMTypeDryRunFunction(
	handler types.ContractHandler,
) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
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

func newInterpreterInternalEVMTypeBatchRunFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeBatchRunFunctionType,
		interpreter.AdaptNativeFunctionForInterpreter(
			newInternalEVMTypeBatchRunFunction(handler),
		),
	)
}

func newInternalEVMTypeBatchRunFunction(
	handler types.ContractHandler,
) interpreter.NativeFunction {
	return func(
		context interpreter.NativeFunctionContext,
		_ interpreter.TypeArgumentsIterator,
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

		loc := common.NewAddressLocation(
			context,
			handler.EVMContractAddress(),
			stdlib.ContractName,
		)
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
		panic(errors.NewUnreachableError())
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
