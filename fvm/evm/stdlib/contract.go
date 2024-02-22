package stdlib

import (
	_ "embed"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"regexp"
	"strings"

	gethABI "github.com/ethereum/go-ethereum/accounts/abi"
	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/errors"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

//go:embed contract.cdc
var contractCode string

//go:embed abiOnlyContract.cdc
var abiOnlyContractCode string

var flowTokenImportPattern = regexp.MustCompile(`(?m)^import "FlowToken"\n`)

func ContractCode(flowTokenAddress flow.Address, evmAbiOnly bool) []byte {
	if evmAbiOnly {
		return []byte(abiOnlyContractCode)
	}

	return []byte(flowTokenImportPattern.ReplaceAllString(
		contractCode,
		fmt.Sprintf("import FlowToken from %s", flowTokenAddress.HexWithPrefix()),
	))
}

const ContractName = "EVM"
const evmAddressTypeBytesFieldName = "bytes"
const evmAddressTypeQualifiedIdentifier = "EVM.EVMAddress"
const evmBalanceTypeQualifiedIdentifier = "EVM.Balance"
const evmResultTypeQualifiedIdentifier = "EVM.Result"
const evmStatusTypeQualifiedIdentifier = "EVM.Status"

const abiEncodingByteSize = 32

var EVMTransactionBytesCadenceType = cadence.NewVariableSizedArrayType(cadence.UInt8Type)
var evmTransactionBytesType = sema.NewVariableSizedType(nil, sema.UInt8Type)

var evmAddressBytesType = sema.NewConstantSizedType(nil, sema.UInt8Type, types.AddressLength)
var evmAddressBytesStaticType = interpreter.ConvertSemaArrayTypeToStaticArrayType(nil, evmAddressBytesType)
var EVMAddressBytesCadenceType = cadence.NewConstantSizedArrayType(types.AddressLength, cadence.UInt8Type)

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
	values *interpreter.ArrayValue,
	evmAddressTypeID common.TypeID,
	reportComputation func(intensity uint),
) {
	values.Iterate(inter, func(element interpreter.Value) (resume bool) {
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
			reportABIEncodingComputation(inter, value, evmAddressTypeID, reportComputation)

		default:
			panic(abiEncodingError{
				Type: element.StaticType(inter),
			})
		}

		// continue iteration
		return true
	})
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

	evmAddressTypeID := location.TypeID(gauge, evmAddressTypeQualifiedIdentifier)

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

			reportABIEncodingComputation(
				inter,
				valuesArray,
				evmAddressTypeID,
				func(intensity uint) {
					inter.ReportComputation(environment.ComputationKindEVMEncodeABI, intensity)
				},
			)

			size := valuesArray.Count()

			values := make([]any, 0, size)
			arguments := make(gethABI.Arguments, 0, size)

			valuesArray.Iterate(inter, func(element interpreter.Value) (resume bool) {
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
			})

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
			addressBytesArrayValue := value.GetMember(inter, locationRange, evmAddressTypeBytesFieldName)
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
		value.Iterate(inter, func(element interpreter.Value) (resume bool) {

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
		})

		return result.Interface(), arrayGethABIType, nil
	}

	return nil, gethABI.Type{}, abiEncodingError{
		Type: value.StaticType(inter),
	}
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
	evmAddressTypeID := location.TypeID(gauge, evmAddressTypeQualifiedIdentifier)

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

			invocation.Interpreter.ReportComputation(
				environment.ComputationKindEVMDecodeABI,
				uint(dataValue.Count()),
			)

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
			})

			decodedValues, err := arguments.Unpack(data)
			if err != nil {
				panic(abiDecodingError{})
			}

			var index int
			values := make([]interpreter.Value, 0, len(decodedValues))

			typesArray.Iterate(inter, func(element interpreter.Value) (resume bool) {
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

func NewEVMAddress(
	inter *interpreter.Interpreter,
	locationRange interpreter.LocationRange,
	location common.AddressLocation,
	address types.Address,
) *interpreter.CompositeValue {
	return interpreter.NewCompositeValue(
		inter,
		locationRange,
		location,
		evmAddressTypeQualifiedIdentifier,
		common.CompositeKindStructure,
		[]interpreter.CompositeField{
			{
				Name:  evmAddressTypeBytesFieldName,
				Value: EVMAddressToAddressBytesArrayValue(inter, address),
			},
		},
		common.ZeroAddress,
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
	// Actually EVM.Result, but cannot refer to it here
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.AnyStructType),
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
			result := handler.Run(transaction, cb)

			return NewResultValue(handler, gauge, inter, locationRange, result)
		},
	)
}

func NewResultValue(
	handler types.ContractHandler,
	gauge common.MemoryGauge,
	inter *interpreter.Interpreter,
	locationRange interpreter.LocationRange,
	result *types.ResultSummary,
) *interpreter.CompositeValue {
	loc := common.NewAddressLocation(gauge, handler.EVMContractAddress(), ContractName)
	return interpreter.NewCompositeValue(
		inter,
		locationRange,
		loc,
		evmResultTypeQualifiedIdentifier,
		common.CompositeKindStructure,
		[]interpreter.CompositeField{
			{
				Name: "status",
				Value: interpreter.NewEnumCaseValue(
					inter,
					locationRange,
					&sema.CompositeType{
						Location:   loc,
						Identifier: evmStatusTypeQualifiedIdentifier,
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
				Name: "gasUsed",
				Value: interpreter.NewUInt64Value(gauge, func() uint64 {
					return result.GasConsumed
				}),
			},
			{
				Name:  "data",
				Value: interpreter.ByteSliceToByteArrayValue(inter, result.ReturnedValue),
			},
		},
		common.ZeroAddress,
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
			TypeAnnotation: sema.NewTypeAnnotation(sema.UIntType),
		},
	},
	// Actually EVM.Result, but cannot refer to it here
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.AnyStructType),
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

			balanceValue, ok := invocation.Arguments[4].(interpreter.UIntValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			balance := types.NewBalance(balanceValue.BigInt)
			// Call

			const isAuthorized = true
			account := handler.AccountByAddress(fromAddress, isAuthorized)
			result := account.Call(toAddress, data, gasLimit, balance)

			return NewResultValue(handler, gauge, inter, locationRange, result)
		},
	)
}

const internalEVMTypeCreateCadenceOwnedAccountFunctionName = "createCadenceOwnedAccount"

var internalEVMTypeCreateCadenceOwnedAccountFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          "uuid",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UInt64Type),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(evmAddressBytesType),
}

func newInternalEVMTypeCreateCadenceOwnedAccountFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		internalEVMTypeCreateCadenceOwnedAccountFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			inter := invocation.Interpreter
			uuid, ok := invocation.Arguments[0].(interpreter.UInt64Value)
			if !ok {
				panic(errors.NewUnreachableError())
			}
			address := handler.DeployCOA(uint64(uuid))
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

			amount := types.NewBalanceFromUFix64(cadence.UFix64(amountValue))

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
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.UIntType),
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

			return interpreter.UIntValue{BigInt: account.Balance()}
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
			TypeAnnotation: sema.NewTypeAnnotation(sema.UIntType),
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

			amountValue, ok := invocation.Arguments[1].(interpreter.UIntValue)
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
				inter,
				locationRange,
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
			TypeAnnotation: sema.NewTypeAnnotation(sema.UIntType),
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

			amountValue, ok := invocation.Arguments[3].(interpreter.UIntValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			amount := types.NewBalance(amountValue.BigInt)

			// Deploy

			const isAuthorized = true
			account := handler.AccountByAddress(fromAddress, isAuthorized)
			address := account.Deploy(code, gasLimit, amount)

			return EVMAddressToAddressBytesArrayValue(inter, address)
		},
	)
}

const internalEVMTypeCastToAttoFLOWFunctionName = "castToAttoFLOW"

var internalEVMTypeCastToAttoFLOWFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          "balance",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UFix64Type),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.UIntType),
}

func newInternalEVMTypeCastToAttoFLOWFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		internalEVMTypeCallFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			balanceValue, ok := invocation.Arguments[0].(interpreter.UFix64Value)
			if !ok {
				panic(errors.NewUnreachableError())
			}
			balance := types.NewBalanceFromUFix64(cadence.UFix64(balanceValue))
			return interpreter.UIntValue{BigInt: balance}
		},
	)
}

const internalEVMTypeCastToFLOWFunctionName = "castToFLOW"

var internalEVMTypeCastToFLOWFunctionType = &sema.FunctionType{
	Parameters: []sema.Parameter{
		{
			Label:          "balance",
			TypeAnnotation: sema.NewTypeAnnotation(sema.UIntType),
		},
	},
	ReturnTypeAnnotation: sema.NewTypeAnnotation(sema.UFix64Type),
}

func newInternalEVMTypeCastToFLOWFunction(
	gauge common.MemoryGauge,
	handler types.ContractHandler,
) *interpreter.HostFunctionValue {
	return interpreter.NewHostFunctionValue(
		gauge,
		internalEVMTypeCallFunctionType,
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
			return interpreter.UFix64Value(v)
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
			internalEVMTypeRunFunctionName:                       newInternalEVMTypeRunFunction(gauge, handler),
			internalEVMTypeCreateCadenceOwnedAccountFunctionName: newInternalEVMTypeCreateCadenceOwnedAccountFunction(gauge, handler),
			internalEVMTypeCallFunctionName:                      newInternalEVMTypeCallFunction(gauge, handler),
			internalEVMTypeDepositFunctionName:                   newInternalEVMTypeDepositFunction(gauge, handler),
			internalEVMTypeWithdrawFunctionName:                  newInternalEVMTypeWithdrawFunction(gauge, handler),
			internalEVMTypeDeployFunctionName:                    newInternalEVMTypeDeployFunction(gauge, handler),
			internalEVMTypeBalanceFunctionName:                   newInternalEVMTypeBalanceFunction(gauge, handler),
			internalEVMTypeEncodeABIFunctionName:                 newInternalEVMTypeEncodeABIFunction(gauge, location),
			internalEVMTypeDecodeABIFunctionName:                 newInternalEVMTypeDecodeABIFunction(gauge, location),
			internalEVMTypeCastToAttoFLOWFunctionName:            newInternalEVMTypeCastToAttoFLOWFunction(gauge, handler),
			internalEVMTypeCastToFLOWFunctionName:                newInternalEVMTypeCastToFLOWFunction(gauge, handler),
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
			internalEVMTypeCreateCadenceOwnedAccountFunctionName,
			internalEVMTypeCreateCadenceOwnedAccountFunctionType,
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
			internalEVMTypeCastToAttoFLOWFunctionName,
			internalEVMTypeCastToAttoFLOWFunctionType,
			"",
		),
		sema.NewUnmeteredPublicFunctionMember(
			ty,
			internalEVMTypeCastToFLOWFunctionName,
			internalEVMTypeCastToFLOWFunctionType,
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
		evmAddressTypeQualifiedIdentifier,
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
		evmBalanceTypeQualifiedIdentifier,
		[]cadence.Field{
			{
				Identifier: "attoflow",
				Type:       cadence.UIntType,
			},
		},
		nil,
	)
}

func ResultSummaryFromEVMResultValue(val cadence.Value) (*types.ResultSummary, error) {
	str, ok := val.(cadence.Struct)
	if !ok {
		return nil, fmt.Errorf("invalid input: unexpected value type")
	}
	if len(str.Fields) != 4 {
		return nil, fmt.Errorf("invalid input: field count mismatch")
	}

	statusEnum, ok := str.Fields[0].(cadence.Enum)
	if !ok {
		return nil, fmt.Errorf("invalid input: unexpected type for status field")
	}

	status, ok := statusEnum.Fields[0].(cadence.UInt8)
	if !ok {
		return nil, fmt.Errorf("invalid input: unexpected type for status field")
	}

	errorCode, ok := str.Fields[1].(cadence.UInt64)
	if !ok {
		return nil, fmt.Errorf("invalid input: unexpected type for error code field")
	}

	gasUsed, ok := str.Fields[2].(cadence.UInt64)
	if !ok {
		return nil, fmt.Errorf("invalid input: unexpected type for gas field")
	}

	data, ok := str.Fields[3].(cadence.Array)
	if !ok {
		return nil, fmt.Errorf("invalid input: unexpected type for data field")
	}

	convertedData := make([]byte, len(data.Values))
	for i, value := range data.Values {
		convertedData[i] = value.(cadence.UInt8).ToGoValue().(uint8)
	}

	return &types.ResultSummary{
		Status:        types.Status(status),
		ErrorCode:     types.ErrorCode(errorCode),
		GasConsumed:   uint64(gasUsed),
		ReturnedValue: convertedData,
	}, nil

}
