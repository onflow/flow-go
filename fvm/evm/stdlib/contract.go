package stdlib

import (
	_ "embed"
	"fmt"
	"regexp"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

//go:embed contract.cdc
var contractCode string

var flowTokenImportPattern = regexp.MustCompile(`^import "FlowToken"\n`)

func ContractCode(flowTokenAddress flow.Address) []byte {
	return []byte(flowTokenImportPattern.ReplaceAllString(
		contractCode,
		fmt.Sprintf("import FlowToken from %s", flowTokenAddress.HexWithPrefix()),
	))
}

const ContractName = "EVM"

var EVMTransactionBytesCadenceType = cadence.NewVariableSizedArrayType(cadence.UInt8Type)
var evmTransactionBytesType = sema.NewVariableSizedType(nil, sema.UInt8Type)

var evmAddressBytesType = sema.NewConstantSizedType(nil, sema.UInt8Type, types.AddressLength)
var evmAddressBytesStaticType = interpreter.ConvertSemaArrayTypeToStaticArrayType(nil, evmAddressBytesType)
var EVMAddressBytesCadenceType = cadence.NewConstantSizedArrayType(types.AddressLength, cadence.UInt8Type)

const internalEVMTypeRunFunctionName = "run"

var internalEVMTypeRunFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          "tx",
			TypeAnnotation: sema.NewTypeAnnotation(evmTransactionBytesType),
		},
		{
			Label:          "coinbase",
			TypeAnnotation: sema.NewTypeAnnotation(evmAddressBytesType),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.BoolType),
}

func newInternalEVMTypeRunFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		internalEVMTypeRunFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			inter := invocation.Interpreter
			locationRange := invocation.LocationRange

			// Get transaction argument

			transactionValue, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			transaction, err := interpreter.ByteArrayValueToByteSlice(inter, transactionValue, locationRange)
			if err != nil {
				panic(err)
			}

			// Get coinbase argument

			coinbaseValue, ok := invocation.Arguments[1].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			coinbase, err := interpreter.ByteArrayValueToByteSlice(inter, coinbaseValue, locationRange)
			if err != nil {
				panic(err)
			}

			// Run

			cb := types.NewAddressFromBytes(coinbase)
			handler.Run(transaction, cb)

			return interpreter.Void
		},
	)
}

func EVMAddressToAddressBytesArrayValue(
	inter *interpreter.Interpreter,
	address types.Address,
) *interpreter.ArrayValue {
	var index int
	return interpreter.NewArrayValueWithIterator(
		inter,
		evmAddressBytesStaticType,
		common.ZeroAddress,
		types.AddressLength,
		func() interpreter.Value {
			if index >= types.AddressLength {
				return nil
			}
			result := interpreter.NewUInt8Value(inter, func() uint8 {
				return address[index]
			})
			index++
			return result
		},
	)
}

const internalEVMTypeCallFunctionName = "call"

var internalEVMTypeCallFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          "from",
			TypeAnnotation: sema.NewTypeAnnotation(evmAddressBytesType),
		},
		{
			Label:          "to",
			TypeAnnotation: sema.NewTypeAnnotation(evmAddressBytesType),
		},
		{
			Label:          "data",
			TypeAnnotation: sema.NewTypeAnnotation(sema.ByteArrayType),
		},
		{
			Label:          "gasLimit",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UInt64Type),
		},
		{
			Label:          "value",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UFix64Type),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.ByteArrayType),
}

func AddressBytesArrayValueToEVMAddress(
	inter *interpreter.Interpreter,
	locationRange interpreter.LocationRange,
	addressBytesValue *interpreter.ArrayValue,
) (
	result types.Address,
	err error,
) {
	// Convert

	var bytes []byte
	bytes, err = interpreter.ByteArrayValueToByteSlice(
		inter,
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

func newInternalEVMTypeCallFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		internalEVMTypeCallFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			inter := invocation.Interpreter
			locationRange := invocation.LocationRange

			// Get from address

			fromAddressValue, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			fromAddress, err := AddressBytesArrayValueToEVMAddress(inter, locationRange, fromAddressValue)
			if err != nil {
				panic(err)
			}

			// Get to address

			toAddressValue, ok := invocation.Arguments[1].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			toAddress, err := AddressBytesArrayValueToEVMAddress(inter, locationRange, toAddressValue)
			if err != nil {
				panic(err)
			}

			// Get data

			dataValue, ok := invocation.Arguments[2].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			data, err := interpreter.ByteArrayValueToByteSlice(inter, dataValue, locationRange)
			if err != nil {
				panic(err)
			}

			// Get gas limit

			gasLimitValue, ok := invocation.Arguments[3].(interpreter.UInt64Value)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			gasLimit := types.GasLimit(gasLimitValue)

			// Get balance

			balanceValue, ok := invocation.Arguments[4].(interpreter.UFix64Value)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			balance := types.Balance(balanceValue)

			// Call

			const isAuthorized = true
			account := handler.AccountByAddress(fromAddress, isAuthorized)
			result := account.Call(toAddress, data, gasLimit, balance)

			return interpreter.ByteSliceToByteArrayValue(inter, result)
		},
	)
}

const internalEVMTypeCreateBridgedAccountFunctionName = "createBridgedAccount"

var internalEVMTypeCreateBridgedAccountFunctionType = &sema.FunctionType{
	ReturnTypeAnnotation: sema.NewTypeAnnotation(evmAddressBytesType),
}

func newInternalEVMTypeCreateBridgedAccountFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		internalEVMTypeCreateBridgedAccountFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			inter := invocation.Interpreter
			address := handler.AllocateAddress()
			return EVMAddressToAddressBytesArrayValue(inter, address)
		},
	)
}

const internalEVMTypeDepositFunctionName = "deposit"

var internalEVMTypeDepositFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          "from",
			TypeAnnotation: sema.NewTypeAnnotation(sema.AnyResourceType),
		},
		{
			Label:          "to",
			TypeAnnotation: sema.NewTypeAnnotation(evmAddressBytesType),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.VoidType),
}

const fungibleTokenVaultTypeBalanceFieldName = "balance"

func newInternalEVMTypeDepositFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		internalEVMTypeCallFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			inter := invocation.Interpreter
			locationRange := invocation.LocationRange

			// Get from vault

			fromValue, ok := invocation.Arguments[0].(*interpreter.CompositeValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			amountValue, ok := fromValue.GetField(
				inter,
				locationRange,
				fungibleTokenVaultTypeBalanceFieldName,
			).(interpreter.UFix64Value)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			amount := types.Balance(amountValue)

			// Get to address

			toAddressValue, ok := invocation.Arguments[1].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			toAddress, err := AddressBytesArrayValueToEVMAddress(inter, locationRange, toAddressValue)
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

const internalEVMTypeBalanceFunctionName = "balance"

var internalEVMTypeBalanceFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          "address",
			TypeAnnotation: sema.NewTypeAnnotation(evmAddressBytesType),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.UFix64Type),
}

// newInternalEVMTypeBalanceFunction returns the Flow balance of the account
func newInternalEVMTypeBalanceFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		internalEVMTypeCallFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			inter := invocation.Interpreter
			locationRange := invocation.LocationRange

			addressValue, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			address, err := AddressBytesArrayValueToEVMAddress(inter, locationRange, addressValue)
			if err != nil {
				panic(err)
			}

			const isAuthorized = false
			account := handler.AccountByAddress(address, isAuthorized)

			return interpreter.UFix64Value(account.Balance())
		},
	)
}

const internalEVMTypeWithdrawFunctionName = "withdraw"

var internalEVMTypeWithdrawFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          "from",
			TypeAnnotation: sema.NewTypeAnnotation(evmAddressBytesType),
		},
		{
			Label:          "amount",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UFix64Type),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.AnyResourceType),
}

func newInternalEVMTypeWithdrawFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		internalEVMTypeCallFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			inter := invocation.Interpreter
			locationRange := invocation.LocationRange

			// Get from address

			fromAddressValue, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			fromAddress, err := AddressBytesArrayValueToEVMAddress(inter, locationRange, fromAddressValue)
			if err != nil {
				panic(err)
			}

			// Get amount

			amountValue, ok := invocation.Arguments[1].(interpreter.UFix64Value)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			amount := types.Balance(amountValue)

			// Withdraw

			const isAuthorized = true
			account := handler.AccountByAddress(fromAddress, isAuthorized)
			vault := account.Withdraw(amount)

			// TODO: improve: maybe call actual constructor
			return interpreter.NewCompositeValue(
				inter,
				locationRange,
				common.NewAddressLocation(gauge, handler.FlowTokenAddress(), "FlowToken"),
				"FlowToken.Vault",
				common.CompositeKindResource,
				[]interpreter.CompositeField{
					{
						Name: "balance",
						Value: interpreter.NewUFix64Value(gauge, func() uint64 {
							return uint64(vault.Balance())
						}),
					},
				},
				common.ZeroAddress,
			)
		},
	)
}

const internalEVMTypeDeployFunctionName = "deploy"

var internalEVMTypeDeployFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          "from",
			TypeAnnotation: sema.NewTypeAnnotation(evmAddressBytesType),
		},
		{
			Label:          "code",
			TypeAnnotation: sema.NewTypeAnnotation(sema.ByteArrayType),
		},
		{
			Label:          "gasLimit",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UInt64Type),
		},
		{
			Label:          "value",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UFix64Type),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(evmAddressBytesType),
}

func newInternalEVMTypeDeployFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		internalEVMTypeCallFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			inter := invocation.Interpreter
			locationRange := invocation.LocationRange

			// Get from address

			fromAddressValue, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			fromAddress, err := AddressBytesArrayValueToEVMAddress(inter, locationRange, fromAddressValue)
			if err != nil {
				panic(err)
			}

			// Get code

			codeValue, ok := invocation.Arguments[1].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			code, err := interpreter.ByteArrayValueToByteSlice(inter, codeValue, locationRange)
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

			amountValue, ok := invocation.Arguments[3].(interpreter.UFix64Value)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			amount := types.Balance(amountValue)

			// Deploy

			const isAuthorized = true
			account := handler.AccountByAddress(fromAddress, isAuthorized)
			address := account.Deploy(code, gasLimit, amount)

			return EVMAddressToAddressBytesArrayValue(inter, address)
		},
	)
}

func NewInternalEVMContractValue(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.SimpleCompositeValue {
	return interpreter.NewSimpleCompositeValue(
		gauge,
		InternalEVMContractType.ID(),
		internalEVMContractStaticType,
		InternalEVMContractType.Fields,
		map[string]interpreter.Value{
			internalEVMTypeRunFunctionName:                  newInternalEVMTypeRunFunction(gauge, handler),
			internalEVMTypeCreateBridgedAccountFunctionName: newInternalEVMTypeCreateBridgedAccountFunction(gauge, handler),
			internalEVMTypeCallFunctionName:                 newInternalEVMTypeCallFunction(gauge, handler),
			internalEVMTypeDepositFunctionName:              newInternalEVMTypeDepositFunction(gauge, handler),
			internalEVMTypeWithdrawFunctionName:             newInternalEVMTypeWithdrawFunction(gauge, handler),
			internalEVMTypeDeployFunctionName:               newInternalEVMTypeDeployFunction(gauge, handler),
			internalEVMTypeBalanceFunctionName:              newInternalEVMTypeBalanceFunction(gauge, handler),
		},
		nil,
		nil,
		nil,
	)
}

const InternalEVMContractName = "InternalEVM"

var InternalEVMContractType = func() *sema.CompositeType {
	ty := &sema.CompositeType{
		Identifier: InternalEVMContractName,
		Kind:       common.CompositeKindContract,
	}

	ty.Members = sema.MembersAsMap([]*sema.Member{
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			internalEVMTypeRunFunctionName,
			internalEVMTypeRunFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			internalEVMTypeCreateBridgedAccountFunctionName,
			internalEVMTypeCreateBridgedAccountFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			internalEVMTypeCallFunctionName,
			internalEVMTypeCallFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			internalEVMTypeDepositFunctionName,
			internalEVMTypeDepositFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			internalEVMTypeWithdrawFunctionName,
			internalEVMTypeWithdrawFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			internalEVMTypeDeployFunctionName,
			internalEVMTypeDeployFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			internalEVMTypeBalanceFunctionName,
			internalEVMTypeBalanceFunctionType,
			"",
		),
	})
	return ty
}()

var internalEVMContractStaticType = interpreter.ConvertSemaCompositeTypeToStaticCompositeType(
	nil,
	InternalEVMContractType,
)

func newInternalEVMStandardLibraryValue(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) stdlib.StandardLibraryValue {
	return stdlib.StandardLibraryValue{
		Name:  InternalEVMContractName,
		Type:  InternalEVMContractType,
		Value: NewInternalEVMContractValue(gauge, handler),
		Kind:  common.DeclarationKindContract,
	}
}

var internalEVMStandardLibraryType = stdlib.StandardLibraryType{
	Name: InternalEVMContractName,
	Type: InternalEVMContractType,
	Kind: common.DeclarationKindContract,
}

func SetupEnvironment(env runtime.Environment, handler types.ContractHandler, service flow.Address) {
	location := common.NewAddressLocation(nil, common.Address(service), ContractName)
	env.DeclareType(
		internalEVMStandardLibraryType,
		location,
	)
	env.DeclareValue(
		newInternalEVMStandardLibraryValue(nil, handler),
		location,
	)
}

func NewEVMAddressCadenceType(address common.Address) *cadence.StructType {
	return cadence.NewStructType(
		common.NewAddressLocation(nil, address, ContractName),
		"EVM.EVMAddress",
		[]cadence.Field{
			{
				Identifier: "bytes",
				Type:       EVMAddressBytesCadenceType,
			},
		},
		nil,
	)
}

func NewBalanceCadenceType(address common.Address) *cadence.StructType {
	return cadence.NewStructType(
		common.NewAddressLocation(nil, address, ContractName),
		"EVM.Balance",
		[]cadence.Field{
			{
				Identifier: "flow",
				Type:       cadence.UFix64Type,
			},
		},
		nil,
	)
}
