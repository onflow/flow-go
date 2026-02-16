package impl

import (
	"fmt"
	"math/big"

	"github.com/holiman/uint256"
	"github.com/onflow/cadence"
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
	location := common.NewAddressLocation(nil, common.Address(contractAddress), stdlib.ContractName)

	return interpreter.NewSimpleCompositeValue(
		gauge,
		stdlib.InternalEVMContractType.ID(),
		internalEVMContractStaticType,
		stdlib.InternalEVMContractType.Fields,
		map[string]interpreter.Value{
			stdlib.InternalEVMTypeRunFunctionName:                       newInternalEVMTypeRunFunction(gauge, handler),
			stdlib.InternalEVMTypeBatchRunFunctionName:                  newInternalEVMTypeBatchRunFunction(gauge, handler),
			stdlib.InternalEVMTypeCreateCadenceOwnedAccountFunctionName: newInternalEVMTypeCreateCadenceOwnedAccountFunction(gauge, handler),
			stdlib.InternalEVMTypeCallFunctionName:                      newInternalEVMTypeCallFunction(gauge, handler),
			stdlib.InternalEVMTypeDepositFunctionName:                   newInternalEVMTypeDepositFunction(gauge, handler),
			stdlib.InternalEVMTypeWithdrawFunctionName:                  newInternalEVMTypeWithdrawFunction(gauge, handler),
			stdlib.InternalEVMTypeDeployFunctionName:                    newInternalEVMTypeDeployFunction(gauge, handler),
			stdlib.InternalEVMTypeBalanceFunctionName:                   newInternalEVMTypeBalanceFunction(gauge, handler),
			stdlib.InternalEVMTypeNonceFunctionName:                     newInternalEVMTypeNonceFunction(gauge, handler),
			stdlib.InternalEVMTypeCodeFunctionName:                      newInternalEVMTypeCodeFunction(gauge, handler),
			stdlib.InternalEVMTypeCodeHashFunctionName:                  newInternalEVMTypeCodeHashFunction(gauge, handler),
			stdlib.InternalEVMTypeEncodeABIFunctionName:                 newInternalEVMTypeEncodeABIFunction(gauge, location),
			stdlib.InternalEVMTypeDecodeABIFunctionName:                 newInternalEVMTypeDecodeABIFunction(gauge, location),
			stdlib.InternalEVMTypeCastToAttoFLOWFunctionName:            newInternalEVMTypeCastToAttoFLOWFunction(gauge),
			stdlib.InternalEVMTypeCastToFLOWFunctionName:                newInternalEVMTypeCastToFLOWFunction(gauge),
			stdlib.InternalEVMTypeGetLatestBlockFunctionName:            newInternalEVMTypeGetLatestBlockFunction(gauge, handler),
			stdlib.InternalEVMTypeDryRunFunctionName:                    newInternalEVMTypeDryRunFunction(gauge, handler),
			stdlib.InternalEVMTypeDryCallFunctionName:                   newInternalEVMTypeDryCallFunction(gauge, handler),
			stdlib.InternalEVMTypeCommitBlockProposalFunctionName:       newInternalEVMTypeCommitBlockProposalFunction(gauge, handler),
		},
		nil,
		nil,
		nil,
		nil,
	)
}

func newInternalEVMTypeGetLatestBlockFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeGetLatestBlockFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext

			latestBlock := handler.LastExecutedBlock()

			return NewEVMBlockValue(
				handler,
				gauge,
				context,
				latestBlock,
			)
		},
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

func newInternalEVMTypeCreateCadenceOwnedAccountFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeCreateCadenceOwnedAccountFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext

			uuid, ok := invocation.Arguments[0].(interpreter.UInt64Value)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			address := handler.DeployCOA(uint64(uuid))

			return EVMAddressToAddressBytesArrayValue(context, address)
		},
	)
}

// newInternalEVMTypeCodeFunction returns the code of the account
func newInternalEVMTypeCodeFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeCodeFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext

			addressValue, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
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
		},
	)
}

// newInternalEVMTypeNonceFunction returns the nonce of the account
func newInternalEVMTypeNonceFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeNonceFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext

			addressValue, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
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
		},
	)
}

func newInternalEVMTypeCallFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeCallFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext

			callArgs, err := parseCallArguments(invocation)
			if err != nil {
				panic(err)
			}

			// Call

			const isAuthorized = true
			account := handler.AccountByAddress(callArgs.from, isAuthorized)
			result := account.Call(callArgs.to, callArgs.data, callArgs.gasLimit, callArgs.balance)

			return NewResultValue(
				handler,
				gauge,
				context,
				result,
			)
		},
	)
}

func newInternalEVMTypeDryCallFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeDryCallFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext

			callArgs, err := parseCallArguments(invocation)
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
			return NewResultValue(handler, gauge, context, res)
		},
	)
}

const fungibleTokenVaultTypeBalanceFieldName = "balance"

func newInternalEVMTypeDepositFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeDepositFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext

			// Get from vault

			fromValue, ok := invocation.Arguments[0].(*interpreter.CompositeValue)
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

			toAddressValue, ok := invocation.Arguments[1].(*interpreter.ArrayValue)
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
		},
	)
}

// newInternalEVMTypeBalanceFunction returns the Flow balance of the account
func newInternalEVMTypeBalanceFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeBalanceFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext

			addressValue, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
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
		},
	)
}

// newInternalEVMTypeCodeHashFunction returns the code hash of the account
func newInternalEVMTypeCodeHashFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeCodeHashFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext

			addressValue, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
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
		},
	)
}

func newInternalEVMTypeWithdrawFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeWithdrawFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext

			// Get from address

			fromAddressValue, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			fromAddress, err := AddressBytesArrayValueToEVMAddress(context, fromAddressValue)
			if err != nil {
				panic(err)
			}

			// Get amount

			amountValue, ok := invocation.Arguments[1].(interpreter.UIntValue)
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
				common.NewAddressLocation(gauge, handler.FlowTokenAddress(), "FlowToken"),
				"FlowToken.Vault",
				common.CompositeKindResource,
				[]interpreter.CompositeField{
					{
						Name: "balance",
						Value: interpreter.NewUFix64Value(gauge, func() uint64 {
							return uint64(ufix)
						}),
					},
					{
						Name: sema.ResourceUUIDFieldName,
						Value: interpreter.NewUInt64Value(gauge, func() uint64 {
							return handler.GenerateResourceUUID()
						}),
					},
				},
				common.ZeroAddress,
			)
		},
	)
}

func newInternalEVMTypeDeployFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeDeployFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext

			// Get from address

			fromAddressValue, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			fromAddress, err := AddressBytesArrayValueToEVMAddress(context, fromAddressValue)
			if err != nil {
				panic(err)
			}

			// Get code

			codeValue, ok := invocation.Arguments[1].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			code, err := interpreter.ByteArrayValueToByteSlice(context, codeValue)
			if err != nil {
				panic(err)
			}

			// Get gas limit

			gasLimitValue, ok := invocation.Arguments[2].(interpreter.UInt64Value)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			gasLimit := types.GasLimit(gasLimitValue)

			// Get value

			amountValue, ok := invocation.Arguments[3].(interpreter.UIntValue)
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
				gauge,
				context,
				result,
			)
		},
	)
}

func newInternalEVMTypeCastToAttoFLOWFunction(
	gauge common.MemoryGauge,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeCastToAttoFLOWFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			balanceValue, ok := invocation.Arguments[0].(interpreter.UFix64Value)
			if !ok {
				panic(errors.NewUnreachableError())
			}
			balance := types.NewBalanceFromUFix64(cadence.UFix64(balanceValue.UFix64Value))
			return interpreter.UIntValue{BigInt: balance}
		},
	)
}

func newInternalEVMTypeCastToFLOWFunction(
	gauge common.MemoryGauge,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeCastToFLOWFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			balanceValue, ok := invocation.Arguments[0].(interpreter.UIntValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}
			balance := types.NewBalance(balanceValue.BigInt)
			// ignoring the rounding error and let user handle it
			v, _, err := types.ConvertBalanceToUFix64(balance)
			if err != nil {
				panic(err)
			}
			return interpreter.NewUFix64Value(gauge, func() uint64 {
				return uint64(v)
			})
		},
	)
}

func newInternalEVMTypeCommitBlockProposalFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeCommitBlockProposalFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			handler.CommitBlockProposal()
			return interpreter.Void
		},
	)
}

func newInternalEVMTypeRunFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeRunFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext

			// Get transaction argument

			transactionValue, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			transaction, err := interpreter.ByteArrayValueToByteSlice(context, transactionValue)
			if err != nil {
				panic(err)
			}

			// Get gas fee collector argument

			gasFeeCollectorValue, ok := invocation.Arguments[1].(*interpreter.ArrayValue)
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
				gauge,
				context,
				result,
			)
		},
	)
}

func newInternalEVMTypeDryRunFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeDryRunFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext

			// Get transaction argument

			transactionValue, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			transaction, err := interpreter.ByteArrayValueToByteSlice(context, transactionValue)
			if err != nil {
				panic(err)
			}

			// Get from argument

			fromValue, ok := invocation.Arguments[1].(*interpreter.ArrayValue)
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
				gauge,
				context,
				res,
			)
		},
	)
}

func newInternalEVMTypeBatchRunFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeBatchRunFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext

			// Get transactions batch argument
			transactionsBatchValue, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
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
			gasFeeCollectorValue, ok := invocation.Arguments[1].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			gasFeeCollector, err := interpreter.ByteArrayValueToByteSlice(context, gasFeeCollectorValue)
			if err != nil {
				panic(err)
			}

			// Batch run
			batchResults := handler.BatchRun(transactionBatch, types.NewAddressFromBytes(gasFeeCollector))

			values := newResultValues(handler, gauge, context, batchResults)

			loc := common.NewAddressLocation(gauge, handler.EVMContractAddress(), stdlib.ContractName)
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
		},
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

func parseCallArguments(invocation interpreter.Invocation) (
	*callArguments,
	error,
) {
	context := invocation.InvocationContext

	// Get from address

	fromAddressValue, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
	if !ok {
		return nil, errors.NewUnreachableError()
	}

	fromAddress, err := AddressBytesArrayValueToEVMAddress(context, fromAddressValue)
	if err != nil {
		return nil, err
	}

	// Get to address

	toAddressValue, ok := invocation.Arguments[1].(*interpreter.ArrayValue)
	if !ok {
		return nil, errors.NewUnreachableError()
	}

	toAddress, err := AddressBytesArrayValueToEVMAddress(context, toAddressValue)
	if err != nil {
		return nil, err
	}

	// Get data

	dataValue, ok := invocation.Arguments[2].(*interpreter.ArrayValue)
	if !ok {
		return nil, errors.NewUnreachableError()
	}

	data, err := interpreter.ByteArrayValueToByteSlice(context, dataValue)
	if err != nil {
		return nil, err
	}

	// Get gas limit

	gasLimitValue, ok := invocation.Arguments[3].(interpreter.UInt64Value)
	if !ok {
		panic(errors.NewUnreachableError())
	}

	gasLimit := types.GasLimit(gasLimitValue)

	// Get balance

	balanceValue, ok := invocation.Arguments[4].(interpreter.UIntValue)
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
