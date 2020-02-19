package encoding

import (
	"bytes"
	"fmt"
	"io"
	"math/big"

	xdr "github.com/davecgh/go-xdr/xdr2"

	"github.com/dapperlabs/flow-go/language"
)

// A Decoder decodes XDR-encoded representations of Cadence values.
type Decoder struct {
	dec *xdr.Decoder
}

// Decode returns a Cadence value decoded from its XDR-encoded representation.
//
// This function returns an error if the bytes do not match the given type
// definition.
func Decode(t language.Type, b []byte) (language.Value, error) {
	r := bytes.NewReader(b)
	dec := NewDecoder(r)

	v, err := dec.Decode(t)
	if err != nil {
		return nil, err
	}

	return v, nil
}

// NewDecoder initializes a Decoder that will decode XDR-encoded bytes from the
// given io.Reader.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{xdr.NewDecoder(r)}
}

// Decode reads XDR-encoded bytes from the io.Reader and decodes them to a
// Cadence value of the given type.
//
// This function returns an error if the bytes do not match the given type
// definition.
func (d *Decoder) Decode(t language.Type) (language.Value, error) {
	switch x := t.(type) {
	case language.VoidType:
		return d.DecodeVoid()
	case language.OptionalType:
		return d.DecodeOptional(x)
	case language.BoolType:
		return d.DecodeBool()
	case language.StringType:
		return d.DecodeString()
	case language.BytesType:
		return d.DecodeBytes()
	case language.AddressType:
		return d.DecodeAddress()
	case language.IntType:
		return d.DecodeInt()
	case language.Int8Type:
		return d.DecodeInt8()
	case language.Int16Type:
		return d.DecodeInt16()
	case language.Int32Type:
		return d.DecodeInt32()
	case language.Int64Type:
		return d.DecodeInt64()
	case language.Int128Type:
		return d.DecodeInt128()
	case language.Int256Type:
		return d.DecodeInt256()
	case language.UIntType:
		return d.DecodeUInt()
	case language.UInt8Type:
		return d.DecodeUInt8()
	case language.UInt16Type:
		return d.DecodeUInt16()
	case language.UInt32Type:
		return d.DecodeUInt32()
	case language.UInt64Type:
		return d.DecodeUInt64()
	case language.UInt128Type:
		return d.DecodeUInt128()
	case language.UInt256Type:
		return d.DecodeUInt256()
	case language.Word8Type:
		return d.DecodeWord8()
	case language.Word16Type:
		return d.DecodeWord16()
	case language.Word32Type:
		return d.DecodeWord32()
	case language.Word64Type:
		return d.DecodeWord64()
	case language.Fix64Type:
		return d.DecodeFix64()
	case language.UFix64Type:
		return d.DecodeUFix64()
	case language.VariableSizedArrayType:
		return d.DecodeVariableSizedArray(x)
	case language.ConstantSizedArrayType:
		return d.DecodeConstantSizedArray(x)
	case language.DictionaryType:
		return d.DecodeDictionary(x)
	case language.CompositeType:
		return d.DecodeComposite(x)
	case language.ResourceType:
		return d.DecodeComposite(x.CompositeType)
	case language.StructType:
		return d.DecodeComposite(x.CompositeType)
	case language.EventType:
		return d.DecodeComposite(x.CompositeType)

	default:
		return nil, fmt.Errorf("unsupported type: %T", t)
	}
}

// DecodeVoid reads the XDR-encoded representation of a void value.
//
// VoidType values are skipped by the decoder because they are empty by
// definition, but this function still exists in order to support composite
// types that contain void values.
func (d *Decoder) DecodeVoid() (language.Void, error) {
	// void values are not encoded
	return language.Void{}, nil
}

// DecodeOptional reads the XDR-encoded representation of an optional value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.19
//  RFC Section 4.19 - Optional-Data
//  Union of boolean and encoded value
func (d *Decoder) DecodeOptional(t language.OptionalType) (v language.Optional, err error) {
	hasValue, err := d.DecodeBool()
	if err != nil {
		return v, err
	}

	if hasValue {
		value, err := d.Decode(t.Type)
		if err != nil {
			return v, err
		}

		return language.Optional{Value: value}, nil
	}

	return language.Optional{Value: nil}, nil
}

// DecodeBool reads the XDR-encoded representation of a boolean value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.4
//  RFC Section 4.4 - Boolean
//  Represented as an XDR encoded enumeration where 0 is false and 1 is true
func (d *Decoder) DecodeBool() (v language.Bool, err error) {
	b, _, err := d.dec.DecodeBool()
	if err != nil {
		return v, err
	}

	return language.NewBool(b), nil
}

// DecodeString reads the XDR-encoded representation of a string value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.11
//  RFC Section 4.11 - StringType
//  Unsigned integer length followed by bytes zero-padded to a multiple of four
func (d *Decoder) DecodeString() (v language.String, err error) {
	str, _, err := d.dec.DecodeString()
	if err != nil {
		return v, err
	}

	return language.NewString(str), nil
}

// DecodeBytes reads the XDR-encoded representation of a byte array.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.10
//  RFC Section 4.10 - Variable-Length Opaque Data
//  Unsigned integer length followed by fixed opaque data of that length
func (d *Decoder) DecodeBytes() (v language.Bytes, err error) {
	b, _, err := d.dec.DecodeOpaque()
	if err != nil {
		return v, err
	}

	if b == nil {
		b = []byte{}
	}

	return language.NewBytes(b), nil
}

// DecodeAddress reads the XDR-encoded representation of an address.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.9
//  RFC Section 4.9 - Fixed-Length Opaque Data
//  Fixed-length uninterpreted data zero-padded to a multiple of four
func (d *Decoder) DecodeAddress() (v language.Address, err error) {
	b, _, err := d.dec.DecodeFixedOpaque(20)
	if err != nil {
		return v, err
	}

	return language.NewAddressFromBytes(b), nil
}

// DecodeInt reads the XDR-encoded representation of an arbitrary-precision
// integer value.
//
// An arbitrary-precision integer is encoded as follows:
//   Sign as a byte flag (positive is 1, negative is 0)
//   Absolute value as variable-length big-endian byte array
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.10
//  RFC Section 4.10 - Variable-Length Opaque Data
//  Unsigned integer length followed by fixed opaque data of that length
func (d *Decoder) DecodeInt() (v language.Int, err error) {
	i, err := d.decodeBig()
	if err != nil {
		return v, err
	}
	return language.NewIntFromBig(i), nil
}

// decodeBig reads the XDR-encoded representation of an arbitrary-precision
// integer value.
//
// An arbitrary-precision integer is encoded as follows:
//   Sign as a byte flag (positive is 1, negative is 0)
//   Absolute value as variable-length big-endian byte array
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.10
//  RFC Section 4.10 - Variable-Length Opaque Data
//  Unsigned integer length followed by fixed opaque data of that length
//
func (d *Decoder) decodeBig() (i *big.Int, err error) {

	b, _, err := d.dec.DecodeOpaque()
	if err != nil {
		return i, err
	}

	isPositive := b[0] == 1

	i = big.NewInt(0).SetBytes(b[1:])

	if !isPositive {
		i = i.Neg(i)
	}

	return i, nil
}

// DecodeInt8 reads the XDR-encoded representation of an int-8 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.1
//  RFC Section 4.1 - Integer
//  32-bit big-endian signed integer in range [-2147483648, 2147483647]
func (d *Decoder) DecodeInt8() (v language.Int8, err error) {
	i, _, err := d.dec.DecodeInt()
	if err != nil {
		return v, err
	}

	return language.NewInt8(int8(i)), nil
}

// DecodeInt16 reads the XDR-encoded representation of an int-16 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.1
//  RFC Section 4.1 - Integer
//  32-bit big-endian signed integer in range [-2147483648, 2147483647]
func (d *Decoder) DecodeInt16() (v language.Int16, err error) {
	i, _, err := d.dec.DecodeInt()
	if err != nil {
		return v, err
	}

	return language.NewInt16(int16(i)), nil
}

// DecodeInt32 reads the XDR-encoded representation of an int-32 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.1
//  RFC Section 4.1 - Integer
//  32-bit big-endian signed integer in range [-2147483648, 2147483647]
func (d *Decoder) DecodeInt32() (v language.Int32, err error) {
	i, _, err := d.dec.DecodeInt()
	if err != nil {
		return v, err
	}

	return language.NewInt32(i), nil
}

// DecodeInt64 reads the XDR-encoded representation of an int-64 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.5
//  RFC Section 4.5 - Hyper Integer
//  64-bit big-endian signed integer in range [-9223372036854775808, 9223372036854775807]
func (d *Decoder) DecodeInt64() (v language.Int64, err error) {
	i, _, err := d.dec.DecodeHyper()
	if err != nil {
		return v, err
	}

	return language.NewInt64(i), nil
}

// DecodeInt128 reads the XDR-encoded representation of an arbitrary-precision
// integer value.
//
// An arbitrary-precision integer is encoded as follows:
//   Sign as a byte flag (positive is 1, negative is 0)
//   Absolute value as variable-length big-endian byte array
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.10
//  RFC Section 4.10 - Variable-Length Opaque Data
//  Unsigned integer length followed by fixed opaque data of that length
func (d *Decoder) DecodeInt128() (v language.Int128, err error) {
	i, err := d.decodeBig()
	if err != nil {
		return v, err
	}
	return language.NewInt128FromBig(i), nil
}

// DecodeInt256 reads the XDR-encoded representation of an arbitrary-precision
// integer value.
//
// An arbitrary-precision integer is encoded as follows:
//   Sign as a byte flag (positive is 1, negative is 0)
//   Absolute value as variable-length big-endian byte array
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.10
//  RFC Section 4.10 - Variable-Length Opaque Data
//  Unsigned integer length followed by fixed opaque data of that length
func (d *Decoder) DecodeInt256() (v language.Int256, err error) {
	i, err := d.decodeBig()
	if err != nil {
		return v, err
	}
	return language.NewInt256FromBig(i), nil
}

// DecodeUInt reads the XDR-encoded representation of an arbitrary-precision
// integer value.
//
// An arbitrary-precision integer is encoded as follows:
//   Sign as a byte flag (positive is 1, negative is 0)
//   Absolute value as variable-length big-endian byte array
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.10
//  RFC Section 4.10 - Variable-Length Opaque Data
//  Unsigned integer length followed by fixed opaque data of that length
func (d *Decoder) DecodeUInt() (v language.UInt, err error) {
	i, err := d.decodeBig()
	if err != nil {
		return v, err
	}
	return language.NewUIntFromBig(i), nil
}

// DecodeUInt8 reads the XDR-encoded representation of a uint-8 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.2
//  RFC Section 4.2 - Unsigned Integer
//  32-bit big-endian unsigned integer in range [0, 4294967295]
func (d *Decoder) DecodeUInt8() (v language.UInt8, err error) {
	i, _, err := d.dec.DecodeUint()
	if err != nil {
		return v, err
	}

	return language.NewUInt8(uint8(i)), nil
}

// DecodeUInt16 reads the XDR-encoded representation of a uint-16 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.2
//  RFC Section 4.2 - Unsigned Integer
//  32-bit big-endian unsigned integer in range [0, 4294967295]
func (d *Decoder) DecodeUInt16() (v language.UInt16, err error) {
	i, _, err := d.dec.DecodeUint()
	if err != nil {
		return v, err
	}

	return language.NewUInt16(uint16(i)), nil
}

// DecodeUInt32 reads the XDR-encoded representation of a uint-32 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.2
//  RFC Section 4.2 - Unsigned Integer
//  32-bit big-endian unsigned integer in range [0, 4294967295]
func (d *Decoder) DecodeUInt32() (v language.UInt32, err error) {
	i, _, err := d.dec.DecodeUint()
	if err != nil {
		return v, err
	}

	return language.NewUInt32(i), nil
}

// DecodeUInt64 reads the XDR-encoded representation of a uint-64 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.5
//  RFC Section 4.5 - Unsigned Hyper Integer
//  64-bit big-endian unsigned integer in range [0, 18446744073709551615]
func (d *Decoder) DecodeUInt64() (v language.UInt64, err error) {
	i, _, err := d.dec.DecodeUhyper()
	if err != nil {
		return v, err
	}

	return language.NewUInt64(i), nil
}

// DecodeUInt128 reads the XDR-encoded representation of an arbitrary-precision
// integer value.
//
// An arbitrary-precision integer is encoded as follows:
//   Sign as a byte flag (positive is 1, negative is 0)
//   Absolute value as variable-length big-endian byte array
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.10
//  RFC Section 4.10 - Variable-Length Opaque Data
//  Unsigned integer length followed by fixed opaque data of that length
func (d *Decoder) DecodeUInt128() (v language.UInt128, err error) {
	i, err := d.decodeBig()
	if err != nil {
		return v, err
	}
	return language.NewUInt128FromBig(i), nil
}

// DecodeUInt256 reads the XDR-encoded representation of an arbitrary-precision
// integer value.
//
// An arbitrary-precision integer is encoded as follows:
//   Sign as a byte flag (positive is 1, negative is 0)
//   Absolute value as variable-length big-endian byte array
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.10
//  RFC Section 4.10 - Variable-Length Opaque Data
//  Unsigned integer length followed by fixed opaque data of that length
func (d *Decoder) DecodeUInt256() (v language.UInt256, err error) {
	i, err := d.decodeBig()
	if err != nil {
		return v, err
	}
	return language.NewUInt256FromBig(i), nil
}

// DecodeWord8 reads the XDR-encoded representation of a uint-8 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.2
//  RFC Section 4.2 - Unsigned Integer
//  32-bit big-endian unsigned integer in range [0, 4294967295]
func (d *Decoder) DecodeWord8() (v language.Word8, err error) {
	i, _, err := d.dec.DecodeUint()
	if err != nil {
		return v, err
	}

	return language.NewWord8(uint8(i)), nil
}

// DecodeWord16 reads the XDR-encoded representation of a uint-16 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.2
//  RFC Section 4.2 - Unsigned Integer
//  32-bit big-endian unsigned integer in range [0, 4294967295]
func (d *Decoder) DecodeWord16() (v language.Word16, err error) {
	i, _, err := d.dec.DecodeUint()
	if err != nil {
		return v, err
	}

	return language.NewWord16(uint16(i)), nil
}

// DecodeWord32 reads the XDR-encoded representation of a uint-32 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.2
//  RFC Section 4.2 - Unsigned Integer
//  32-bit big-endian unsigned integer in range [0, 4294967295]
func (d *Decoder) DecodeWord32() (v language.Word32, err error) {
	i, _, err := d.dec.DecodeUint()
	if err != nil {
		return v, err
	}

	return language.NewWord32(i), nil
}

// DecodeWord64 reads the XDR-encoded representation of a uint-64 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.5
//  RFC Section 4.5 - Unsigned Hyper Integer
//  64-bit big-endian unsigned integer in range [0, 18446744073709551615]
func (d *Decoder) DecodeWord64() (v language.Word64, err error) {
	i, _, err := d.dec.DecodeUhyper()
	if err != nil {
		return v, err
	}

	return language.NewWord64(i), nil
}

// DecodeFix64 reads the XDR-encoded representation of an int-64 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.5
//  RFC Section 4.5 - Hyper Integer
//  64-bit big-endian signed integer in range [-9223372036854775808, 9223372036854775807]
func (d *Decoder) DecodeFix64() (v language.Fix64, err error) {
	i, _, err := d.dec.DecodeHyper()
	if err != nil {
		return v, err
	}

	return language.NewFix64(i), nil
}

// DecodeUFix64 reads the XDR-encoded representation of a uint-64 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.5
//  RFC Section 4.5 - Unsigned Hyper Integer
//  64-bit big-endian unsigned integer in range [0, 18446744073709551615]
func (d *Decoder) DecodeUFix64() (v language.UFix64, err error) {
	i, _, err := d.dec.DecodeUhyper()
	if err != nil {
		return v, err
	}

	return language.NewUFix64(i), nil
}

// DecodeVariableSizedArray reads the XDR-encoded representation of a
// variable-sized array.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.13
//  RFC Section 4.13 - Variable-Length Array
//  Unsigned integer length followed by individually XDR-encoded array elements
func (d *Decoder) DecodeVariableSizedArray(t language.VariableSizedArrayType) (v language.VariableSizedArray, err error) {
	size, _, err := d.dec.DecodeUint()
	if err != nil {
		return v, err
	}

	vals, err := d.decodeArray(t.ElementType, uint(size))
	if err != nil {
		return v, err
	}

	return language.NewVariableSizedArray(vals), nil
}

// DecodeConstantSizedArray reads the XDR-encoded representation of a
// constant-sized array.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.12
//  RFC Section 4.12 - Fixed-Length Array
//  Individually XDR-encoded array elements
func (d *Decoder) DecodeConstantSizedArray(t language.ConstantSizedArrayType) (v language.ConstantSizedArray, err error) {
	vals, err := d.decodeArray(t.ElementType, t.Size)
	if err != nil {
		return v, err
	}

	return language.NewConstantSizedArray(vals), nil
}

// decodeArray reads the XDR-encoded representation of a constant-sized array.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.12
//  RFC Section 4.12 - Fixed-Length Array
//  Individually XDR-encoded array elements
func (d *Decoder) decodeArray(t language.Type, size uint) ([]language.Value, error) {
	array := make([]language.Value, size)

	var i uint
	for i = 0; i < size; i++ {
		value, err := d.Decode(t)
		if err != nil {
			return nil, err
		}

		array[i] = value
	}

	return array, nil
}

// DecodeDictionary reads the XDR-encoded representation of a dictionary.
//
// The size of the dictionary is encoded as an unsigned integer, followed by
// the dictionary keys, then elements, each represented as individually
// XDR-encoded array elements.
func (d *Decoder) DecodeDictionary(t language.DictionaryType) (v language.Dictionary, err error) {
	size, _, err := d.dec.DecodeUint()
	if err != nil {
		return v, err
	}

	keys, err := d.decodeArray(t.KeyType, uint(size))
	if err != nil {
		return v, err
	}

	elements, err := d.decodeArray(t.ElementType, uint(size))
	if err != nil {
		return v, err
	}

	pairs := make([]language.KeyValuePair, size)

	for i := 0; i < int(size); i++ {
		key := keys[i]
		element := elements[i]

		pairs[i] = language.KeyValuePair{
			Key:   key,
			Value: element,
		}
	}

	return language.NewDictionary(pairs), nil
}

// DecodeComposite reads the XDR-encoded representation of a composite value.
//
// A composite is encoded as a fixed-length array of its field values.
func (d *Decoder) DecodeComposite(t language.CompositeType) (v language.Composite, err error) {
	vals := make([]language.Value, len(t.Fields))

	for i, field := range t.Fields {
		value, err := d.Decode(field.Type)
		if err != nil {
			return v, err
		}

		vals[i] = value
	}

	return language.NewComposite(vals), nil
}
