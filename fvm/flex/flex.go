package flex

import (
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/stdlib"
)

const Flex_AddressTypeBytesFieldName = "bytes"

var Flex_AddressTypeBytesFieldType = &sema.ConstantSizedType{
	Type: sema.UInt8Type,
	Size: 20,
}

const Flex_AddressTypeBytesFieldDocString = `
Bytes of the address
`

const Flex_AddressTypeName = "Address"

var Flex_AddressType = func() *sema.CompositeType {
	var t = &sema.CompositeType{
		Identifier: Flex_AddressTypeName,
		Kind:       common.CompositeKindStructure,
	}

	return t
}()

func init() {
	var members = []*sema.Member{
		sema.NewUnmeteredFieldMember(
			Flex_AddressType,
			ast.AccessPublic,
			ast.VariableKindConstant,
			Flex_AddressTypeBytesFieldName,
			Flex_AddressTypeBytesFieldType,
			Flex_AddressTypeBytesFieldDocString,
		),
	}

	Flex_AddressType.Members = sema.MembersAsMap(members)
	Flex_AddressType.Fields = sema.MembersFieldNames(members)
}

const FlexTypeRunFunctionName = "run"

var FlexTypeRunFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Identifier: "tx",
			TypeAnnotation: sema.NewTypeAnnotation(&sema.VariableSizedType{
				Type: sema.UInt8Type,
			}),
		},
		{
			Identifier:     "coinbase",
			TypeAnnotation: sema.NewTypeAnnotation(Flex_AddressType),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		sema.VoidType,
	),
}

const FlexTypeRunFunctionDocString = `
Run runs a flex transaction, deducts the gas fees and deposits them into the
provided coinbase address
`

const FlexTypeName = "Flex"

var FlexType = func() *sema.CompositeType {
	var t = &sema.CompositeType{
		Identifier: FlexTypeName,
		Kind:       common.CompositeKindContract,
	}

	t.SetNestedType(Flex_AddressTypeName, Flex_AddressType)
	return t
}()

func init() {
	var members = []*sema.Member{
		sema.NewUnmeteredFunctionMember(
			FlexType,
			ast.AccessPublic,
			FlexTypeRunFunctionName,
			FlexTypeRunFunctionType,
			FlexTypeRunFunctionDocString,
		),
	}

	FlexType.Members = sema.MembersAsMap(members)
	FlexType.Fields = sema.MembersFieldNames(members)
}

var flexContractStaticType interpreter.StaticType = interpreter.NewCompositeStaticType(
	nil, // TODO deal with memory gage
	FlexType.Location,
	FlexType.QualifiedIdentifier(),
	FlexType.ID(),
)

func newFlexTypeRunFunction(
	gauge common.MemoryGauge,
	handler FlexContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		FlexTypeRunFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			// inter := invocation.Interpreter
			locationRange := invocation.LocationRange

			input, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			// TODO capture computation
			// invocation.Interpreter.ReportComputation(common.ComputationKindSTDLIBRLPDecodeString, uint(input.Count()))

			convertedInput, err := interpreter.ByteArrayValueToByteSlice(invocation.Interpreter, input, locationRange)
			if err != nil {
				// TODO deal wrap this with proper error
				panic(err)
				// panic(RLPDecodeStringError{
				// 	Msg:           err.Error(),
				// 	LocationRange: locationRange,
				// })
			}

			// TODO deal with the coinbase address
			res := handler.Run(convertedInput, FlexAddress(common.ZeroAddress[:]))

			return interpreter.AsBoolValue(res)
		},
	)
}

func NewFlexContractValue(
	gauge common.MemoryGauge,
	handler FlexContractHandler,
) *interpreter.SimpleCompositeValue {
	return interpreter.NewSimpleCompositeValue(
		gauge,
		FlexType.ID(),
		flexContractStaticType,
		FlexType.Fields,
		map[string]interpreter.Value{
			FlexTypeRunFunctionName: newFlexTypeRunFunction(gauge, handler),
		},
		nil,
		nil,
		nil,
	)
}

func NewFlexStandardLibraryValue(
	gauge common.MemoryGauge,
	handler FlexContractHandler,
) *stdlib.StandardLibraryValue {
	return &stdlib.StandardLibraryValue{
		Name:  FlexTypeName,
		Type:  FlexType,
		Value: NewFlexContractValue(gauge, handler),
		Kind:  common.DeclarationKindContract,
	}
}
