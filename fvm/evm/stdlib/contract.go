package stdlib

import (
	_ "embed"
	"fmt"
	"math/big"
	"regexp"

	gethABI "github.com/ethereum/go-ethereum/accounts/abi"
	gethCommon "github.com/ethereum/go-ethereum/common"
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
const evmAddressTypeBytesFieldName = "bytes"
const evmAddressTypeStructName = "EVMAddress"

var EVMTransactionBytesCadenceType = cadence.NewVariableSizedArrayType(cadence.TheUInt8Type)
var evmTransactionBytesType = sema.NewVariableSizedType(nil, sema.UInt8Type)

var evmAddressBytesType = sema.NewConstantSizedType(nil, sema.UInt8Type, types.AddressLength)
var evmAddressBytesStaticType = interpreter.ConvertSemaArrayTypeToStaticArrayType(nil, evmAddressBytesType)
var EVMAddressBytesCadenceType = cadence.NewConstantSizedArrayType(types.AddressLength, cadence.TheUInt8Type)

// unknownABITypeError

type unknownABITypeError struct {
	TypeName string
}

var _ errors.UserError = unknownABITypeError{}

func (unknownABITypeError) IsUserError() {}

func (e unknownABITypeError) Error() string {
	return fmt.Sprintf("unknown ABI type: %s", e.TypeName)
}

// unsupportedValueABIEncodingError

type unsupportedValueABIEncodingError struct {
	Type    interpreter.StaticType
	Message string
}

var _ errors.UserError = unsupportedValueABIEncodingError{}

func (unsupportedValueABIEncodingError) IsUserError() {}

func (e unsupportedValueABIEncodingError) Error() string {
	return fmt.Sprintf("unsupported ABI encoding for value of type: %v", e.Type)
}

// unsupportedValueABIDecodingError

type unsupportedValueABIDecodingError struct {
	Type    interpreter.StaticType
	Message string
}

var _ errors.UserError = unsupportedValueABIDecodingError{}

func (unsupportedValueABIDecodingError) IsUserError() {}

func (e unsupportedValueABIDecodingError) Error() string {
	return fmt.Sprintf("unsupported ABI decoding for value of type: %v", e.Type)
}

// abiEncodingError

type abiEncodingError struct {
	Message string
}

var _ errors.UserError = abiEncodingError{}

func (abiEncodingError) IsUserError() {}

func (e abiEncodingError) Error() string {
	return fmt.Sprintf("encoding of values to ABI failed with: %v", e.Message)
}

// abiDecodingError

type abiDecodingError struct {
	Message string
}

var _ errors.UserError = abiDecodingError{}

func (abiDecodingError) IsUserError() {}

func (e abiDecodingError) Error() string {
	return fmt.Sprintf("decoding of ABI to values failed with: %v", e.Message)
}

func newGethArgument(typeName string) gethABI.Argument {
	typ, err := gethABI.NewType(typeName, "", nil)
	if err != nil {
		panic(unknownABITypeError{TypeName: typeName})
	}
	return gethABI.Argument{Type: typ}
}

func evmAddressTypeID(location common.AddressLocation) common.TypeID {
	return common.TypeID(
		fmt.Sprintf("A.%v.%v", location, evmAddressTypeStructName),
	)
}

// EVM.encodeABI

const internalEVMTypeEncodeABIFunctionName = "encodeABI"

var internalEVMTypeEncodeABIFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:      sema.ArgumentLabelNotRequired,
			Identifier: "values",
			TypeAnnotation: sema.NewTypeAnnotation(
				sema.NewVariableSizedType(nil, sema.AnyStructType),
			),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.ByteArrayType),
}

func newInternalEVMTypeEncodeABIFunction(
	gauge common.MemoryGauge,
	location common.AddressLocation,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		internalEVMTypeEncodeABIFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			inter := invocation.Interpreter
			locationRange := invocation.LocationRange

			// Get `values` argument

			valuesArray, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			values := make([]interface{}, 0)
			var arguments gethABI.Arguments

			valuesArray.Iterate(inter, func(element interpreter.Value) (resume bool) {
				switch value := element.(type) {
				case *interpreter.StringValue:
					values = append(values, value.Str)
					arguments = append(arguments, newGethArgument("string"))
				case interpreter.BoolValue:
					values = append(values, bool(value))
					arguments = append(arguments, newGethArgument("bool"))
				case interpreter.UInt8Value:
					values = append(values, uint8(value))
					arguments = append(arguments, newGethArgument("uint8"))
				case interpreter.UInt16Value:
					values = append(values, uint16(value))
					arguments = append(arguments, newGethArgument("uint16"))
				case interpreter.UInt32Value:
					values = append(values, uint32(value))
					arguments = append(arguments, newGethArgument("uint32"))
				case interpreter.UInt64Value:
					values = append(values, uint64(value))
					arguments = append(arguments, newGethArgument("uint64"))
				case interpreter.UInt128Value:
					values = append(values, value.BigInt)
					arguments = append(arguments, newGethArgument("uint128"))
				case interpreter.UInt256Value:
					values = append(values, value.BigInt)
					arguments = append(arguments, newGethArgument("uint256"))
				case interpreter.Int8Value:
					values = append(values, int8(value))
					arguments = append(arguments, newGethArgument("int8"))
				case interpreter.Int16Value:
					values = append(values, int16(value))
					arguments = append(arguments, newGethArgument("int16"))
				case interpreter.Int32Value:
					values = append(values, int32(value))
					arguments = append(arguments, newGethArgument("int32"))
				case interpreter.Int64Value:
					values = append(values, int64(value))
					arguments = append(arguments, newGethArgument("int64"))
				case interpreter.Int128Value:
					values = append(values, value.BigInt)
					arguments = append(arguments, newGethArgument("int128"))
				case interpreter.Int256Value:
					values = append(values, value.BigInt)
					arguments = append(arguments, newGethArgument("int256"))
				case *interpreter.CompositeValue:
					if value.TypeID() == evmAddressTypeID(location) {
						bytes, err := interpreter.ByteArrayValueToByteSlice(
							inter,
							value.GetMember(inter, locationRange, evmAddressTypeBytesFieldName),
							locationRange,
						)
						if err != nil {
							panic(err)
						}
						values = append(values, gethCommon.Address(bytes))
						arguments = append(arguments, newGethArgument("address"))
					} else {
						panic(
							unsupportedValueABIEncodingError{
								Type: element.StaticType(inter),
							},
						)
					}
				case *interpreter.ArrayValue:
					switch value.Type.ElementType() {
					case interpreter.PrimitiveStaticTypeString:
						elements := make([]string, 0)
						value.Iterate(inter, func(element interpreter.Value) (resume bool) {
							v, ok := element.(*interpreter.StringValue)
							if !ok {
								panic(
									unsupportedValueABIEncodingError{
										Type: element.StaticType(inter),
									},
								)
							}
							elements = append(elements, v.Str)

							// continue iteration
							return true
						})
						values = append(values, elements)
						arguments = append(arguments, newGethArgument("string[]"))
					case interpreter.PrimitiveStaticTypeBool:
						elements := make([]bool, 0)
						value.Iterate(inter, func(element interpreter.Value) (resume bool) {
							v, ok := element.(interpreter.BoolValue)
							if !ok {
								panic(
									unsupportedValueABIEncodingError{
										Type: element.StaticType(inter),
									},
								)
							}
							elements = append(elements, bool(v))

							// continue iteration
							return true
						})
						values = append(values, elements)
						arguments = append(arguments, newGethArgument("bool[]"))
					case interpreter.PrimitiveStaticTypeUInt8:
						elements := make([]uint8, 0)
						value.Iterate(inter, func(element interpreter.Value) (resume bool) {
							v, ok := element.(interpreter.UInt8Value)
							if !ok {
								panic(
									unsupportedValueABIEncodingError{
										Type: element.StaticType(inter),
									},
								)
							}
							elements = append(elements, uint8(v))

							// continue iteration
							return true
						})
						values = append(values, elements)
						arguments = append(arguments, newGethArgument("uint8[]"))
					case interpreter.PrimitiveStaticTypeUInt16:
						elements := make([]uint16, 0)
						value.Iterate(inter, func(element interpreter.Value) (resume bool) {
							v, ok := element.(interpreter.UInt16Value)
							if !ok {
								panic(
									unsupportedValueABIEncodingError{
										Type: element.StaticType(inter),
									},
								)
							}
							elements = append(elements, uint16(v))

							// continue iteration
							return true
						})
						values = append(values, elements)
						arguments = append(arguments, newGethArgument("uint16[]"))
					case interpreter.PrimitiveStaticTypeUInt32:
						elements := make([]uint32, 0)
						value.Iterate(inter, func(element interpreter.Value) (resume bool) {
							v, ok := element.(interpreter.UInt32Value)
							if !ok {
								panic(
									unsupportedValueABIEncodingError{
										Type: element.StaticType(inter),
									},
								)
							}
							elements = append(elements, uint32(v))

							// continue iteration
							return true
						})
						values = append(values, elements)
						arguments = append(arguments, newGethArgument("uint32[]"))
					case interpreter.PrimitiveStaticTypeUInt64:
						elements := make([]uint64, 0)
						value.Iterate(inter, func(element interpreter.Value) (resume bool) {
							v, ok := element.(interpreter.UInt64Value)
							if !ok {
								panic(
									unsupportedValueABIEncodingError{
										Type: element.StaticType(inter),
									},
								)
							}
							elements = append(elements, uint64(v))

							// continue iteration
							return true
						})
						values = append(values, elements)
						arguments = append(arguments, newGethArgument("uint64[]"))
					case interpreter.PrimitiveStaticTypeUInt128:
						elements := make([]*big.Int, 0)
						value.Iterate(inter, func(element interpreter.Value) (resume bool) {
							v, ok := element.(interpreter.UInt128Value)
							if !ok {
								panic(
									unsupportedValueABIEncodingError{
										Type: element.StaticType(inter),
									},
								)
							}
							elements = append(elements, v.BigInt)

							// continue iteration
							return true
						})
						values = append(values, elements)
						arguments = append(arguments, newGethArgument("uint128[]"))
					case interpreter.PrimitiveStaticTypeUInt256:
						elements := make([]*big.Int, 0)
						value.Iterate(inter, func(element interpreter.Value) (resume bool) {
							v, ok := element.(interpreter.UInt256Value)
							if !ok {
								panic(
									unsupportedValueABIEncodingError{
										Type: element.StaticType(inter),
									},
								)
							}
							elements = append(elements, v.BigInt)

							// continue iteration
							return true
						})
						values = append(values, elements)
						arguments = append(arguments, newGethArgument("uint256[]"))
					case interpreter.PrimitiveStaticTypeInt8:
						elements := make([]int8, 0)
						value.Iterate(inter, func(element interpreter.Value) (resume bool) {
							v, ok := element.(interpreter.Int8Value)
							if !ok {
								panic(
									unsupportedValueABIEncodingError{
										Type: element.StaticType(inter),
									},
								)
							}
							elements = append(elements, int8(v))

							// continue iteration
							return true
						})
						values = append(values, elements)
						arguments = append(arguments, newGethArgument("int8[]"))
					case interpreter.PrimitiveStaticTypeInt16:
						elements := make([]int16, 0)
						value.Iterate(inter, func(element interpreter.Value) (resume bool) {
							v, ok := element.(interpreter.Int16Value)
							if !ok {
								panic(
									unsupportedValueABIEncodingError{
										Type: element.StaticType(inter),
									},
								)
							}
							elements = append(elements, int16(v))

							// continue iteration
							return true
						})
						values = append(values, elements)
						arguments = append(arguments, newGethArgument("int16[]"))
					case interpreter.PrimitiveStaticTypeInt32:
						elements := make([]int32, 0)
						value.Iterate(inter, func(element interpreter.Value) (resume bool) {
							v, ok := element.(interpreter.Int32Value)
							if !ok {
								panic(
									unsupportedValueABIEncodingError{
										Type: element.StaticType(inter),
									},
								)
							}
							elements = append(elements, int32(v))

							// continue iteration
							return true
						})
						values = append(values, elements)
						arguments = append(arguments, newGethArgument("int32[]"))
					case interpreter.PrimitiveStaticTypeInt64:
						elements := make([]int64, 0)
						value.Iterate(inter, func(element interpreter.Value) (resume bool) {
							v, ok := element.(interpreter.Int64Value)
							if !ok {
								panic(
									unsupportedValueABIEncodingError{
										Type: element.StaticType(inter),
									},
								)
							}
							elements = append(elements, int64(v))

							// continue iteration
							return true
						})
						values = append(values, elements)
						arguments = append(arguments, newGethArgument("int64[]"))
					case interpreter.PrimitiveStaticTypeInt128:
						elements := make([]*big.Int, 0)
						value.Iterate(inter, func(element interpreter.Value) (resume bool) {
							v, ok := element.(interpreter.Int128Value)
							if !ok {
								panic(
									unsupportedValueABIEncodingError{
										Type: element.StaticType(inter),
									},
								)
							}
							elements = append(elements, v.BigInt)

							// continue iteration
							return true
						})
						values = append(values, elements)
						arguments = append(arguments, newGethArgument("int128[]"))
					case interpreter.PrimitiveStaticTypeInt256:
						elements := make([]*big.Int, 0)
						value.Iterate(inter, func(element interpreter.Value) (resume bool) {
							v, ok := element.(interpreter.Int256Value)
							if !ok {
								panic(
									unsupportedValueABIEncodingError{
										Type: element.StaticType(inter),
									},
								)
							}
							elements = append(elements, v.BigInt)

							// continue iteration
							return true
						})
						values = append(values, elements)
						arguments = append(arguments, newGethArgument("int256[]"))
					default:
						panic(
							unsupportedValueABIEncodingError{
								Type: element.StaticType(inter),
							},
						)
					}
				default:
					panic(
						unsupportedValueABIEncodingError{
							Type: element.StaticType(inter),
						},
					)
				}

				// continue iteration
				return true
			})

			encodedValues, err := arguments.Pack(values...)
			if err != nil {
				panic(
					abiEncodingError{Message: err.Error()},
				)
			}

			return interpreter.ByteSliceToByteArrayValue(inter, encodedValues)
		},
	)
}

// EVM.decodeABI

const internalEVMTypeDecodeABIFunctionName = "decodeABI"

var internalEVMTypeDecodeABIFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Identifier: "types",
			TypeAnnotation: sema.NewTypeAnnotation(
				sema.NewVariableSizedType(nil, sema.MetaType),
			),
		},
		{
			Label:          "data",
			TypeAnnotation: sema.NewTypeAnnotation(sema.ByteArrayType),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(
		sema.NewVariableSizedType(nil, sema.AnyStructType),
	),
}

func newInternalEVMTypeDecodeABIFunction(
	gauge common.MemoryGauge,
	location common.AddressLocation,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		internalEVMTypeDecodeABIFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			inter := invocation.Interpreter
			locationRange := invocation.LocationRange

			// Get `types` argument

			typesArray, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			// Get `data` argument

			dataValue, ok := invocation.Arguments[1].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			data, err := interpreter.ByteArrayValueToByteSlice(inter, dataValue, locationRange)
			if err != nil {
				panic(err)
			}

			var arguments gethABI.Arguments
			typesArray.Iterate(inter, func(element interpreter.Value) (resume bool) {
				typeValue, ok := element.(interpreter.TypeValue)
				if !ok {
					panic(errors.NewUnreachableError())
				}

				supportedValue := true

				switch value := typeValue.Type.(type) {
				case interpreter.ArrayStaticType:
					switch value.ElementType() {
					case interpreter.PrimitiveStaticTypeString:
						arguments = append(arguments, newGethArgument("string[]"))
					case interpreter.PrimitiveStaticTypeBool:
						arguments = append(arguments, newGethArgument("bool[]"))
					case interpreter.PrimitiveStaticTypeUInt8:
						arguments = append(arguments, newGethArgument("uint8[]"))
					case interpreter.PrimitiveStaticTypeUInt16:
						arguments = append(arguments, newGethArgument("uint16[]"))
					case interpreter.PrimitiveStaticTypeUInt32:
						arguments = append(arguments, newGethArgument("uint32[]"))
					case interpreter.PrimitiveStaticTypeUInt64:
						arguments = append(arguments, newGethArgument("uint64[]"))
					case interpreter.PrimitiveStaticTypeUInt128:
						arguments = append(arguments, newGethArgument("uint128[]"))
					case interpreter.PrimitiveStaticTypeUInt256:
						arguments = append(arguments, newGethArgument("uint256[]"))
					case interpreter.PrimitiveStaticTypeInt8:
						arguments = append(arguments, newGethArgument("int8[]"))
					case interpreter.PrimitiveStaticTypeInt16:
						arguments = append(arguments, newGethArgument("int16[]"))
					case interpreter.PrimitiveStaticTypeInt32:
						arguments = append(arguments, newGethArgument("int32[]"))
					case interpreter.PrimitiveStaticTypeInt64:
						arguments = append(arguments, newGethArgument("int64[]"))
					case interpreter.PrimitiveStaticTypeInt128:
						arguments = append(arguments, newGethArgument("int128[]"))
					case interpreter.PrimitiveStaticTypeInt256:
						arguments = append(arguments, newGethArgument("int256[]"))
					default:
						panic(
							unsupportedValueABIDecodingError{
								Type: value.ElementType(),
							},
						)
					}
				case interpreter.CompositeStaticType:
					if value.TypeID == evmAddressTypeID(location) {
						arguments = append(arguments, newGethArgument("address"))
					} else {
						panic(
							unsupportedValueABIDecodingError{
								Type: typeValue.Type,
							},
						)
					}
				default:
					supportedValue = false
				}

				if supportedValue {
					// continue iteration
					return true
				}

				switch typeValue.Type {
				case interpreter.PrimitiveStaticTypeString:
					arguments = append(arguments, newGethArgument("string"))
				case interpreter.PrimitiveStaticTypeBool:
					arguments = append(arguments, newGethArgument("bool"))
				case interpreter.PrimitiveStaticTypeUInt8:
					arguments = append(arguments, newGethArgument("uint8"))
				case interpreter.PrimitiveStaticTypeUInt16:
					arguments = append(arguments, newGethArgument("uint16"))
				case interpreter.PrimitiveStaticTypeUInt32:
					arguments = append(arguments, newGethArgument("uint32"))
				case interpreter.PrimitiveStaticTypeUInt64:
					arguments = append(arguments, newGethArgument("uint64"))
				case interpreter.PrimitiveStaticTypeUInt128:
					arguments = append(arguments, newGethArgument("uint128"))
				case interpreter.PrimitiveStaticTypeUInt256:
					arguments = append(arguments, newGethArgument("uint256"))
				case interpreter.PrimitiveStaticTypeInt8:
					arguments = append(arguments, newGethArgument("int8"))
				case interpreter.PrimitiveStaticTypeInt16:
					arguments = append(arguments, newGethArgument("int16"))
				case interpreter.PrimitiveStaticTypeInt32:
					arguments = append(arguments, newGethArgument("int32"))
				case interpreter.PrimitiveStaticTypeInt64:
					arguments = append(arguments, newGethArgument("int64"))
				case interpreter.PrimitiveStaticTypeInt128:
					arguments = append(arguments, newGethArgument("int128"))
				case interpreter.PrimitiveStaticTypeInt256:
					arguments = append(arguments, newGethArgument("int256"))
				default:
					panic(
						unsupportedValueABIDecodingError{
							Type: typeValue.Type,
						},
					)
				}

				// continue iteration
				return true
			})

			decodedValues, err := arguments.Unpack(data)
			if err != nil {
				panic(
					abiDecodingError{Message: err.Error()},
				)
			}

			if len(decodedValues) != typesArray.Count() {
				message := fmt.Sprintf(
					"received %d types for %d values",
					typesArray.Count(),
					len(decodedValues),
				)
				panic(
					abiDecodingError{Message: message},
				)
			}

			i := 0
			values := make([]interpreter.Value, 0)
			typesArray.Iterate(inter, func(element interpreter.Value) (resume bool) {
				typeValue, ok := element.(interpreter.TypeValue)
				if !ok {
					panic(errors.NewUnreachableError())
				}

				switch value := typeValue.Type.(type) {
				case interpreter.ArrayStaticType:
					var arrayElementType interpreter.PrimitiveStaticType
					arrValues := make([]interpreter.Value, 0)

					switch value.ElementType() {
					case interpreter.PrimitiveStaticTypeString:
						arrayElementType = interpreter.PrimitiveStaticTypeString
						elements, ok := decodedValues[i].([]string)
						if !ok {
							panic(abiDecodingError{Message: "could not decode ABI value to [String]"})
						}
						for _, v := range elements {
							arrValues = append(arrValues, interpreter.NewStringValue(inter, common.NewStringMemoryUsage(len(v)), func() string {
								return v
							}))
						}
					case interpreter.PrimitiveStaticTypeBool:
						arrayElementType = interpreter.PrimitiveStaticTypeBool
						elements, ok := decodedValues[i].([]bool)
						if !ok {
							panic(abiDecodingError{Message: "could not decode ABI value to [Bool]"})
						}
						for _, v := range elements {
							arrValues = append(arrValues, interpreter.BoolValue(v))
						}
					case interpreter.PrimitiveStaticTypeUInt8:
						arrayElementType = interpreter.PrimitiveStaticTypeUInt8
						elements, ok := decodedValues[i].([]uint8)
						if !ok {
							panic(abiDecodingError{Message: "could not decode ABI value to [UInt8]"})
						}
						for _, v := range elements {
							arrValues = append(arrValues, interpreter.NewUInt8Value(inter, func() uint8 {
								return v
							}))
						}
					case interpreter.PrimitiveStaticTypeUInt16:
						arrayElementType = interpreter.PrimitiveStaticTypeUInt16
						elements, ok := decodedValues[i].([]uint16)
						if !ok {
							panic(abiDecodingError{Message: "could not decode ABI value to [UInt16]"})
						}
						for _, v := range elements {
							arrValues = append(arrValues, interpreter.NewUInt16Value(inter, func() uint16 {
								return v
							}))
						}
					case interpreter.PrimitiveStaticTypeUInt32:
						arrayElementType = interpreter.PrimitiveStaticTypeUInt32
						elements, ok := decodedValues[i].([]uint32)
						if !ok {
							panic(abiDecodingError{Message: "could not decode ABI value to [UInt32]"})
						}
						for _, v := range elements {
							arrValues = append(arrValues, interpreter.NewUInt32Value(inter, func() uint32 {
								return v
							}))
						}
					case interpreter.PrimitiveStaticTypeUInt64:
						arrayElementType = interpreter.PrimitiveStaticTypeUInt64
						elements, ok := decodedValues[i].([]uint64)
						if !ok {
							panic(abiDecodingError{Message: "could not decode ABI value to [UInt64]"})
						}
						for _, v := range elements {
							arrValues = append(arrValues, interpreter.NewUInt64Value(inter, func() uint64 {
								return v
							}))
						}
					case interpreter.PrimitiveStaticTypeUInt128:
						arrayElementType = interpreter.PrimitiveStaticTypeUInt128
						elements, ok := decodedValues[i].([]*big.Int)
						if !ok {
							panic(abiDecodingError{Message: "could not decode ABI value to [UInt128]"})
						}
						for _, v := range elements {
							arrValues = append(arrValues, interpreter.NewUInt128ValueFromBigInt(inter, func() *big.Int {
								return v
							}))
						}
					case interpreter.PrimitiveStaticTypeUInt256:
						arrayElementType = interpreter.PrimitiveStaticTypeUInt256
						elements, ok := decodedValues[i].([]*big.Int)
						if !ok {
							panic(abiDecodingError{Message: "could not decode ABI value to [UInt256]"})
						}
						for _, v := range elements {
							arrValues = append(arrValues, interpreter.NewUInt256ValueFromBigInt(inter, func() *big.Int {
								return v
							}))
						}
					case interpreter.PrimitiveStaticTypeInt8:
						arrayElementType = interpreter.PrimitiveStaticTypeInt8
						elements, ok := decodedValues[i].([]int8)
						if !ok {
							panic(abiDecodingError{Message: "could not decode ABI value to [Int8]"})
						}
						for _, v := range elements {
							arrValues = append(arrValues, interpreter.NewInt8Value(inter, func() int8 {
								return v
							}))
						}
					case interpreter.PrimitiveStaticTypeInt16:
						arrayElementType = interpreter.PrimitiveStaticTypeInt16
						elements, ok := decodedValues[i].([]int16)
						if !ok {
							panic(abiDecodingError{Message: "could not decode ABI value to [Int16]"})
						}
						for _, v := range elements {
							arrValues = append(arrValues, interpreter.NewInt16Value(inter, func() int16 {
								return v
							}))
						}
					case interpreter.PrimitiveStaticTypeInt32:
						arrayElementType = interpreter.PrimitiveStaticTypeInt32
						elements, ok := decodedValues[i].([]int32)
						if !ok {
							panic(abiDecodingError{Message: "could not decode ABI value to [Int32]"})
						}
						for _, v := range elements {
							arrValues = append(arrValues, interpreter.NewInt32Value(inter, func() int32 {
								return v
							}))
						}
					case interpreter.PrimitiveStaticTypeInt64:
						arrayElementType = interpreter.PrimitiveStaticTypeInt64
						elements, ok := decodedValues[i].([]int64)
						if !ok {
							panic(abiDecodingError{Message: "could not decode ABI value to [Int64]"})
						}
						for _, v := range elements {
							arrValues = append(arrValues, interpreter.NewInt64Value(inter, func() int64 {
								return v
							}))
						}
					case interpreter.PrimitiveStaticTypeInt128:
						arrayElementType = interpreter.PrimitiveStaticTypeInt128
						elements, ok := decodedValues[i].([]*big.Int)
						if !ok {
							panic(abiDecodingError{Message: "could not decode ABI value to [Int128]"})
						}
						for _, v := range elements {
							arrValues = append(arrValues, interpreter.NewInt128ValueFromBigInt(inter, func() *big.Int {
								return v
							}))
						}
					case interpreter.PrimitiveStaticTypeInt256:
						arrayElementType = interpreter.PrimitiveStaticTypeInt256
						elements, ok := decodedValues[i].([]*big.Int)
						if !ok {
							panic(abiDecodingError{Message: "could not decode ABI value to [Int256]"})
						}
						for _, v := range elements {
							arrValues = append(arrValues, interpreter.NewInt256ValueFromBigInt(inter, func() *big.Int {
								return v
							}))
						}
					}

					arrayType := interpreter.NewVariableSizedStaticType(
						inter,
						interpreter.NewPrimitiveStaticType(
							inter,
							arrayElementType,
						),
					)
					arr := interpreter.NewArrayValue(
						inter,
						invocation.LocationRange,
						arrayType,
						common.ZeroAddress,
						arrValues...,
					)
					values = append(values, arr)
				case interpreter.CompositeStaticType:
					if value.TypeID == evmAddressTypeID(location) {
						addr, ok := decodedValues[i].(gethCommon.Address)
						if !ok {
							panic(abiDecodingError{Message: "could not decode ABI value to EVM.EVMAddress"})
						}
						var address types.Address
						copy(address[:], addr.Bytes())
						compositeValue := interpreter.NewCompositeValue(
							inter,
							locationRange,
							location,
							fmt.Sprintf("%s.%s", ContractName, evmAddressTypeStructName),
							common.CompositeKindStructure,
							[]interpreter.CompositeField{
								{
									Name:  evmAddressTypeBytesFieldName,
									Value: EVMAddressToAddressBytesArrayValue(inter, address),
								},
							},
							common.ZeroAddress,
						)
						values = append(values, compositeValue)
					} else {
						panic(
							unsupportedValueABIDecodingError{
								Type: typeValue.Type,
							},
						)
					}
				}

				switch typeValue.Type {
				case interpreter.PrimitiveStaticTypeString:
					value, ok := decodedValues[i].(string)
					if !ok {
						panic(abiDecodingError{Message: "could not decode ABI value to String"})
					}
					values = append(values, interpreter.NewStringValue(
						inter,
						common.NewStringMemoryUsage(len(value)),
						func() string {
							return value
						},
					))
				case interpreter.PrimitiveStaticTypeBool:
					value, ok := decodedValues[i].(bool)
					if !ok {
						panic(abiDecodingError{Message: "could not decode ABI value to Bool"})
					}
					values = append(values, interpreter.BoolValue(value))
				case interpreter.PrimitiveStaticTypeUInt8:
					value, ok := decodedValues[i].(uint8)
					if !ok {
						panic(abiDecodingError{Message: "could not decode ABI value to UInt8"})
					}
					values = append(values, interpreter.NewUInt8Value(
						inter,
						func() uint8 {
							return value
						},
					))
				case interpreter.PrimitiveStaticTypeUInt16:
					value, ok := decodedValues[i].(uint16)
					if !ok {
						panic(abiDecodingError{Message: "could not decode ABI value to UInt16"})
					}
					values = append(values, interpreter.NewUInt16Value(
						inter,
						func() uint16 {
							return value
						},
					))
				case interpreter.PrimitiveStaticTypeUInt32:
					value, ok := decodedValues[i].(uint32)
					if !ok {
						panic(abiDecodingError{Message: "could not decode ABI value to UInt32"})
					}
					values = append(values, interpreter.NewUInt32Value(
						inter,
						func() uint32 {
							return value
						},
					))
				case interpreter.PrimitiveStaticTypeUInt64:
					value, ok := decodedValues[i].(uint64)
					if !ok {
						panic(abiDecodingError{Message: "could not decode ABI value to UInt64"})
					}
					values = append(values, interpreter.NewUInt64Value(
						inter,
						func() uint64 {
							return value
						},
					))
				case interpreter.PrimitiveStaticTypeUInt128:
					value, ok := decodedValues[i].(*big.Int)
					if !ok {
						panic(abiDecodingError{Message: "could not decode ABI value to UInt128"})
					}
					values = append(values, interpreter.NewUInt128ValueFromBigInt(
						inter,
						func() *big.Int {
							return value
						},
					))
				case interpreter.PrimitiveStaticTypeUInt256:
					value, ok := decodedValues[i].(*big.Int)
					if !ok {
						panic(abiDecodingError{Message: "could not decode ABI value to UInt256"})
					}
					values = append(values, interpreter.NewUInt256ValueFromBigInt(
						inter,
						func() *big.Int {
							return value
						},
					))
				case interpreter.PrimitiveStaticTypeInt8:
					value, ok := decodedValues[i].(int8)
					if !ok {
						panic(abiDecodingError{Message: "could not decode ABI value to Int8"})
					}
					values = append(values, interpreter.NewInt8Value(
						inter,
						func() int8 {
							return value
						},
					))
				case interpreter.PrimitiveStaticTypeInt16:
					value, ok := decodedValues[i].(int16)
					if !ok {
						panic(abiDecodingError{Message: "could not decode ABI value to Int16"})
					}
					values = append(values, interpreter.NewInt16Value(
						inter,
						func() int16 {
							return value
						},
					))
				case interpreter.PrimitiveStaticTypeInt32:
					value, ok := decodedValues[i].(int32)
					if !ok {
						panic(abiDecodingError{Message: "could not decode ABI value to Int32"})
					}
					values = append(values, interpreter.NewInt32Value(
						inter,
						func() int32 {
							return value
						},
					))
				case interpreter.PrimitiveStaticTypeInt64:
					value, ok := decodedValues[i].(int64)
					if !ok {
						panic(abiDecodingError{Message: "could not decode ABI value to Int64"})
					}
					values = append(values, interpreter.NewInt64Value(
						inter,
						func() int64 {
							return value
						},
					))
				case interpreter.PrimitiveStaticTypeInt128:
					value, ok := decodedValues[i].(*big.Int)
					if !ok {
						panic(abiDecodingError{Message: "could not decode ABI value to Int128"})
					}
					values = append(values, interpreter.NewInt128ValueFromBigInt(
						inter,
						func() *big.Int {
							return value
						},
					))
				case interpreter.PrimitiveStaticTypeInt256:
					value, ok := decodedValues[i].(*big.Int)
					if !ok {
						panic(abiDecodingError{Message: "could not decode ABI value to Int256"})
					}
					values = append(values, interpreter.NewInt256ValueFromBigInt(
						inter,
						func() *big.Int {
							return value
						},
					))
				}

				i += 1
				// continue iteration
				return true
			})

			arrayType := interpreter.NewVariableSizedStaticType(
				inter,
				interpreter.NewPrimitiveStaticType(
					inter,
					interpreter.PrimitiveStaticTypeAnyStruct,
				),
			)

			return interpreter.NewArrayValue(
				inter,
				invocation.LocationRange,
				arrayType,
				common.ZeroAddress,
				values...,
			)
		},
	)
}

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
	location common.AddressLocation,
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
			internalEVMTypeEncodeABIFunctionName:            newInternalEVMTypeEncodeABIFunction(gauge, location),
			internalEVMTypeDecodeABIFunctionName:            newInternalEVMTypeDecodeABIFunction(gauge, location),
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
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			internalEVMTypeEncodeABIFunctionName,
			internalEVMTypeEncodeABIFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			internalEVMTypeDecodeABIFunctionName,
			internalEVMTypeDecodeABIFunctionType,
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
	location common.AddressLocation,
) stdlib.StandardLibraryValue {
	return stdlib.StandardLibraryValue{
		Name:  InternalEVMContractName,
		Type:  InternalEVMContractType,
		Value: NewInternalEVMContractValue(gauge, handler, location),
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
		newInternalEVMStandardLibraryValue(nil, handler, location),
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
				Type:       cadence.UFix64Type{},
			},
		},
		nil,
	)
}
