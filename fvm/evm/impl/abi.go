package impl

import (
	"math"
	"math/big"
	"reflect"
	"strings"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/interpreter"
	gethABI "github.com/onflow/go-ethereum/accounts/abi"
	gethCommon "github.com/onflow/go-ethereum/common"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/evm/types"
)

const abiEncodingByteSize = 32

// abiEncodingError
type abiEncodingError struct {
	Type interpreter.StaticType
}

var _ errors.UserError = abiEncodingError{}

func (abiEncodingError) IsUserError() {}

func (e abiEncodingError) Error() string {
	var b strings.Builder
	b.WriteString("failed to ABI encode value")

	ty := e.Type
	if ty != nil {
		b.WriteString(" of type ")
		b.WriteString(ty.String())
	}

	return b.String()
}

// abiDecodingError
type abiDecodingError struct {
	Type    interpreter.StaticType
	Message string
}

var _ errors.UserError = abiDecodingError{}

func (abiDecodingError) IsUserError() {}

func (e abiDecodingError) Error() string {
	var b strings.Builder
	b.WriteString("failed to ABI decode data")

	ty := e.Type
	if ty != nil {
		b.WriteString(" with type ")
		b.WriteString(ty.String())
	}

	message := e.Message
	if message != "" {
		b.WriteString(": ")
		b.WriteString(message)
	}

	return b.String()
}

func reportABIEncodingComputation(
	inter *interpreter.Interpreter,
	locationRange interpreter.LocationRange,
	values *interpreter.ArrayValue,
	evmAddressTypeID common.TypeID,
	reportComputation func(intensity uint),
) {
	values.Iterate(
		inter,
		func(element interpreter.Value) (resume bool) {
			switch value := element.(type) {
			case *interpreter.StringValue:
				// Dynamic variables, such as strings, are encoded
				// in 2+ chunks of 32 bytes. The first chunk contains
				// the index where information for the string begin,
				// the second chunk contains the number of bytes the
				// string occupies, and the third chunk contains the
				// value of the string itself.
				computation := uint(2 * abiEncodingByteSize)
				stringLength := len(value.Str)
				chunks := math.Ceil(float64(stringLength) / float64(abiEncodingByteSize))
				computation += uint(chunks * abiEncodingByteSize)
				reportComputation(computation)

			case interpreter.BoolValue,
				interpreter.UInt8Value,
				interpreter.UInt16Value,
				interpreter.UInt32Value,
				interpreter.UInt64Value,
				interpreter.UInt128Value,
				interpreter.UInt256Value,
				interpreter.Int8Value,
				interpreter.Int16Value,
				interpreter.Int32Value,
				interpreter.Int64Value,
				interpreter.Int128Value,
				interpreter.Int256Value:

				// Numeric and bool variables are also static variables
				// with a fixed size of 32 bytes.
				reportComputation(abiEncodingByteSize)

			case *interpreter.CompositeValue:
				if value.TypeID() == evmAddressTypeID {
					// EVM addresses are static variables with a fixed
					// size of 32 bytes.
					reportComputation(abiEncodingByteSize)
				} else {
					panic(abiEncodingError{
						Type: value.StaticType(inter),
					})
				}
			case *interpreter.ArrayValue:
				// Dynamic variables, such as arrays & slices, are encoded
				// in 2+ chunks of 32 bytes. The first chunk contains
				// the index where information for the array begin,
				// the second chunk contains the number of bytes the
				// array occupies, and the third chunk contains the
				// values of the array itself.
				computation := uint(2 * abiEncodingByteSize)
				reportComputation(computation)
				reportABIEncodingComputation(
					inter,
					locationRange,
					value,
					evmAddressTypeID,
					reportComputation,
				)

			default:
				panic(abiEncodingError{
					Type: element.StaticType(inter),
				})
			}

			// continue iteration
			return true
		},
		false,
		locationRange,
	)
}

func newInternalEVMTypeEncodeABIFunction(
	gauge common.MemoryGauge,
	location common.AddressLocation,
) *interpreter.HostFunctionValue {

	evmAddressTypeID := location.TypeID(gauge, stdlib.EVMAddressTypeQualifiedIdentifier)

	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeEncodeABIFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			inter := invocation.Interpreter
			locationRange := invocation.LocationRange

			// Get `values` argument

			valuesArray, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			reportABIEncodingComputation(
				inter,
				locationRange,
				valuesArray,
				evmAddressTypeID,
				func(intensity uint) {
					inter.ReportComputation(environment.ComputationKindEVMEncodeABI, intensity)
				},
			)

			size := valuesArray.Count()

			values := make([]any, 0, size)
			arguments := make(gethABI.Arguments, 0, size)

			valuesArray.Iterate(
				inter,
				func(element interpreter.Value) (resume bool) {
					value, ty, err := encodeABI(
						inter,
						locationRange,
						element,
						element.StaticType(inter),
						evmAddressTypeID,
					)
					if err != nil {
						panic(err)
					}

					values = append(values, value)
					arguments = append(arguments, gethABI.Argument{Type: ty})

					// continue iteration
					return true
				},
				false,
				locationRange,
			)

			encodedValues, err := arguments.Pack(values...)
			if err != nil {
				panic(abiEncodingError{})
			}

			return interpreter.ByteSliceToByteArrayValue(inter, encodedValues)
		},
	)
}

var gethTypeString = gethABI.Type{T: gethABI.StringTy}

var gethTypeBool = gethABI.Type{T: gethABI.BoolTy}

var gethTypeUint8 = gethABI.Type{T: gethABI.UintTy, Size: 8}

var gethTypeUint16 = gethABI.Type{T: gethABI.UintTy, Size: 16}

var gethTypeUint32 = gethABI.Type{T: gethABI.UintTy, Size: 32}

var gethTypeUint64 = gethABI.Type{T: gethABI.UintTy, Size: 64}

var gethTypeUint128 = gethABI.Type{T: gethABI.UintTy, Size: 128}

var gethTypeUint256 = gethABI.Type{T: gethABI.UintTy, Size: 256}

var gethTypeInt8 = gethABI.Type{T: gethABI.IntTy, Size: 8}

var gethTypeInt16 = gethABI.Type{T: gethABI.IntTy, Size: 16}

var gethTypeInt32 = gethABI.Type{T: gethABI.IntTy, Size: 32}

var gethTypeInt64 = gethABI.Type{T: gethABI.IntTy, Size: 64}

var gethTypeInt128 = gethABI.Type{T: gethABI.IntTy, Size: 128}

var gethTypeInt256 = gethABI.Type{T: gethABI.IntTy, Size: 256}

var gethTypeAddress = gethABI.Type{Size: 20, T: gethABI.AddressTy}

func gethABIType(staticType interpreter.StaticType, evmAddressTypeID common.TypeID) (gethABI.Type, bool) {
	switch staticType {
	case interpreter.PrimitiveStaticTypeString:
		return gethTypeString, true
	case interpreter.PrimitiveStaticTypeBool:
		return gethTypeBool, true
	case interpreter.PrimitiveStaticTypeUInt8:
		return gethTypeUint8, true
	case interpreter.PrimitiveStaticTypeUInt16:
		return gethTypeUint16, true
	case interpreter.PrimitiveStaticTypeUInt32:
		return gethTypeUint32, true
	case interpreter.PrimitiveStaticTypeUInt64:
		return gethTypeUint64, true
	case interpreter.PrimitiveStaticTypeUInt128:
		return gethTypeUint128, true
	case interpreter.PrimitiveStaticTypeUInt256:
		return gethTypeUint256, true
	case interpreter.PrimitiveStaticTypeInt8:
		return gethTypeInt8, true
	case interpreter.PrimitiveStaticTypeInt16:
		return gethTypeInt16, true
	case interpreter.PrimitiveStaticTypeInt32:
		return gethTypeInt32, true
	case interpreter.PrimitiveStaticTypeInt64:
		return gethTypeInt64, true
	case interpreter.PrimitiveStaticTypeInt128:
		return gethTypeInt128, true
	case interpreter.PrimitiveStaticTypeInt256:
		return gethTypeInt256, true
	case interpreter.PrimitiveStaticTypeAddress:
		return gethTypeAddress, true
	}

	switch staticType := staticType.(type) {
	case *interpreter.CompositeStaticType:
		if staticType.TypeID != evmAddressTypeID {
			break
		}

		return gethTypeAddress, true

	case *interpreter.ConstantSizedStaticType:
		elementGethABIType, ok := gethABIType(
			staticType.ElementType(),
			evmAddressTypeID,
		)
		if !ok {
			break
		}

		return gethABI.Type{
			T:    gethABI.ArrayTy,
			Elem: &elementGethABIType,
			Size: int(staticType.Size),
		}, true

	case *interpreter.VariableSizedStaticType:
		elementGethABIType, ok := gethABIType(
			staticType.ElementType(),
			evmAddressTypeID,
		)
		if !ok {
			break
		}

		return gethABI.Type{
			T:    gethABI.SliceTy,
			Elem: &elementGethABIType,
		}, true

	}

	return gethABI.Type{}, false
}

func goType(
	staticType interpreter.StaticType,
	evmAddressTypeID common.TypeID,
) (reflect.Type, bool) {
	switch staticType {
	case interpreter.PrimitiveStaticTypeString:
		return reflect.TypeOf(""), true
	case interpreter.PrimitiveStaticTypeBool:
		return reflect.TypeOf(true), true
	case interpreter.PrimitiveStaticTypeUInt8:
		return reflect.TypeOf(uint8(0)), true
	case interpreter.PrimitiveStaticTypeUInt16:
		return reflect.TypeOf(uint16(0)), true
	case interpreter.PrimitiveStaticTypeUInt32:
		return reflect.TypeOf(uint32(0)), true
	case interpreter.PrimitiveStaticTypeUInt64:
		return reflect.TypeOf(uint64(0)), true
	case interpreter.PrimitiveStaticTypeUInt128:
		return reflect.TypeOf((*big.Int)(nil)), true
	case interpreter.PrimitiveStaticTypeUInt256:
		return reflect.TypeOf((*big.Int)(nil)), true
	case interpreter.PrimitiveStaticTypeInt8:
		return reflect.TypeOf(int8(0)), true
	case interpreter.PrimitiveStaticTypeInt16:
		return reflect.TypeOf(int16(0)), true
	case interpreter.PrimitiveStaticTypeInt32:
		return reflect.TypeOf(int32(0)), true
	case interpreter.PrimitiveStaticTypeInt64:
		return reflect.TypeOf(int64(0)), true
	case interpreter.PrimitiveStaticTypeInt128:
		return reflect.TypeOf((*big.Int)(nil)), true
	case interpreter.PrimitiveStaticTypeInt256:
		return reflect.TypeOf((*big.Int)(nil)), true
	case interpreter.PrimitiveStaticTypeAddress:
		return reflect.TypeOf((*big.Int)(nil)), true
	}

	switch staticType := staticType.(type) {
	case *interpreter.ConstantSizedStaticType:
		elementType, ok := goType(staticType.ElementType(), evmAddressTypeID)
		if !ok {
			break
		}

		return reflect.ArrayOf(int(staticType.Size), elementType), true

	case *interpreter.VariableSizedStaticType:
		elementType, ok := goType(staticType.ElementType(), evmAddressTypeID)
		if !ok {
			break
		}

		return reflect.SliceOf(elementType), true
	}

	if staticType.ID() == evmAddressTypeID {
		return reflect.TypeOf(gethCommon.Address{}), true
	}

	return nil, false
}

func encodeABI(
	inter *interpreter.Interpreter,
	locationRange interpreter.LocationRange,
	value interpreter.Value,
	staticType interpreter.StaticType,
	evmAddressTypeID common.TypeID,
) (
	any,
	gethABI.Type,
	error,
) {

	switch value := value.(type) {
	case *interpreter.StringValue:
		if staticType == interpreter.PrimitiveStaticTypeString {
			return value.Str, gethTypeString, nil
		}

	case interpreter.BoolValue:
		if staticType == interpreter.PrimitiveStaticTypeBool {
			return bool(value), gethTypeBool, nil
		}

	case interpreter.UInt8Value:
		if staticType == interpreter.PrimitiveStaticTypeUInt8 {
			return uint8(value), gethTypeUint8, nil
		}

	case interpreter.UInt16Value:
		if staticType == interpreter.PrimitiveStaticTypeUInt16 {
			return uint16(value), gethTypeUint16, nil
		}

	case interpreter.UInt32Value:
		if staticType == interpreter.PrimitiveStaticTypeUInt32 {
			return uint32(value), gethTypeUint32, nil
		}

	case interpreter.UInt64Value:
		if staticType == interpreter.PrimitiveStaticTypeUInt64 {
			return uint64(value), gethTypeUint64, nil
		}

	case interpreter.UInt128Value:
		if staticType == interpreter.PrimitiveStaticTypeUInt128 {
			return value.BigInt, gethTypeUint128, nil
		}

	case interpreter.UInt256Value:
		if staticType == interpreter.PrimitiveStaticTypeUInt256 {
			return value.BigInt, gethTypeUint256, nil
		}

	case interpreter.Int8Value:
		if staticType == interpreter.PrimitiveStaticTypeInt8 {
			return int8(value), gethTypeInt8, nil
		}

	case interpreter.Int16Value:
		if staticType == interpreter.PrimitiveStaticTypeInt16 {
			return int16(value), gethTypeInt16, nil
		}

	case interpreter.Int32Value:
		if staticType == interpreter.PrimitiveStaticTypeInt32 {
			return int32(value), gethTypeInt32, nil
		}

	case interpreter.Int64Value:
		if staticType == interpreter.PrimitiveStaticTypeInt64 {
			return int64(value), gethTypeInt64, nil
		}

	case interpreter.Int128Value:
		if staticType == interpreter.PrimitiveStaticTypeInt128 {
			return value.BigInt, gethTypeInt128, nil
		}

	case interpreter.Int256Value:
		if staticType == interpreter.PrimitiveStaticTypeInt256 {
			return value.BigInt, gethTypeInt256, nil
		}

	case *interpreter.CompositeValue:
		if value.TypeID() == evmAddressTypeID {
			addressBytesArrayValue := value.GetMember(inter, locationRange, stdlib.EVMAddressTypeBytesFieldName)
			bytes, err := interpreter.ByteArrayValueToByteSlice(
				inter,
				addressBytesArrayValue,
				locationRange,
			)
			if err != nil {
				panic(err)
			}

			return gethCommon.Address(bytes), gethTypeAddress, nil
		}

	case *interpreter.ArrayValue:
		arrayStaticType := value.Type

		arrayGethABIType, ok := gethABIType(arrayStaticType, evmAddressTypeID)
		if !ok {
			break
		}

		elementStaticType := arrayStaticType.ElementType()

		elementGoType, ok := goType(elementStaticType, evmAddressTypeID)
		if !ok {
			break
		}

		var result reflect.Value

		switch arrayStaticType := arrayStaticType.(type) {
		case *interpreter.ConstantSizedStaticType:
			size := int(arrayStaticType.Size)
			result = reflect.Indirect(reflect.New(reflect.ArrayOf(size, elementGoType)))

		case *interpreter.VariableSizedStaticType:
			size := value.Count()
			result = reflect.MakeSlice(reflect.SliceOf(elementGoType), size, size)
		}

		var index int
		value.Iterate(
			inter,
			func(element interpreter.Value) (resume bool) {

				arrayElement, _, err := encodeABI(
					inter,
					locationRange,
					element,
					element.StaticType(inter),
					evmAddressTypeID,
				)
				if err != nil {
					panic(err)
				}

				result.Index(index).Set(reflect.ValueOf(arrayElement))

				index++

				// continue iteration
				return true
			},
			false,
			locationRange,
		)

		return result.Interface(), arrayGethABIType, nil
	}

	return nil, gethABI.Type{}, abiEncodingError{
		Type: value.StaticType(inter),
	}
}
func newInternalEVMTypeDecodeABIFunction(
	gauge common.MemoryGauge,
	location common.AddressLocation,
) *interpreter.HostFunctionValue {
	evmAddressTypeID := location.TypeID(gauge, stdlib.EVMAddressTypeQualifiedIdentifier)

	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeDecodeABIFunctionType,
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

			invocation.Interpreter.ReportComputation(
				environment.ComputationKindEVMDecodeABI,
				uint(dataValue.Count()),
			)

			data, err := interpreter.ByteArrayValueToByteSlice(inter, dataValue, locationRange)
			if err != nil {
				panic(err)
			}

			var arguments gethABI.Arguments
			typesArray.Iterate(
				inter,
				func(element interpreter.Value) (resume bool) {
					typeValue, ok := element.(interpreter.TypeValue)
					if !ok {
						panic(errors.NewUnreachableError())
					}

					staticType := typeValue.Type

					gethABITy, ok := gethABIType(staticType, evmAddressTypeID)
					if !ok {
						panic(abiDecodingError{
							Type: staticType,
						})
					}

					arguments = append(
						arguments,
						gethABI.Argument{
							Type: gethABITy,
						},
					)

					// continue iteration
					return true
				},
				false,
				locationRange,
			)

			decodedValues, err := arguments.Unpack(data)
			if err != nil {
				panic(abiDecodingError{})
			}

			var index int
			values := make([]interpreter.Value, 0, len(decodedValues))

			typesArray.Iterate(
				inter,
				func(element interpreter.Value) (resume bool) {
					typeValue, ok := element.(interpreter.TypeValue)
					if !ok {
						panic(errors.NewUnreachableError())
					}

					staticType := typeValue.Type

					value, err := decodeABI(
						inter,
						locationRange,
						decodedValues[index],
						staticType,
						location,
						evmAddressTypeID,
					)
					if err != nil {
						panic(err)
					}

					index++

					values = append(values, value)

					// continue iteration
					return true
				},
				false,
				locationRange,
			)

			arrayType := interpreter.NewVariableSizedStaticType(
				inter,
				interpreter.NewPrimitiveStaticType(
					inter,
					interpreter.PrimitiveStaticTypeAnyStruct,
				),
			)

			return interpreter.NewArrayValue(
				inter,
				locationRange,
				arrayType,
				common.ZeroAddress,
				values...,
			)
		},
	)
}

func decodeABI(
	inter *interpreter.Interpreter,
	locationRange interpreter.LocationRange,
	value any,
	staticType interpreter.StaticType,
	location common.AddressLocation,
	evmAddressTypeID common.TypeID,
) (
	interpreter.Value,
	error,
) {

	switch staticType {
	case interpreter.PrimitiveStaticTypeString:
		value, ok := value.(string)
		if !ok {
			break
		}
		return interpreter.NewStringValue(
			inter,
			common.NewStringMemoryUsage(len(value)),
			func() string {
				return value
			},
		), nil

	case interpreter.PrimitiveStaticTypeBool:
		value, ok := value.(bool)
		if !ok {
			break
		}
		return interpreter.BoolValue(value), nil

	case interpreter.PrimitiveStaticTypeUInt8:
		value, ok := value.(uint8)
		if !ok {
			break
		}
		return interpreter.NewUInt8Value(inter, func() uint8 { return value }), nil

	case interpreter.PrimitiveStaticTypeUInt16:
		value, ok := value.(uint16)
		if !ok {
			break
		}
		return interpreter.NewUInt16Value(inter, func() uint16 { return value }), nil

	case interpreter.PrimitiveStaticTypeUInt32:
		value, ok := value.(uint32)
		if !ok {
			break
		}
		return interpreter.NewUInt32Value(inter, func() uint32 { return value }), nil

	case interpreter.PrimitiveStaticTypeUInt64:
		value, ok := value.(uint64)
		if !ok {
			break
		}
		return interpreter.NewUInt64Value(inter, func() uint64 { return value }), nil

	case interpreter.PrimitiveStaticTypeUInt128:
		value, ok := value.(*big.Int)
		if !ok {
			break
		}
		return interpreter.NewUInt128ValueFromBigInt(inter, func() *big.Int { return value }), nil

	case interpreter.PrimitiveStaticTypeUInt256:
		value, ok := value.(*big.Int)
		if !ok {
			break
		}
		return interpreter.NewUInt256ValueFromBigInt(inter, func() *big.Int { return value }), nil

	case interpreter.PrimitiveStaticTypeInt8:
		value, ok := value.(int8)
		if !ok {
			break
		}
		return interpreter.NewInt8Value(inter, func() int8 { return value }), nil

	case interpreter.PrimitiveStaticTypeInt16:
		value, ok := value.(int16)
		if !ok {
			break
		}
		return interpreter.NewInt16Value(inter, func() int16 { return value }), nil

	case interpreter.PrimitiveStaticTypeInt32:
		value, ok := value.(int32)
		if !ok {
			break
		}
		return interpreter.NewInt32Value(inter, func() int32 { return value }), nil

	case interpreter.PrimitiveStaticTypeInt64:
		value, ok := value.(int64)
		if !ok {
			break
		}
		return interpreter.NewInt64Value(inter, func() int64 { return value }), nil

	case interpreter.PrimitiveStaticTypeInt128:
		value, ok := value.(*big.Int)
		if !ok {
			break
		}
		return interpreter.NewInt128ValueFromBigInt(inter, func() *big.Int { return value }), nil

	case interpreter.PrimitiveStaticTypeInt256:
		value, ok := value.(*big.Int)
		if !ok {
			break
		}
		return interpreter.NewInt256ValueFromBigInt(inter, func() *big.Int { return value }), nil
	}

	switch staticType := staticType.(type) {
	case interpreter.ArrayStaticType:
		array := reflect.ValueOf(value)

		elementStaticType := staticType.ElementType()

		size := array.Len()

		var index int
		return interpreter.NewArrayValueWithIterator(
			inter,
			staticType,
			common.ZeroAddress,
			uint64(size),
			func() interpreter.Value {
				if index >= size {
					return nil
				}

				element := array.Index(index).Interface()

				result, err := decodeABI(
					inter,
					locationRange,
					element,
					elementStaticType,
					location,
					evmAddressTypeID,
				)
				if err != nil {
					panic(err)
				}

				index++

				return result
			},
		), nil

	case *interpreter.CompositeStaticType:
		if staticType.TypeID != evmAddressTypeID {
			break
		}

		addr, ok := value.(gethCommon.Address)
		if !ok {
			break
		}

		var address types.Address
		copy(address[:], addr.Bytes())
		return NewEVMAddress(
			inter,
			locationRange,
			location,
			address,
		), nil
	}

	return nil, abiDecodingError{
		Type: staticType,
	}
}
