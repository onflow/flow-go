package stdlib

import (
	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/flex/models"
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
	_flex_FlexAddressConstructorType             *sema.FunctionType
	Flex_FlowOwnedAccountType                    *sema.CompositeType
	Flex_FlowOwnedAccountTypeAddressFunctionName string
	Flex_FlowOwnedAccountTypeAddressFunctionType *sema.FunctionType
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

func NewFlexTypeRunFunction(
	gauge common.MemoryGauge,
	def FlexTypeDefinition,
	handler models.FlexContractHandler,
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

			cb := models.FlexAddress(coinbase)
			res := handler.Run(transaction, cb)

			return interpreter.AsBoolValue(res)
		},
	)
}

var flexAddressArrayStaticType = interpreter.NewConstantSizedStaticType(
	nil,
	interpreter.PrimitiveStaticTypeUInt8,
	models.FlexAddressLength,
)

func FlexAddressToAddressBytesArrayValue(
	inter *interpreter.Interpreter,
	address models.FlexAddress,
) *interpreter.ArrayValue {
	var index int
	return interpreter.NewArrayValueWithIterator(
		inter,
		flexAddressArrayStaticType,
		common.ZeroAddress,
		models.FlexAddressLength,
		func() interpreter.Value {
			if index >= models.FlexAddressLength {
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

func NewFlexOwnedAccountTypeAddressFunction(
	gauge common.MemoryGauge,
	def FlexTypeDefinition,
	addressBytesValue *interpreter.ArrayValue,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		def.Flex_FlowOwnedAccountTypeAddressFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			inter := invocation.Interpreter
			locationRange := invocation.LocationRange

			// NOTE: important: provide a *copy*, so modifications are not reflected in storage
			addressBytesValue := addressBytesValue.Transfer(
				inter,
				locationRange,
				atree.Address{},
				false,
				nil,
				nil,
			)

			return interpreter.NewCompositeValue(
				inter,
				locationRange,
				nil,
				def.Flex_FlexAddressType.QualifiedIdentifier(),
				common.CompositeKindStructure,
				[]interpreter.CompositeField{
					{
						Name:  def.Flex_FlexAddressTypeBytesFieldName,
						Value: addressBytesValue,
					},
				},
				common.ZeroAddress,
			)
		},
	)
}

func NewFlexTypeCreateOwnedAccountFunction(
	gauge common.MemoryGauge,
	def FlexTypeDefinition,
	handler models.FlexContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		def.FlexTypeCreateFlowOwnedAccountFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			inter := invocation.Interpreter
			locationRange := invocation.LocationRange

			address := handler.AllocateAddress()

			addressBytesValue := FlexAddressToAddressBytesArrayValue(inter, address)

			// Construct and return Flex.FlowOwnedAccount

			return interpreter.NewCompositeValue(
				inter,
				locationRange,
				nil,
				def.Flex_FlowOwnedAccountType.QualifiedIdentifier(),
				common.CompositeKindResource,
				[]interpreter.CompositeField{
					{
						Value: addressBytesValue,
						Name:  Flex_FlowOwnedAccountTypeAddressBytesFieldName,
					},
					// TODO: inject properly as function
					{
						Name:  def.Flex_FlowOwnedAccountTypeAddressFunctionName,
						Value: NewFlexOwnedAccountTypeAddressFunction(gauge, def, addressBytesValue),
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
				def.FlexType.Location,
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

func NewFlexContractValue(
	gauge common.MemoryGauge,
	def FlexTypeDefinition,
	handler models.FlexContractHandler,
) *interpreter.SimpleCompositeValue {
	return interpreter.NewSimpleCompositeValue(
		gauge,
		def.FlexType.ID(),
		def.FlexStaticType(),
		def.FlexType.Fields,
		map[string]interpreter.Value{
			def.Flex_FlexAddressType.Identifier:            NewFlexAddressConstructor(def),
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
	handler models.FlexContractHandler,
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
