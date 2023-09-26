package stdlib

import (
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/fvm/flex/models"
)

// TODO: switch to released version once available
//go:generate env GOPROXY=direct go run github.com/onflow/cadence/runtime/sema/gen@1e04b7af1c098a3deff37931ef33191644606a89 -p stdlib flex.cdc flex.gen.go

var flexContractStaticType interpreter.StaticType = interpreter.NewCompositeStaticType(
	nil,
	FlexType.Location,
	FlexType.QualifiedIdentifier(),
	FlexType.ID(),
)

func newFlexTypeRunFunction(
	gauge common.MemoryGauge,
	handler models.FlexContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		FlexTypeRunFunctionType,
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

			coinbaseBytesValue := coinbaseValue.GetMember(inter, locationRange, Flex_FlexAddressTypeBytesFieldName)
			if coinbaseBytesValue == nil {
				panic(errors.NewUnreachableError())
			}

			coinbase, err := interpreter.ByteArrayValueToByteSlice(inter, coinbaseBytesValue, locationRange)
			if err != nil {
				panic(err)
			}

			// Run

			cb := models.FlexAddress(coinbase)
			res := handler.Run(transaction, &cb)

			return interpreter.AsBoolValue(res)
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

var Flex_FlexAddressConstructorType = constructorType(Flex_FlexAddressType)

var flexAddressConstructor = interpreter.NewHostFunctionValue(
	nil,
	Flex_FlexAddressConstructorType,
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
			FlexType.Location,
			Flex_FlexAddressType.QualifiedIdentifier(),
			Flex_FlexAddressType.Kind,
			[]interpreter.CompositeField{
				{
					Name:  Flex_FlexAddressTypeBytesFieldName,
					Value: bytesValue,
				},
			},
			common.ZeroAddress,
		)
	},
)

func NewFlexContractValue(
	gauge common.MemoryGauge,
	handler models.FlexContractHandler,
) *interpreter.SimpleCompositeValue {
	return interpreter.NewSimpleCompositeValue(
		gauge,
		FlexType.ID(),
		flexContractStaticType,
		FlexType.Fields,
		map[string]interpreter.Value{
			Flex_FlexAddressTypeName: flexAddressConstructor,
			FlexTypeRunFunctionName:  newFlexTypeRunFunction(gauge, handler),
		},
		nil,
		nil,
		nil,
	)
}

func NewFlexStandardLibraryValue(
	gauge common.MemoryGauge,
	handler models.FlexContractHandler,
) stdlib.StandardLibraryValue {
	return stdlib.StandardLibraryValue{
		Name:  FlexTypeName,
		Type:  FlexType,
		Value: NewFlexContractValue(gauge, handler),
		Kind:  common.DeclarationKindContract,
	}
}

var FlexStandardLibraryType = stdlib.StandardLibraryType{
	Name: FlexTypeName,
	Type: FlexType,
	Kind: common.DeclarationKindContract,
}
