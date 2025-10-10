package impl

import (
	"fmt"
	"math/big"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/bbq"
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
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext
			locationRange := invocation.LocationRange

			latestBlock := handler.LastExecutedBlock()

			return NewEVMBlockValue(
				handler,
				gauge,
				context,
				locationRange,
				latestBlock,
			)
		},
	)
}

func newVMInternalEVMTypeGetLatestBlockFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeGetLatestBlockFunctionName,
		stdlib.InternalEVMTypeGetLatestBlockFunctionType,
		func(context *vm.Context, _ []bbq.StaticType, receiver vm.Value, args ...vm.Value) vm.Value {

			latestBlock := handler.LastExecutedBlock()

			return NewEVMBlockValue(
				handler,
				context,
				context,
				interpreter.EmptyLocationRange,
				latestBlock,
			)
		},
	)
}

func NewEVMBlockValue(
	handler types.ContractHandler,
	gauge common.MemoryGauge,
	context interpreter.MemberAccessibleContext,
	locationRange interpreter.LocationRange,
	block *types.Block,
) *interpreter.CompositeValue {
	loc := common.NewAddressLocation(gauge, handler.EVMContractAddress(), stdlib.ContractName)
	hash, err := block.Hash()
	if err != nil {
		panic(err)
	}

	return interpreter.NewCompositeValue(
		context,
		locationRange,
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
	locationRange interpreter.LocationRange,
	location common.AddressLocation,
	address types.Address,
) *interpreter.CompositeValue {
	return interpreter.NewCompositeValue(
		context,
		locationRange,
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
	locationRange interpreter.LocationRange,
	location common.AddressLocation,
	bytes []byte,
) *interpreter.CompositeValue {
	return interpreter.NewCompositeValue(
		context,
		locationRange,
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
	locationRange interpreter.LocationRange,
	location common.AddressLocation,
	bytes [4]byte,
) *interpreter.CompositeValue {
	return interpreter.NewCompositeValue(
		context,
		locationRange,
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
	locationRange interpreter.LocationRange,
	location common.AddressLocation,
	bytes [32]byte,
) *interpreter.CompositeValue {
	return interpreter.NewCompositeValue(
		context,
		locationRange,
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
	locationRange interpreter.LocationRange,
	addressBytesValue *interpreter.ArrayValue,
) (
	result types.Address,
	err error,
) {
	// Convert

	var bytes []byte
	bytes, err = interpreter.ByteArrayValueToByteSlice(
		context,
		addressBytesValue,
		locationRange,
	)
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
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext
			args := invocation.Arguments

			uuid, ok := args[0].(interpreter.UInt64Value)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			return performDeployCOA(context, uuid, handler)
		},
	)
}

func newVMInternalEVMTypeCreateCadenceOwnedAccountFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeCreateCadenceOwnedAccountFunctionName,
		stdlib.InternalEVMTypeCreateCadenceOwnedAccountFunctionType,
		func(context *vm.Context, _ []bbq.StaticType, _ vm.Value, args ...vm.Value) vm.Value {

			uuid, ok := args[0].(interpreter.UInt64Value)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			return performDeployCOA(context, uuid, handler)
		},
	)
}

func performDeployCOA(
	context interpreter.InvocationContext,
	uuid interpreter.UInt64Value,
	handler types.ContractHandler,
) interpreter.Value {
	address := handler.DeployCOA(uint64(uuid))

	return EVMAddressToAddressBytesArrayValue(context, address)
}

// newInterpreterInternalEVMTypeCodeFunction returns the code of the account
func newInterpreterInternalEVMTypeCodeFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeCodeFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext
			locationRange := invocation.LocationRange
			args := invocation.Arguments

			addressValue, ok := args[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			return performCode(
				context,
				addressValue,
				locationRange,
				handler,
			)
		},
	)
}

func newVMInternalEVMTypeCodeFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeCodeFunctionName,
		stdlib.InternalEVMTypeCodeFunctionType,
		func(context *vm.Context, _ []bbq.StaticType, _ vm.Value, args ...vm.Value) vm.Value {

			addressValue, ok := args[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			return performCode(
				context,
				addressValue,
				interpreter.EmptyLocationRange,
				handler,
			)
		},
	)
}

func performCode(
	context interpreter.InvocationContext,
	addressValue *interpreter.ArrayValue,
	locationRange interpreter.LocationRange,
	handler types.ContractHandler,
) interpreter.Value {
	address, err := AddressBytesArrayValueToEVMAddress(
		context,
		locationRange,
		addressValue,
	)
	if err != nil {
		panic(err)
	}

	const isAuthorized = false
	account := handler.AccountByAddress(address, isAuthorized)

	return interpreter.ByteSliceToByteArrayValue(context, account.Code())
}

// newInterpreterInternalEVMTypeNonceFunction returns the nonce of the account
func newInterpreterInternalEVMTypeNonceFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeNonceFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext
			locationRange := invocation.LocationRange
			args := invocation.Arguments

			addressValue, ok := args[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			return performNonce(
				context,
				addressValue,
				locationRange,
				handler,
			)
		},
	)
}

func newVMInternalEVMTypeNonceFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeNonceFunctionName,
		stdlib.InternalEVMTypeNonceFunctionType,
		func(context *vm.Context, _ []bbq.StaticType, _ vm.Value, args ...vm.Value) vm.Value {

			addressValue, ok := args[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			return performNonce(
				context,
				addressValue,
				interpreter.EmptyLocationRange,
				handler,
			)
		},
	)
}

func performNonce(
	context interpreter.InvocationContext,
	addressValue *interpreter.ArrayValue,
	locationRange interpreter.LocationRange,
	handler types.ContractHandler,
) interpreter.Value {
	address, err := AddressBytesArrayValueToEVMAddress(
		context,
		locationRange,
		addressValue,
	)
	if err != nil {
		panic(err)
	}

	const isAuthorized = false
	account := handler.AccountByAddress(address, isAuthorized)

	return interpreter.UInt64Value(account.Nonce())
}

func newInterpreterInternalEVMTypeCallFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeCallFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext
			locationRange := invocation.LocationRange
			args := invocation.Arguments

			return performCall(
				context,
				args,
				locationRange,
				handler,
			)
		},
	)
}

func newVMInternalEVMTypeCallFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeCallFunctionName,
		stdlib.InternalEVMTypeCallFunctionType,
		func(context *vm.Context, _ []bbq.StaticType, _ vm.Value, args ...vm.Value) vm.Value {

			return performCall(
				context,
				args,
				interpreter.EmptyLocationRange,
				handler,
			)
		},
	)
}

func performCall(
	context interpreter.InvocationContext,
	args []interpreter.Value,
	locationRange interpreter.LocationRange,
	handler types.ContractHandler,
) interpreter.Value {
	callArgs, err := parseCallArguments(
		context,
		args,
		locationRange,
	)
	if err != nil {
		panic(err)
	}

	// Call

	const isAuthorized = true
	account := handler.AccountByAddress(callArgs.from, isAuthorized)
	result := account.Call(
		callArgs.to,
		callArgs.data,
		callArgs.gasLimit,
		callArgs.balance,
	)

	return NewResultValue(
		handler,
		context,
		context,
		locationRange,
		result,
	)
}

func newInterpreterInternalEVMTypeDryCallFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeDryCallFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext
			locationRange := invocation.LocationRange
			args := invocation.Arguments

			return performDryCall(
				context,
				args,
				locationRange,
				handler,
			)
		},
	)
}

func newVMInternalEVMTypeDryCallFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeDryCallFunctionName,
		stdlib.InternalEVMTypeDryCallFunctionType,
		func(context *vm.Context, _ []bbq.StaticType, _ vm.Value, args ...vm.Value) vm.Value {

			return performDryCall(
				context,
				args,
				interpreter.EmptyLocationRange,
				handler,
			)
		},
	)
}

func performDryCall(
	context interpreter.InvocationContext,
	args []interpreter.Value,
	locationRange interpreter.LocationRange,
	handler types.ContractHandler,
) interpreter.Value {
	callArgs, err := parseCallArguments(
		context,
		args,
		locationRange,
	)
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
		locationRange,
		res,
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
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext
			locationRange := invocation.LocationRange
			args := invocation.Arguments

			// Get from vault

			fromValue, ok := args[0].(*interpreter.CompositeValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			// Get to address

			toAddressValue, ok := args[1].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			performDeposit(
				context,
				fromValue,
				toAddressValue,
				locationRange,
				handler,
			)

			return interpreter.Void
		},
	)
}

func newVMInternalEVMTypeDepositFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeDepositFunctionName,
		stdlib.InternalEVMTypeDepositFunctionType,
		func(context *vm.Context, _ []bbq.StaticType, _ vm.Value, args ...vm.Value) vm.Value {

			// Get from vault

			fromValue, ok := args[0].(*interpreter.CompositeValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			// Get to address

			toAddressValue, ok := args[1].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			performDeposit(
				context,
				fromValue,
				toAddressValue,
				interpreter.EmptyLocationRange,
				handler,
			)

			return interpreter.Void
		},
	)
}

func performDeposit(
	context interpreter.InvocationContext,
	fromValue *interpreter.CompositeValue,
	toAddressValue *interpreter.ArrayValue,
	locationRange interpreter.LocationRange,
	handler types.ContractHandler,
) {
	amountValue, ok := fromValue.GetField(
		context,
		fungibleTokenVaultTypeBalanceFieldName,
	).(interpreter.UFix64Value)
	if !ok {
		panic(errors.NewUnreachableError())
	}

	amount := types.NewBalanceFromUFix64(cadence.UFix64(amountValue.UFix64Value))

	toAddress, err := AddressBytesArrayValueToEVMAddress(
		context,
		locationRange,
		toAddressValue,
	)
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
}

// newInterpreterInternalEVMTypeBalanceFunction returns the Flow balance of the account
func newInterpreterInternalEVMTypeBalanceFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeBalanceFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext
			locationRange := invocation.LocationRange
			args := invocation.Arguments

			addressValue, ok := args[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			address, err := AddressBytesArrayValueToEVMAddress(context, locationRange, addressValue)
			if err != nil {
				panic(err)
			}

			return performBalance(
				context,
				address,
				handler,
			)
		},
	)
}

func newVMInternalEVMTypeBalanceFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeBalanceFunctionName,
		stdlib.InternalEVMTypeBalanceFunctionType,
		func(context *vm.Context, _ []bbq.StaticType, _ vm.Value, args ...vm.Value) vm.Value {

			addressValue, ok := args[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			address, err := AddressBytesArrayValueToEVMAddress(
				context,
				interpreter.EmptyLocationRange,
				addressValue,
			)
			if err != nil {
				panic(err)
			}

			return performBalance(
				context,
				address,
				handler,
			)
		},
	)
}

func performBalance(
	context interpreter.InvocationContext,
	address types.Address,
	handler types.ContractHandler,
) interpreter.Value {
	const isAuthorized = false
	account := handler.AccountByAddress(address, isAuthorized)

	balance := account.Balance()
	memoryUsage := common.NewBigIntMemoryUsage(
		common.BigIntByteLength(balance),
	)
	return interpreter.NewUIntValueFromBigInt(
		context,
		memoryUsage,
		func() *big.Int {
			return balance
		},
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
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext
			locationRange := invocation.LocationRange
			args := invocation.Arguments

			addressValue, ok := args[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			return performCodeHash(context, locationRange, addressValue, handler)
		},
	)
}

func newVMInternalEVMTypeCodeHashFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeCodeHashFunctionName,
		stdlib.InternalEVMTypeCodeHashFunctionType,
		func(context *vm.Context, _ []bbq.StaticType, _ vm.Value, args ...vm.Value) vm.Value {

			// Get address

			addressValue, ok := args[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			return performCodeHash(
				context,
				interpreter.EmptyLocationRange,
				addressValue,
				handler,
			)
		},
	)
}

func performCodeHash(
	context interpreter.InvocationContext,
	locationRange interpreter.LocationRange,
	addressValue *interpreter.ArrayValue,
	handler types.ContractHandler,
) interpreter.Value {
	address, err := AddressBytesArrayValueToEVMAddress(
		context,
		locationRange,
		addressValue,
	)
	if err != nil {
		panic(err)
	}

	const isAuthorized = false
	account := handler.AccountByAddress(address, isAuthorized)

	return interpreter.ByteSliceToByteArrayValue(context, account.CodeHash())
}

func newInterpreterInternalEVMTypeWithdrawFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeWithdrawFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext
			locationRange := invocation.LocationRange
			args := invocation.Arguments

			// Get from address

			fromAddressValue, ok := args[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			// Get amount

			amountValue, ok := args[1].(interpreter.UIntValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			return performWithdraw(
				context,
				fromAddressValue,
				amountValue,
				locationRange,
				handler,
			)
		},
	)
}

func newVMInternalEVMTypeWithdrawFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeWithdrawFunctionName,
		stdlib.InternalEVMTypeWithdrawFunctionType,
		func(context *vm.Context, _ []bbq.StaticType, _ vm.Value, args ...vm.Value) vm.Value {

			// Get from address

			fromAddressValue, ok := args[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			// Get amount

			amountValue, ok := args[1].(interpreter.UIntValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			return performWithdraw(
				context,
				fromAddressValue,
				amountValue,
				interpreter.EmptyLocationRange,
				handler,
			)
		},
	)
}

func performWithdraw(
	context interpreter.InvocationContext,
	fromAddressValue *interpreter.ArrayValue,
	amountValue interpreter.UIntValue,
	locationRange interpreter.LocationRange,
	handler types.ContractHandler,
) interpreter.Value {

	fromAddress, err := AddressBytesArrayValueToEVMAddress(
		context,
		locationRange,
		fromAddressValue,
	)
	if err != nil {
		panic(err)
	}

	amount := types.NewBalance(amountValue.BigInt)

	// Withdraw

	const isAuthorized = true
	account := handler.AccountByAddress(fromAddress, isAuthorized)
	vault := account.Withdraw(amount)

	// TODO: improve: maybe call actual constructor
	return interpreter.NewCompositeValue(
		context,
		locationRange,
		common.NewAddressLocation(context, handler.FlowTokenAddress(), "FlowToken"),
		"FlowToken.Vault",
		common.CompositeKindResource,
		[]interpreter.CompositeField{
			{
				Name: "balance",
				Value: interpreter.NewUFix64Value(context, func() uint64 {
					ufix, roundedOff, err := types.ConvertBalanceToUFix64(vault.Balance())
					if err != nil {
						panic(err)
					}
					if roundedOff {
						panic(types.ErrWithdrawBalanceRounding)
					}
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

func newInterpreterInternalEVMTypeDeployFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeDeployFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext
			locationRange := invocation.LocationRange
			args := invocation.Arguments

			// Get from address

			fromAddressValue, ok := args[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			// Get code

			codeValue, ok := args[1].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			// Get gas limit

			gasLimitValue, ok := args[2].(interpreter.UInt64Value)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			// Get amount

			amountValue, ok := args[3].(interpreter.UIntValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			return performDeploy(
				context,
				fromAddressValue,
				codeValue,
				gasLimitValue,
				amountValue,
				locationRange,
				handler,
			)
		},
	)
}

func newVMInternalEVMTypeDeployFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeDeployFunctionName,
		stdlib.InternalEVMTypeDeployFunctionType,
		func(context *vm.Context, _ []bbq.StaticType, _ vm.Value, args ...vm.Value) vm.Value {

			// Get from address

			fromAddressValue, ok := args[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			// Get code

			codeValue, ok := args[1].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			// Get gas limit

			gasLimitValue, ok := args[2].(interpreter.UInt64Value)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			// Get amount

			amountValue, ok := args[3].(interpreter.UIntValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			return performDeploy(
				context,
				fromAddressValue,
				codeValue,
				gasLimitValue,
				amountValue,
				interpreter.EmptyLocationRange,
				handler,
			)
		},
	)
}

func performDeploy(
	context interpreter.InvocationContext,
	fromAddressValue *interpreter.ArrayValue,
	codeValue *interpreter.ArrayValue,
	gasLimitValue interpreter.UInt64Value,
	amountValue interpreter.UIntValue,
	locationRange interpreter.LocationRange,
	handler types.ContractHandler,
) interpreter.Value {
	fromAddress, err := AddressBytesArrayValueToEVMAddress(context, locationRange, fromAddressValue)
	if err != nil {
		panic(err)
	}

	code, err := interpreter.ByteArrayValueToByteSlice(context, codeValue, locationRange)
	if err != nil {
		panic(err)
	}

	gasLimit := types.GasLimit(gasLimitValue)

	amount := types.NewBalance(amountValue.BigInt)

	// Deploy

	const isAuthorized = true
	account := handler.AccountByAddress(fromAddress, isAuthorized)
	result := account.Deploy(code, gasLimit, amount)

	return NewResultValue(
		handler,
		context,
		context,
		locationRange,
		result,
	)
}

func newInterpreterInternalEVMTypeCastToAttoFLOWFunction(
	gauge common.MemoryGauge,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeCastToAttoFLOWFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext
			args := invocation.Arguments

			balanceValue, ok := args[0].(interpreter.UFix64Value)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			return performCastToAttoFLOW(
				context,
				balanceValue,
			)
		},
	)
}

var vmInternalEVMTypeCastToAttoFLOWFunction = vm.NewNativeFunctionValue(
	stdlib.InternalEVMTypeCastToAttoFLOWFunctionName,
	stdlib.InternalEVMTypeCastToAttoFLOWFunctionType,
	func(context *vm.Context, _ []bbq.StaticType, _ vm.Value, args ...vm.Value) vm.Value {

		// Get balance

		balanceValue, ok := args[0].(interpreter.UFix64Value)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		return performCastToAttoFLOW(
			context,
			balanceValue,
		)
	},
)

func performCastToAttoFLOW(
	context interpreter.InvocationContext,
	balanceValue interpreter.UFix64Value,
) interpreter.Value {
	balance := types.NewBalanceFromUFix64(cadence.UFix64(balanceValue.UFix64Value))
	memoryUsage := common.NewBigIntMemoryUsage(
		common.BigIntByteLength(balance),
	)
	return interpreter.NewUIntValueFromBigInt(
		context,
		memoryUsage,
		func() *big.Int {
			return balance
		},
	)
}

func newInterpreterInternalEVMTypeCastToFLOWFunction(
	gauge common.MemoryGauge,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeCastToFLOWFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			args := invocation.Arguments

			balanceValue, ok := args[0].(interpreter.UIntValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			return performCastToFLOW(gauge, balanceValue)
		},
	)
}

var vmInternalEVMTypeCastToFLOWFunction = vm.NewNativeFunctionValue(
	stdlib.InternalEVMTypeCastToFLOWFunctionName,
	stdlib.InternalEVMTypeCastToFLOWFunctionType,
	func(context *vm.Context, _ []bbq.StaticType, _ vm.Value, args ...vm.Value) vm.Value {

		// Get balance

		balanceValue, ok := args[0].(interpreter.UIntValue)
		if !ok {
			panic(errors.NewUnreachableError())
		}

		return performCastToFLOW(context, balanceValue)
	},
)

func performCastToFLOW(
	gauge common.MemoryGauge,
	balanceValue interpreter.UIntValue,
) interpreter.Value {
	return interpreter.NewUFix64Value(gauge, func() uint64 {
		balance := types.NewBalance(balanceValue.BigInt)
		// ignoring the rounding error and let user handle it
		v, _, err := types.ConvertBalanceToUFix64(balance)
		if err != nil {
			panic(err)
		}
		return uint64(v)
	})
}

func newInterpreterInternalEVMTypeCommitBlockProposalFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeCommitBlockProposalFunctionType,
		func(_ interpreter.Invocation) interpreter.Value {
			handler.CommitBlockProposal()
			return interpreter.Void
		},
	)
}

func newVMInternalEVMTypeCommitBlockProposalFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeCommitBlockProposalFunctionName,
		stdlib.InternalEVMTypeCommitBlockProposalFunctionType,
		func(_ *vm.Context, _ []bbq.StaticType, _ vm.Value, _ ...vm.Value) vm.Value {
			handler.CommitBlockProposal()
			return interpreter.Void
		},
	)
}

func newInterpreterInternalEVMTypeRunFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeRunFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext
			locationRange := invocation.LocationRange
			args := invocation.Arguments

			// Get transaction argument

			transactionValue, ok := args[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			// Get gas fee collector argument

			gasFeeCollectorValue, ok := args[1].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			return performRun(
				context,
				transactionValue,
				gasFeeCollectorValue,
				locationRange,
				handler,
			)
		},
	)
}

func newVMInternalEVMTypeRunFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeRunFunctionName,
		stdlib.InternalEVMTypeRunFunctionType,
		func(context *vm.Context, _ []bbq.StaticType, _ vm.Value, args ...vm.Value) vm.Value {

			// Get transaction argument

			transactionValue, ok := args[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			// Get gas fee collector argument

			gasFeeCollectorValue, ok := args[1].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			return performRun(
				context,
				transactionValue,
				gasFeeCollectorValue,
				interpreter.EmptyLocationRange,
				handler,
			)
		},
	)
}

func performRun(
	context interpreter.InvocationContext,
	transactionValue *interpreter.ArrayValue,
	gasFeeCollectorValue *interpreter.ArrayValue,
	locationRange interpreter.LocationRange,
	handler types.ContractHandler,
) interpreter.Value {
	transaction, err := interpreter.ByteArrayValueToByteSlice(
		context,
		transactionValue,
		locationRange,
	)
	if err != nil {
		panic(err)
	}

	gasFeeCollector, err := interpreter.ByteArrayValueToByteSlice(
		context,
		gasFeeCollectorValue,
		locationRange,
	)
	if err != nil {
		panic(err)
	}

	// run transaction
	result := handler.Run(
		transaction,
		types.NewAddressFromBytes(gasFeeCollector),
	)

	return NewResultValue(
		handler,
		context,
		context,
		locationRange,
		result,
	)
}

func newInterpreterInternalEVMTypeDryRunFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeDryRunFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext
			locationRange := invocation.LocationRange
			args := invocation.Arguments

			// Get transaction argument

			transactionValue, ok := args[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			// Get from argument

			fromValue, ok := args[1].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			return performDryRun(
				context,
				transactionValue,
				fromValue,
				locationRange,
				handler,
			)
		},
	)
}

func newVMInternalEVMTypeDryRunFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeDryRunFunctionName,
		stdlib.InternalEVMTypeDryRunFunctionType,
		func(context *vm.Context, _ []bbq.StaticType, _ vm.Value, args ...vm.Value) vm.Value {

			// Get transaction argument

			transactionValue, ok := args[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			// Get from argument

			fromValue, ok := args[1].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			return performDryRun(
				context,
				transactionValue,
				fromValue,
				interpreter.EmptyLocationRange,
				handler,
			)
		},
	)
}

func performDryRun(
	context interpreter.InvocationContext,
	transactionValue *interpreter.ArrayValue,
	fromValue *interpreter.ArrayValue,
	locationRange interpreter.LocationRange,
	handler types.ContractHandler,
) interpreter.Value {
	transaction, err := interpreter.ByteArrayValueToByteSlice(context, transactionValue, locationRange)
	if err != nil {
		panic(err)
	}

	from, err := interpreter.ByteArrayValueToByteSlice(context, fromValue, locationRange)
	if err != nil {
		panic(err)
	}

	// call estimate

	res := handler.DryRun(transaction, types.NewAddressFromBytes(from))
	return NewResultValue(
		handler,
		context,
		context,
		locationRange,
		res,
	)
}

func newInterpreterInternalEVMTypeBatchRunFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeBatchRunFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext
			locationRange := invocation.LocationRange
			args := invocation.Arguments

			// Get transactions batch argument
			transactionsBatchValue, ok := args[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			// Get gas fee collector argument
			gasFeeCollectorValue, ok := args[1].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			return performBatchRun(
				context,
				transactionsBatchValue,
				gasFeeCollectorValue,
				locationRange,
				handler,
			)
		},
	)
}

func newVMInternalEVMTypeBatchRunFunction(
	handler types.ContractHandler,
) *vm.NativeFunctionValue {
	return vm.NewNativeFunctionValue(
		stdlib.InternalEVMTypeBatchRunFunctionName,
		stdlib.InternalEVMTypeBatchRunFunctionType,
		func(context *vm.Context, _ []bbq.StaticType, _ vm.Value, args ...vm.Value) vm.Value {

			// Get transactions batch argument
			transactionsBatchValue, ok := args[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			// Get gas fee collector argument
			gasFeeCollectorValue, ok := args[1].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			return performBatchRun(
				context,
				transactionsBatchValue,
				gasFeeCollectorValue,
				interpreter.EmptyLocationRange,
				handler,
			)
		},
	)
}

func performBatchRun(
	context interpreter.InvocationContext,
	transactionsBatchValue *interpreter.ArrayValue,
	gasFeeCollectorValue *interpreter.ArrayValue,
	locationRange interpreter.LocationRange,
	handler types.ContractHandler,
) interpreter.Value {

	batchCount := transactionsBatchValue.Count()
	var transactionBatch [][]byte
	if batchCount > 0 {
		transactionBatch = make([][]byte, batchCount)
		i := 0
		transactionsBatchValue.Iterate(context, func(transactionValue interpreter.Value) (resume bool) {
			t, err := interpreter.ByteArrayValueToByteSlice(context, transactionValue, locationRange)
			if err != nil {
				panic(err)
			}
			transactionBatch[i] = t
			i++
			return true
		}, false, locationRange)
	}

	gasFeeCollector, err := interpreter.ByteArrayValueToByteSlice(
		context,
		gasFeeCollectorValue,
		locationRange,
	)
	if err != nil {
		panic(err)
	}

	// Batch run
	batchResults := handler.BatchRun(transactionBatch, types.NewAddressFromBytes(gasFeeCollector))

	values := newResultValues(handler, context, context, locationRange, batchResults)

	location := common.NewAddressLocation(
		context,
		handler.EVMContractAddress(),
		stdlib.ContractName,
	)
	evmResultType := interpreter.NewVariableSizedStaticType(
		context,
		interpreter.NewCompositeStaticType(
			nil,
			location,
			stdlib.EVMResultTypeQualifiedIdentifier,
			common.NewTypeIDFromQualifiedName(
				nil,
				location,
				stdlib.EVMResultTypeQualifiedIdentifier,
			),
		),
	)

	return interpreter.NewArrayValue(
		context,
		locationRange,
		evmResultType,
		common.ZeroAddress,
		values...,
	)
}

// newResultValues converts batch run result summary type to cadence array of structs
func newResultValues(
	handler types.ContractHandler,
	gauge common.MemoryGauge,
	context interpreter.MemberAccessibleContext,
	locationRange interpreter.LocationRange,
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
				locationRange,
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
	locationRange interpreter.LocationRange,
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
				locationRange,
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
				locationRange,
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
		locationRange,
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
	locationRange interpreter.LocationRange,
) (
	*callArguments,
	error,
) {
	// Get from address

	fromAddressValue, ok := args[0].(*interpreter.ArrayValue)
	if !ok {
		return nil, errors.NewUnreachableError()
	}

	fromAddress, err := AddressBytesArrayValueToEVMAddress(context, locationRange, fromAddressValue)
	if err != nil {
		return nil, err
	}

	// Get to address

	toAddressValue, ok := args[1].(*interpreter.ArrayValue)
	if !ok {
		return nil, errors.NewUnreachableError()
	}

	toAddress, err := AddressBytesArrayValueToEVMAddress(context, locationRange, toAddressValue)
	if err != nil {
		return nil, err
	}

	// Get data

	dataValue, ok := args[2].(*interpreter.ArrayValue)
	if !ok {
		return nil, errors.NewUnreachableError()
	}

	data, err := interpreter.ByteArrayValueToByteSlice(context, dataValue, locationRange)
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
