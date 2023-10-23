package stdlib

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

const Flex_FlowOwnedAccountTypeAddressBytesFieldName = "addressBytes"

func NewFlowTokenVaultType(chain flow.Chain) *sema.CompositeType {
	const flowTokenContractName = "FlowToken"
	const vaultTypeName = "Vault"

	var flowTokenAddress, err = chain.AddressAtIndex(environment.FlowTokenAccountIndex)
	if err != nil {
		panic(err)
	}

	location := common.NewAddressLocation(
		nil,
		common.Address(flowTokenAddress),
		flowTokenContractName,
	)

	// TODO: replace with proper type, e.g. extracted from checker

	flowTokenType := &sema.CompositeType{
		Location:   location,
		Identifier: flowTokenContractName,
		Kind:       common.CompositeKindContract,
	}

	vaultType := &sema.CompositeType{
		Location:   location,
		Identifier: vaultTypeName,
		Kind:       common.CompositeKindResource,
	}

	flowTokenType.SetNestedType(vaultTypeName, vaultType)

	return vaultType
}

type FlexTypeDefinition struct {
	FlexType *sema.CompositeType
	// Deprecated: Use FlexStaticType()
	_flexStaticType                            interpreter.StaticType
	FlexTypeRunFunctionName                    string
	FlexTypeRunFunctionType                    *sema.FunctionType
	FlexTypeCreateFlowOwnedAccountFunctionName string
	FlexTypeCreateFlowOwnedAccountFunctionType *sema.FunctionType
	Flex_FlexAddressType                       *sema.CompositeType
	Flex_FlexAddressTypeBytesFieldName         string
	// Deprecated: Use Flex_FlexAddressConstructorType
	_flex_FlexAddressConstructorType *sema.FunctionType
	Flex_BalanceType                 *sema.CompositeType
	Flex_BalanceTypeFlowFieldName    string
	// Deprecated: Use Flex_BalanceConstructorType
	_flex_BalanceConstructorType                 *sema.FunctionType
	Flex_FlowOwnedAccountType                    *sema.CompositeType
	Flex_FlowOwnedAccountTypeAddressFunctionName string
	Flex_FlowOwnedAccountTypeAddressFunctionType *sema.FunctionType
	Flex_FlowOwnedAccountTypeCallFunctionName    string
	Flex_FlowOwnedAccountTypeCallFunctionType    *sema.FunctionType
}

func (t *FlexTypeDefinition) FlexStaticType() interpreter.StaticType {
	if t._flexStaticType == nil {
		flexType := t.FlexType
		t._flexStaticType = interpreter.NewCompositeStaticType(
			nil,
			flexType.Location,
			flexType.QualifiedIdentifier(),
			flexType.ID(),
		)
	}
	return t._flexStaticType
}

func (t *FlexTypeDefinition) Flex_FlexAddressConstructorType() *sema.FunctionType {
	if t._flex_FlexAddressConstructorType == nil {
		t._flex_FlexAddressConstructorType = constructorType(t.Flex_FlexAddressType)
	}
	return t._flex_FlexAddressConstructorType
}

func (t *FlexTypeDefinition) Flex_BalanceConstructorType() *sema.FunctionType {
	if t._flex_BalanceConstructorType == nil {
		t._flex_BalanceConstructorType = constructorType(t.Flex_BalanceType)
	}
	return t._flex_BalanceConstructorType
}

func NewFlexTypeRunFunction(
	gauge common.MemoryGauge,
	def FlexTypeDefinition,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		def.FlexTypeRunFunctionType,
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

			coinbaseValue, ok := invocation.Arguments[1].(interpreter.MemberAccessibleValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			coinbaseBytesValue := coinbaseValue.GetMember(inter, locationRange, def.Flex_FlexAddressTypeBytesFieldName)
			if coinbaseBytesValue == nil {
				panic(errors.NewUnreachableError())
			}

			coinbase, err := interpreter.ByteArrayValueToByteSlice(inter, coinbaseBytesValue, locationRange)
			if err != nil {
				panic(err)
			}

			// Run

			cb := types.Address(coinbase)
			res := handler.Run(transaction, cb)

			return interpreter.AsBoolValue(res)
		},
	)
}

var flexAddressArrayStaticType = interpreter.NewConstantSizedStaticType(
	nil,
	interpreter.PrimitiveStaticTypeUInt8,
	types.AddressLength,
)

func FlexAddressToAddressBytesArrayValue(
	inter *interpreter.Interpreter,
	address types.Address,
) *interpreter.ArrayValue {
	var index int
	return interpreter.NewArrayValueWithIterator(
		inter,
		flexAddressArrayStaticType,
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

func AddressBytesArrayValueToFlexAddress(
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

func NewFlexOwnedAccountTypeAddressFunction(
	gauge common.MemoryGauge,
	def FlexTypeDefinition,
	address types.Address,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		def.Flex_FlowOwnedAccountTypeAddressFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			inter := invocation.Interpreter
			locationRange := invocation.LocationRange

			// NOTE: important: create a new value
			bytesValue := FlexAddressToAddressBytesArrayValue(inter, address)

			return interpreter.NewCompositeValue(
				inter,
				locationRange,
				def.Flex_FlexAddressType.Location,
				def.Flex_FlexAddressType.QualifiedIdentifier(),
				def.Flex_FlexAddressType.Kind,
				[]interpreter.CompositeField{
					{
						Name:  def.Flex_FlexAddressTypeBytesFieldName,
						Value: bytesValue,
					},
				},
				common.ZeroAddress,
			)
		},
	)
}

func NewFlexOwnedAccountTypeCallFunction(
	gauge common.MemoryGauge,
	def FlexTypeDefinition,
	handler types.ContractHandler,
	flowOwnedAccountAddress types.Address,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		def.Flex_FlowOwnedAccountTypeCallFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			inter := invocation.Interpreter
			locationRange := invocation.LocationRange

			// Get address

			addressValue, ok := invocation.Arguments[0].(*interpreter.CompositeValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			addressBytesValue, ok := addressValue.GetField(
				inter,
				locationRange,
				def.Flex_FlexAddressTypeBytesFieldName,
			).(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			address, err := AddressBytesArrayValueToFlexAddress(inter, locationRange, addressBytesValue)
			if err != nil {
				panic(err)
			}

			// Get data

			dataValue, ok := invocation.Arguments[1].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			data, err := interpreter.ByteArrayValueToByteSlice(inter, dataValue, locationRange)
			if err != nil {
				panic(err)
			}

			// Get gas limit

			gasLimitValue, ok := invocation.Arguments[2].(interpreter.UInt64Value)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			gasLimit := types.GasLimit(gasLimitValue)

			// Get balance

			balanceValue, ok := invocation.Arguments[3].(*interpreter.CompositeValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			balanceFlowValue, ok := balanceValue.GetField(
				inter,
				locationRange,
				def.Flex_BalanceTypeFlowFieldName,
			).(interpreter.UFix64Value)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			balance := types.Balance(balanceFlowValue)

			// Call

			const isFOA = true
			account := handler.AccountByAddress(flowOwnedAccountAddress, isFOA)
			result := account.Call(address, data, gasLimit, balance)

			return interpreter.ByteSliceToByteArrayValue(inter, result)
		},
	)
}

func NewFlexTypeCreateOwnedAccountFunction(
	gauge common.MemoryGauge,
	def FlexTypeDefinition,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		def.FlexTypeCreateFlowOwnedAccountFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			inter := invocation.Interpreter
			locationRange := invocation.LocationRange

			address := handler.AllocateAddress()

			// Construct and return Flex.FlowOwnedAccount

			return interpreter.NewCompositeValue(
				inter,
				locationRange,
				def.Flex_FlowOwnedAccountType.Location,
				def.Flex_FlowOwnedAccountType.QualifiedIdentifier(),
				def.Flex_FlowOwnedAccountType.Kind,
				[]interpreter.CompositeField{
					{
						Value: FlexAddressToAddressBytesArrayValue(inter, address),
						Name:  Flex_FlowOwnedAccountTypeAddressBytesFieldName,
					},
					// TODO: inject properly as function
					{
						Name:  def.Flex_FlowOwnedAccountTypeAddressFunctionName,
						Value: NewFlexOwnedAccountTypeAddressFunction(gauge, def, address),
					},
					{
						Name:  def.Flex_FlowOwnedAccountTypeCallFunctionName,
						Value: NewFlexOwnedAccountTypeCallFunction(gauge, def, handler, address),
					},
					// TODO: inject other functions
				},
				common.ZeroAddress,
			)
		},
	)
}

func constructorType(compositeType *sema.CompositeType) *sema.FunctionType {
	// TODO: Use t.ConstructorType() at call-sites, once available.
	//   Depends on Stable Cadence / https://github.com/onflow/cadence/pull/2805
	return &sema.FunctionType{
		IsConstructor:        true,
		Parameters:           compositeType.ConstructorParameters,
		ReturnTypeAnnotation: sema.NewTypeAnnotation(compositeType),
	}
}

func NewFlexAddressConstructor(def FlexTypeDefinition) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		nil,
		def.Flex_FlexAddressConstructorType(),
		func(invocation interpreter.Invocation) interpreter.Value {
			inter := invocation.Interpreter
			locationRange := invocation.LocationRange

			// Get address

			bytesValue, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			return interpreter.NewCompositeValue(
				inter,
				locationRange,
				def.Flex_FlexAddressType.Location,
				def.Flex_FlexAddressType.QualifiedIdentifier(),
				def.Flex_FlexAddressType.Kind,
				[]interpreter.CompositeField{
					{
						Name:  def.Flex_FlexAddressTypeBytesFieldName,
						Value: bytesValue,
					},
				},
				common.ZeroAddress,
			)
		},
	)
}

func NewBalanceConstructor(def FlexTypeDefinition) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		nil,
		def.Flex_BalanceConstructorType(),
		func(invocation interpreter.Invocation) interpreter.Value {
			inter := invocation.Interpreter
			locationRange := invocation.LocationRange

			// Get amount

			flowValue, ok := invocation.Arguments[0].(interpreter.UFix64Value)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			return interpreter.NewCompositeValue(
				inter,
				locationRange,
				def.Flex_BalanceType.Location,
				def.Flex_BalanceType.QualifiedIdentifier(),
				def.Flex_BalanceType.Kind,
				[]interpreter.CompositeField{
					{
						Name:  def.Flex_BalanceTypeFlowFieldName,
						Value: flowValue,
					},
				},
				common.ZeroAddress,
			)
		},
	)
}

func NewFlexContractValue(
	gauge common.MemoryGauge,
	def FlexTypeDefinition,
	handler types.ContractHandler,
) *interpreter.SimpleCompositeValue {
	return interpreter.NewSimpleCompositeValue(
		gauge,
		def.FlexType.ID(),
		def.FlexStaticType(),
		def.FlexType.Fields,
		map[string]interpreter.Value{
			def.Flex_FlexAddressType.Identifier:            NewFlexAddressConstructor(def),
			def.Flex_BalanceType.Identifier:                NewBalanceConstructor(def),
			def.FlexTypeRunFunctionName:                    NewFlexTypeRunFunction(gauge, def, handler),
			def.FlexTypeCreateFlowOwnedAccountFunctionName: NewFlexTypeCreateOwnedAccountFunction(gauge, def, handler),
		},
		nil,
		nil,
		nil,
	)
}

func NewFlexStandardLibraryValue(
	gauge common.MemoryGauge,
	def FlexTypeDefinition,
	handler types.ContractHandler,
) stdlib.StandardLibraryValue {
	return stdlib.StandardLibraryValue{
		Name:  def.FlexType.Identifier,
		Type:  def.FlexType,
		Value: NewFlexContractValue(gauge, def, handler),
		Kind:  common.DeclarationKindContract,
	}
}

func NewFlexStandardLibraryType(def FlexTypeDefinition) stdlib.StandardLibraryType {
	return stdlib.StandardLibraryType{
		Name: def.FlexType.Identifier,
		Type: def.FlexType,
		Kind: common.DeclarationKindContract,
	}
}
