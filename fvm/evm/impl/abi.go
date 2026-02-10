package impl

import (
	"math"
	"math/big"
	"reflect"
	"strings"

	gethABI "github.com/ethereum/go-ethereum/accounts/abi"
	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/errors"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/cadence/sema"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/fvm/evm/types"
)

const abiEncodingByteSize = 32

// abiEncodingError
type abiEncodingError struct {
	Type    interpreter.StaticType
	Message string
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

	message := e.Message
	if message != "" {
		b.WriteString(": ")
		b.WriteString(message)
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

type evmSpecialTypeIDs struct {
	AddressTypeID common.TypeID
	BytesTypeID   common.TypeID
	Bytes4TypeID  common.TypeID
	Bytes32TypeID common.TypeID
}

func NewEVMSpecialTypeIDs(
	gauge common.MemoryGauge,
	location common.AddressLocation,
) *evmSpecialTypeIDs {
	return &evmSpecialTypeIDs{
		AddressTypeID: location.TypeID(gauge, stdlib.EVMAddressTypeQualifiedIdentifier),
		BytesTypeID:   location.TypeID(gauge, stdlib.EVMBytesTypeQualifiedIdentifier),
		Bytes4TypeID:  location.TypeID(gauge, stdlib.EVMBytes4TypeQualifiedIdentifier),
		Bytes32TypeID: location.TypeID(gauge, stdlib.EVMBytes32TypeQualifiedIdentifier),
	}
}

type abiEncodingContext interface {
	interpreter.MemberAccessibleContext
	interpreter.ValueTransferContext
}

func reportArrayABIEncodingComputation(
	context abiEncodingContext,
	values *interpreter.ArrayValue,
	evmTypeIDs *evmSpecialTypeIDs,
	reportComputation func(intensity uint64),
) {
	values.Iterate(
		context,
		func(element interpreter.Value) (resume bool) {
			reportABIEncodingComputation(
				context,
				element,
				evmTypeIDs,
				reportComputation,
			)

			// continue iteration
			return true
		},
		false,
	)
}

func reportABIEncodingComputation(
	context abiEncodingContext,
	value interpreter.Value,
	evmTypeIDs *evmSpecialTypeIDs,
	reportComputation func(intensity uint64),
) {
	switch value := value.(type) {
	case *interpreter.StringValue:
		// Dynamic variables, such as strings, are encoded
		// in 2+ chunks of 32 bytes. The first chunk contains
		// the index where information for the string begin,
		// the second chunk contains the number of bytes the
		// string occupies, and the third chunk contains the
		// value of the string itself.
		computation := uint64(2 * abiEncodingByteSize)
		stringLength := len(value.Str)
		chunks := math.Ceil(float64(stringLength) / float64(abiEncodingByteSize))
		computation += uint64(chunks * abiEncodingByteSize)
		reportComputation(computation)

	case interpreter.BoolValue,
		interpreter.UIntValue,
		interpreter.UInt8Value,
		interpreter.UInt16Value,
		interpreter.UInt32Value,
		interpreter.UInt64Value,
		interpreter.UInt128Value,
		interpreter.UInt256Value,
		interpreter.IntValue,
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
		switch value.TypeID() {
		case evmTypeIDs.AddressTypeID:
			// EVM addresses are static variables with a fixed
			// size of 32 bytes.
			reportComputation(abiEncodingByteSize)

		case evmTypeIDs.BytesTypeID:
			computation := uint64(2 * abiEncodingByteSize)
			valueMember := value.GetMember(context, stdlib.EVMBytesTypeValueFieldName)
			bytesArray, ok := valueMember.(*interpreter.ArrayValue)
			if !ok {
				panic(abiEncodingError{
					Type:    value.StaticType(context),
					Message: "could not convert value field to array",
				})
			}
			bytesLength := bytesArray.Count()
			chunks := math.Ceil(float64(bytesLength) / float64(abiEncodingByteSize))
			computation += uint64(chunks * abiEncodingByteSize)
			reportComputation(computation)

		case evmTypeIDs.Bytes4TypeID:
			reportComputation(abiEncodingByteSize)

		case evmTypeIDs.Bytes32TypeID:
			reportComputation(abiEncodingByteSize)

		default:
			staticType := value.StaticType(context)
			semaType := interpreter.MustConvertStaticToSemaType(staticType, context)
			if compositeType := asTupleEncodableCompositeType(semaType); compositeType != nil {

				compositeType.Members.Foreach(func(name string, member *sema.Member) {
					if member.DeclarationKind != common.DeclarationKindField {
						return
					}

					fieldValue := value.GetMember(context, name)
					reportABIEncodingComputation(
						context,
						fieldValue,
						evmTypeIDs,
						reportComputation,
					)
				})

			} else {
				panic(abiEncodingError{
					Type: value.StaticType(context),
				})
			}
		}

	case *interpreter.ArrayValue:
		// Dynamic variables, such as arrays & slices, are encoded
		// in 2+ chunks of 32 bytes. The first chunk contains
		// the index where information for the array begin,
		// the second chunk contains the number of bytes the
		// array occupies, and the third chunk contains the
		// values of the array itself.
		computation := uint64(2 * abiEncodingByteSize)
		reportComputation(computation)
		reportArrayABIEncodingComputation(
			context,
			value,
			evmTypeIDs,
			reportComputation,
		)

	default:
		panic(abiEncodingError{
			Type: value.StaticType(context),
		})
	}
}

func newInternalEVMTypeEncodeABIFunction(
	gauge common.MemoryGauge,
	location common.AddressLocation,
) *interpreter.HostFunctionValue {

	evmSpecialTypeIDs := NewEVMSpecialTypeIDs(gauge, location)

	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeEncodeABIFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext

			// Get `values` argument

			valuesArray, ok := invocation.Arguments[0].(*interpreter.ArrayValue)
			if !ok {
				panic(errors.NewUnreachableError())
			}

			size := valuesArray.Count()

			values := make([]any, 0, size)
			arguments := make(gethABI.Arguments, 0, size)

			valuesArray.Iterate(
				context,
				func(element interpreter.Value) (resume bool) {

					reportABIEncodingComputation(
						context,
						element,
						evmSpecialTypeIDs,
						func(intensity uint64) {
							common.UseComputation(
								context,
								common.ComputationUsage{
									Kind:      environment.ComputationKindEVMEncodeABI,
									Intensity: intensity,
								},
							)
						},
					)

					value, ty, err := encodeABI(
						context,
						element,
						element.StaticType(context),
						evmSpecialTypeIDs,
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
			)

			encodedValues, err := arguments.Pack(values...)
			if err != nil {
				panic(
					abiEncodingError{
						Message: err.Error(),
					},
				)
			}

			return interpreter.ByteSliceToByteArrayValue(context, encodedValues)
		},
	)
}

var gethTypeString = gethABI.Type{T: gethABI.StringTy}

var gethTypeBool = gethABI.Type{T: gethABI.BoolTy}

var gethTypeUint = gethABI.Type{T: gethABI.UintTy, Size: 256}

var gethTypeUint8 = gethABI.Type{T: gethABI.UintTy, Size: 8}

var gethTypeUint16 = gethABI.Type{T: gethABI.UintTy, Size: 16}

var gethTypeUint32 = gethABI.Type{T: gethABI.UintTy, Size: 32}

var gethTypeUint64 = gethABI.Type{T: gethABI.UintTy, Size: 64}

var gethTypeUint128 = gethABI.Type{T: gethABI.UintTy, Size: 128}

var gethTypeUint256 = gethABI.Type{T: gethABI.UintTy, Size: 256}

var gethTypeInt = gethABI.Type{T: gethABI.IntTy, Size: 256}

var gethTypeInt8 = gethABI.Type{T: gethABI.IntTy, Size: 8}

var gethTypeInt16 = gethABI.Type{T: gethABI.IntTy, Size: 16}

var gethTypeInt32 = gethABI.Type{T: gethABI.IntTy, Size: 32}

var gethTypeInt64 = gethABI.Type{T: gethABI.IntTy, Size: 64}

var gethTypeInt128 = gethABI.Type{T: gethABI.IntTy, Size: 128}

var gethTypeInt256 = gethABI.Type{T: gethABI.IntTy, Size: 256}

var gethTypeAddress = gethABI.Type{T: gethABI.AddressTy, Size: 20}

var gethTypeBytes = gethABI.Type{T: gethABI.BytesTy}

var gethTypeBytes4 = gethABI.Type{T: gethABI.FixedBytesTy, Size: 4}

var gethTypeBytes32 = gethABI.Type{T: gethABI.FixedBytesTy, Size: 32}

func exportedName(name string) string {
	return strings.ToUpper(name[:1]) + name[1:]
}

func gethABIType(
	context abiEncodingContext,
	staticType interpreter.StaticType,
	evmTypeIDs *evmSpecialTypeIDs,
) (gethABI.Type, bool) {
	switch staticType {
	case interpreter.PrimitiveStaticTypeString:
		return gethTypeString, true
	case interpreter.PrimitiveStaticTypeBool:
		return gethTypeBool, true
	case interpreter.PrimitiveStaticTypeUInt:
		return gethTypeUint, true
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
	case interpreter.PrimitiveStaticTypeInt:
		return gethTypeInt, true
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
		switch staticType.TypeID {
		case evmTypeIDs.AddressTypeID:
			return gethTypeAddress, true
		case evmTypeIDs.BytesTypeID:
			return gethTypeBytes, true
		case evmTypeIDs.Bytes4TypeID:
			return gethTypeBytes4, true
		case evmTypeIDs.Bytes32TypeID:
			return gethTypeBytes32, true
		}

		semaType := interpreter.MustConvertStaticToSemaType(staticType, context)
		if compositeType := asTupleEncodableCompositeType(semaType); compositeType != nil {

			var (
				fieldTypeIsInvalid   bool
				goStructFields       []reflect.StructField
				gethABITupleElements []*gethABI.Type
				gethABITupleNames    []string
			)

			compositeType.Members.Foreach(func(name string, member *sema.Member) {
				if fieldTypeIsInvalid {
					return
				}

				if member.DeclarationKind != common.DeclarationKindField {
					return
				}

				fieldStaticType := interpreter.ConvertSemaToStaticType(
					context,
					member.TypeAnnotation.Type,
				)

				fieldGethABIType, ok := gethABIType(
					context,
					fieldStaticType,
					evmTypeIDs,
				)
				if !ok {
					fieldTypeIsInvalid = true
					return
				}

				gethABITupleElements = append(gethABITupleElements, &fieldGethABIType)

				// reflect.StructField.Name must be exported (start with uppercase)
				goStructFieldName := exportedName(name)

				gethABITupleNames = append(gethABITupleNames, goStructFieldName)

				goStructFields = append(
					goStructFields,
					reflect.StructField{
						Name: goStructFieldName,
						Type: fieldGethABIType.GetType(),
					},
				)
			})

			if fieldTypeIsInvalid {
				break
			}

			return gethABI.Type{
				T:             gethABI.TupleTy,
				TupleType:     reflect.StructOf(goStructFields),
				TupleElems:    gethABITupleElements,
				TupleRawNames: gethABITupleNames,
			}, true
		}

	case *interpreter.ConstantSizedStaticType:
		elementGethABIType, ok := gethABIType(
			context,
			staticType.ElementType(),
			evmTypeIDs,
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
			context,
			staticType.ElementType(),
			evmTypeIDs,
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

var (
	goStringType    = reflect.TypeFor[string]()
	goBoolType      = reflect.TypeFor[bool]()
	goUint8Type     = reflect.TypeFor[uint8]()
	goUint16Type    = reflect.TypeFor[uint16]()
	goUint32Type    = reflect.TypeFor[uint32]()
	goUint64Type    = reflect.TypeFor[uint64]()
	goInt8Type      = reflect.TypeFor[int8]()
	goInt16Type     = reflect.TypeFor[int16]()
	goInt32Type     = reflect.TypeFor[int32]()
	goInt64Type     = reflect.TypeFor[int64]()
	goBigIntType    = reflect.TypeFor[*big.Int]()
	gethAddressType = reflect.TypeFor[gethCommon.Address]()
	goByteSliceType = reflect.TypeFor[[]byte]()
	evmBytes4Type   = reflect.TypeFor[[stdlib.EVMBytes4Length]byte]()
	evmBytes32Type  = reflect.TypeFor[[stdlib.EVMBytes32Length]byte]()
)

func goType(
	context abiEncodingContext,
	staticType interpreter.StaticType,
	evmTypeIDs *evmSpecialTypeIDs,
) (reflect.Type, bool) {
	switch staticType {
	case interpreter.PrimitiveStaticTypeString:
		return goStringType, true
	case interpreter.PrimitiveStaticTypeBool:
		return goBoolType, true
	case interpreter.PrimitiveStaticTypeUInt:
		return goBigIntType, true
	case interpreter.PrimitiveStaticTypeUInt8:
		return goUint8Type, true
	case interpreter.PrimitiveStaticTypeUInt16:
		return goUint16Type, true
	case interpreter.PrimitiveStaticTypeUInt32:
		return goUint32Type, true
	case interpreter.PrimitiveStaticTypeUInt64:
		return goUint64Type, true
	case interpreter.PrimitiveStaticTypeUInt128:
		return goBigIntType, true
	case interpreter.PrimitiveStaticTypeUInt256:
		return goBigIntType, true
	case interpreter.PrimitiveStaticTypeInt:
		return goBigIntType, true
	case interpreter.PrimitiveStaticTypeInt8:
		return goInt8Type, true
	case interpreter.PrimitiveStaticTypeInt16:
		return goInt16Type, true
	case interpreter.PrimitiveStaticTypeInt32:
		return goInt32Type, true
	case interpreter.PrimitiveStaticTypeInt64:
		return goInt64Type, true
	case interpreter.PrimitiveStaticTypeInt128:
		return goBigIntType, true
	case interpreter.PrimitiveStaticTypeInt256:
		return goBigIntType, true
	case interpreter.PrimitiveStaticTypeAddress:
		return goBigIntType, true
	}

	switch staticType := staticType.(type) {
	case *interpreter.ConstantSizedStaticType:
		elementType, ok := goType(context, staticType.ElementType(), evmTypeIDs)
		if !ok {
			break
		}

		return reflect.ArrayOf(int(staticType.Size), elementType), true

	case *interpreter.VariableSizedStaticType:
		elementType, ok := goType(context, staticType.ElementType(), evmTypeIDs)
		if !ok {
			break
		}

		return reflect.SliceOf(elementType), true
	}

	switch staticType.ID() {
	case evmTypeIDs.AddressTypeID:
		return gethAddressType, true
	case evmTypeIDs.BytesTypeID:
		return goByteSliceType, true
	case evmTypeIDs.Bytes4TypeID:
		return evmBytes4Type, true
	case evmTypeIDs.Bytes32TypeID:
		return evmBytes32Type, true
	}

	gethABIType, ok := gethABIType(
		context,
		staticType,
		evmTypeIDs,
	)
	// All user-defined Cadence structs, are ABI encoded/decoded as Solidity tuples.
	// Except for the structs defined in the EVM system contract:
	// - `EVM.EVMAddress`
	// - `EVM.EVMBytes`
	// - `EVM.EVMBytes4`
	// - `EVM.EVMBytes32`
	// These have their own ABI encoding/decoding format.
	if ok && gethABIType.T == gethABI.TupleTy {
		return gethABIType.TupleType, true
	}

	return nil, false
}

func encodeABI(
	context abiEncodingContext,
	value interpreter.Value,
	staticType interpreter.StaticType,
	evmTypeIDs *evmSpecialTypeIDs,
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

	case interpreter.UIntValue:
		if staticType == interpreter.PrimitiveStaticTypeUInt {
			if value.BigInt.Cmp(sema.UInt256TypeMaxIntBig) > 0 || value.BigInt.Cmp(sema.UInt256TypeMinIntBig) < 0 {
				return nil, gethABI.Type{}, abiEncodingError{
					Type:    value.StaticType(context),
					Message: "value outside the boundaries of uint256",
				}
			}
			return value.BigInt, gethTypeUint, nil
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

	case interpreter.IntValue:
		if staticType == interpreter.PrimitiveStaticTypeInt {
			if value.BigInt.Cmp(sema.Int256TypeMaxIntBig) > 0 || value.BigInt.Cmp(sema.Int256TypeMinIntBig) < 0 {
				return nil, gethABI.Type{}, abiEncodingError{
					Type:    value.StaticType(context),
					Message: "value outside the boundaries of int256",
				}
			}
			return value.BigInt, gethTypeInt, nil
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
		switch value.TypeID() {
		case evmTypeIDs.AddressTypeID:
			addressBytesArrayValue := value.GetMember(context, stdlib.EVMAddressTypeBytesFieldName)
			bytes, err := interpreter.ByteArrayValueToByteSlice(context, addressBytesArrayValue)
			if err != nil {
				panic(err)
			}
			return gethCommon.Address(bytes), gethTypeAddress, nil

		case evmTypeIDs.BytesTypeID:
			bytesValue := value.GetMember(context, stdlib.EVMBytesTypeValueFieldName)
			bytes, err := interpreter.ByteArrayValueToByteSlice(context, bytesValue)
			if err != nil {
				panic(err)
			}
			return bytes, gethTypeBytes, nil

		case evmTypeIDs.Bytes4TypeID:
			bytesValue := value.GetMember(context, stdlib.EVMBytesTypeValueFieldName)
			bytes, err := interpreter.ByteArrayValueToByteSlice(context, bytesValue)
			if err != nil {
				panic(err)
			}
			return [stdlib.EVMBytes4Length]byte(bytes), gethTypeBytes4, nil

		case evmTypeIDs.Bytes32TypeID:
			bytesValue := value.GetMember(context, stdlib.EVMBytesTypeValueFieldName)
			bytes, err := interpreter.ByteArrayValueToByteSlice(context, bytesValue)
			if err != nil {
				panic(err)
			}
			return [stdlib.EVMBytes32Length]byte(bytes), gethTypeBytes32, nil
		}

		staticType := value.StaticType(context)
		semaType := interpreter.MustConvertStaticToSemaType(staticType, context)

		if compositeType := asTupleEncodableCompositeType(semaType); compositeType != nil {

			tupleGethABIType, ok := gethABIType(
				context,
				staticType,
				evmTypeIDs,
			)
			if !ok {
				break
			}

			result := reflect.New(tupleGethABIType.TupleType)

			var index int

			compositeType.Members.Foreach(func(name string, member *sema.Member) {

				if member.DeclarationKind != common.DeclarationKindField {
					return
				}

				goStructFieldName := tupleGethABIType.TupleRawNames[index]

				if exportedName(name) != goStructFieldName {
					// Continue to next member
					return
				}

				index++

				fieldValue := value.GetMember(context, name)

				fieldElement, _, err := encodeABI(
					context,
					fieldValue,
					fieldValue.StaticType(context),
					evmTypeIDs,
				)
				if err != nil {
					panic(err)
				}

				field := result.Elem().FieldByName(goStructFieldName)
				field.Set(reflect.ValueOf(fieldElement))
			})

			return result.Interface(), tupleGethABIType, nil
		}

	case *interpreter.ArrayValue:
		arrayStaticType := value.Type

		arrayGethABIType, ok := gethABIType(
			context,
			arrayStaticType,
			evmTypeIDs,
		)
		if !ok {
			break
		}

		elementStaticType := arrayStaticType.ElementType()

		elementGoType, ok := goType(context, elementStaticType, evmTypeIDs)
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

		semaType := interpreter.MustConvertStaticToSemaType(elementStaticType, context)
		isTuple := asTupleEncodableCompositeType(semaType) != nil

		var index int
		value.Iterate(
			context,
			func(element interpreter.Value) (resume bool) {

				arrayElement, _, err := encodeABI(
					context,
					element,
					element.StaticType(context),
					evmTypeIDs,
				)
				if err != nil {
					panic(err)
				}

				if isTuple {
					// For tuples, the underlying `arrayElement` is a value of
					// type *struct { X,Y,Z fields }, so we need to indirect
					// the pointer
					result.Index(index).Set(
						reflect.Indirect(reflect.ValueOf(arrayElement)),
					)
				} else {
					result.Index(index).Set(reflect.ValueOf(arrayElement))
				}

				index++

				// continue iteration
				return true
			},
			false,
		)

		return result.Interface(), arrayGethABIType, nil
	}

	return nil, gethABI.Type{}, abiEncodingError{
		Type: value.StaticType(context),
	}
}

// asTupleEncodableCompositeType determines if the given type can be encoded as a tuple
// (when the type is user-defined (location != nil) struct type)
func asTupleEncodableCompositeType(ty sema.Type) *sema.CompositeType {
	compositeType, ok := ty.(*sema.CompositeType)
	if !ok ||
		compositeType.Location == nil ||
		compositeType.IsResourceType() {

		return nil
	}

	return compositeType
}

func newInternalEVMTypeDecodeABIFunction(
	gauge common.MemoryGauge,
	location common.AddressLocation,
) *interpreter.HostFunctionValue {

	evmSpecialTypeIDs := NewEVMSpecialTypeIDs(gauge, location)

	return interpreter.NewStaticHostFunctionValue(
		gauge,
		stdlib.InternalEVMTypeDecodeABIFunctionType,
		func(invocation interpreter.Invocation) interpreter.Value {
			context := invocation.InvocationContext

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

			common.UseComputation(
				context,
				common.ComputationUsage{
					Kind:      environment.ComputationKindEVMDecodeABI,
					Intensity: uint64(dataValue.Count()),
				},
			)

			data, err := interpreter.ByteArrayValueToByteSlice(context, dataValue)
			if err != nil {
				panic(err)
			}

			arguments := make(gethABI.Arguments, 0, typesArray.Count())
			staticTypes := make([]interpreter.StaticType, 0, typesArray.Count())
			typesArray.Iterate(
				context,
				func(element interpreter.Value) (resume bool) {
					typeValue, ok := element.(interpreter.TypeValue)
					if !ok {
						panic(errors.NewUnreachableError())
					}

					staticType := typeValue.Type

					gethABITy, ok := gethABIType(
						context,
						staticType,
						evmSpecialTypeIDs,
					)
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

					staticTypes = append(staticTypes, staticType)

					// continue iteration
					return true
				},
				false,
			)

			decodedValues, err := arguments.Unpack(data)
			if err != nil {
				panic(abiDecodingError{})
			}

			values := make([]interpreter.Value, 0, len(decodedValues))

			for i, staticType := range staticTypes {
				value, err := decodeABI(
					context,
					decodedValues[i],
					staticType,
					location,
					evmSpecialTypeIDs,
				)
				if err != nil {
					panic(err)
				}

				values = append(values, value)
			}

			arrayType := interpreter.NewVariableSizedStaticType(
				context,
				interpreter.NewPrimitiveStaticType(
					context,
					interpreter.PrimitiveStaticTypeAnyStruct,
				),
			)

			return interpreter.NewArrayValue(
				context,
				arrayType,
				common.ZeroAddress,
				values...,
			)
		},
	)
}

type memberAccessibleArrayCreationContext interface {
	interpreter.MemberAccessibleContext
	interpreter.ArrayCreationContext
}

func decodeABI(
	context memberAccessibleArrayCreationContext,
	value any,
	staticType interpreter.StaticType,
	location common.AddressLocation,
	evmTypeIDs *evmSpecialTypeIDs,
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
			context,
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

	case interpreter.PrimitiveStaticTypeUInt:
		value, ok := value.(*big.Int)
		if !ok {
			break
		}
		memoryUsage := common.NewBigIntMemoryUsage(
			common.BigIntByteLength(value),
		)
		return interpreter.NewUIntValueFromBigInt(context, memoryUsage, func() *big.Int { return value }), nil

	case interpreter.PrimitiveStaticTypeUInt8:
		value, ok := value.(uint8)
		if !ok {
			break
		}
		return interpreter.NewUInt8Value(context, func() uint8 { return value }), nil

	case interpreter.PrimitiveStaticTypeUInt16:
		value, ok := value.(uint16)
		if !ok {
			break
		}
		return interpreter.NewUInt16Value(context, func() uint16 { return value }), nil

	case interpreter.PrimitiveStaticTypeUInt32:
		value, ok := value.(uint32)
		if !ok {
			break
		}
		return interpreter.NewUInt32Value(context, func() uint32 { return value }), nil

	case interpreter.PrimitiveStaticTypeUInt64:
		value, ok := value.(uint64)
		if !ok {
			break
		}
		return interpreter.NewUInt64Value(context, func() uint64 { return value }), nil

	case interpreter.PrimitiveStaticTypeUInt128:
		value, ok := value.(*big.Int)
		if !ok {
			break
		}
		return interpreter.NewUInt128ValueFromBigInt(context, func() *big.Int { return value }), nil

	case interpreter.PrimitiveStaticTypeUInt256:
		value, ok := value.(*big.Int)
		if !ok {
			break
		}
		return interpreter.NewUInt256ValueFromBigInt(context, func() *big.Int { return value }), nil

	case interpreter.PrimitiveStaticTypeInt:
		value, ok := value.(*big.Int)
		if !ok {
			break
		}
		memoryUsage := common.NewBigIntMemoryUsage(
			common.BigIntByteLength(value),
		)
		return interpreter.NewIntValueFromBigInt(context, memoryUsage, func() *big.Int { return value }), nil

	case interpreter.PrimitiveStaticTypeInt8:
		value, ok := value.(int8)
		if !ok {
			break
		}
		return interpreter.NewInt8Value(context, func() int8 { return value }), nil

	case interpreter.PrimitiveStaticTypeInt16:
		value, ok := value.(int16)
		if !ok {
			break
		}
		return interpreter.NewInt16Value(context, func() int16 { return value }), nil

	case interpreter.PrimitiveStaticTypeInt32:
		value, ok := value.(int32)
		if !ok {
			break
		}
		return interpreter.NewInt32Value(context, func() int32 { return value }), nil

	case interpreter.PrimitiveStaticTypeInt64:
		value, ok := value.(int64)
		if !ok {
			break
		}
		return interpreter.NewInt64Value(context, func() int64 { return value }), nil

	case interpreter.PrimitiveStaticTypeInt128:
		value, ok := value.(*big.Int)
		if !ok {
			break
		}
		return interpreter.NewInt128ValueFromBigInt(context, func() *big.Int { return value }), nil

	case interpreter.PrimitiveStaticTypeInt256:
		value, ok := value.(*big.Int)
		if !ok {
			break
		}
		return interpreter.NewInt256ValueFromBigInt(context, func() *big.Int { return value }), nil
	}

	switch staticType := staticType.(type) {
	case interpreter.ArrayStaticType:
		array := reflect.ValueOf(value)

		elementStaticType := staticType.ElementType()

		size := array.Len()

		var index int
		return interpreter.NewArrayValueWithIterator(
			context,
			staticType,
			common.ZeroAddress,
			uint64(size),
			func() interpreter.Value {
				if index >= size {
					return nil
				}

				element := array.Index(index).Interface()

				result, err := decodeABI(
					context,
					element,
					elementStaticType,
					location,
					evmTypeIDs,
				)
				if err != nil {
					panic(err)
				}

				index++

				return result
			},
		), nil

	case *interpreter.CompositeStaticType:
		switch staticType.TypeID {
		case evmTypeIDs.AddressTypeID:
			addr, ok := value.(gethCommon.Address)
			if !ok {
				break
			}

			var address types.Address
			copy(address[:], addr.Bytes())
			return NewEVMAddress(
				context,
				location,
				address,
			), nil

		case evmTypeIDs.BytesTypeID:
			bytes, ok := value.([]byte)
			if !ok {
				break
			}
			return NewEVMBytes(
				context,
				location,
				bytes,
			), nil

		case evmTypeIDs.Bytes4TypeID:
			bytes, ok := value.([stdlib.EVMBytes4Length]byte)
			if !ok {
				break
			}
			return NewEVMBytes4(
				context,
				location,
				bytes,
			), nil

		case evmTypeIDs.Bytes32TypeID:
			bytes, ok := value.([stdlib.EVMBytes32Length]byte)
			if !ok {
				break
			}
			return NewEVMBytes32(
				context,
				location,
				bytes,
			), nil

		default:
			semaType := interpreter.MustConvertStaticToSemaType(staticType, context)
			if compositeType := asTupleEncodableCompositeType(semaType); compositeType != nil {

				valueStruct := reflect.ValueOf(value)

				fields := make([]interpreter.CompositeField, 0, compositeType.Members.Len())

				compositeType.Members.Foreach(func(name string, member *sema.Member) {
					if member.DeclarationKind != common.DeclarationKindField {
						return
					}

					fieldValue := valueStruct.FieldByName(exportedName(name)).Interface()

					fieldStaticType := interpreter.ConvertSemaToStaticType(
						context,
						member.TypeAnnotation.Type,
					)

					decodedFieldValue, err := decodeABI(
						context,
						fieldValue,
						fieldStaticType,
						location,
						evmTypeIDs,
					)
					if err != nil {
						panic(err)
					}

					field := interpreter.NewCompositeField(context, name, decodedFieldValue)
					fields = append(fields, field)
				})

				return interpreter.NewCompositeValue(
					context,
					compositeType.Location,
					compositeType.QualifiedIdentifier(),
					compositeType.Kind,
					fields,
					common.ZeroAddress,
				), nil
			}
		}
	}

	return nil, abiDecodingError{
		Type: staticType,
	}
}
