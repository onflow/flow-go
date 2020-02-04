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
func (e *Decoder) Decode(t language.Type) (language.Value, error) {
	switch x := t.(type) {
	case language.VoidType:
		return e.DecodeVoid()
	case language.OptionalType:
		return e.DecodeOptional(x)
	case language.BoolType:
		return e.DecodeBool()
	case language.StringType:
		return e.DecodeString()
	case language.BytesType:
		return e.DecodeBytes()
	case language.AddressType:
		return e.DecodeAddress()
	case language.IntType:
		return e.DecodeInt()
	case language.Int8Type:
		return e.DecodeInt8()
	case language.Int16Type:
		return e.DecodeInt16()
	case language.Int32Type:
		return e.DecodeInt32()
	case language.Int64Type:
		return e.DecodeInt64()
	case language.UInt8Type:
		return e.DecodeUInt8()
	case language.UInt16Type:
		return e.DecodeUInt16()
	case language.UInt32Type:
		return e.DecodeUInt32()
	case language.UInt64Type:
		return e.DecodeUInt64()
	case language.VariableSizedArrayType:
		return e.DecodeVariableSizedArray(x)
	case language.ConstantSizedArrayType:
		return e.DecodeConstantSizedArray(x)
	case language.DictionaryType:
		return e.DecodeDictionary(x)
	case language.CompositeType:
		return e.DecodeComposite(x)
	case language.ResourceType:
		return e.DecodeComposite(x.CompositeType)
	case language.StructType:
		return e.DecodeComposite(x.CompositeType)
	case language.EventType:
		return e.DecodeComposite(x.CompositeType)

	default:
		return nil, fmt.Errorf("unsupported type: %T", t)
	}
}

// DecodeVoid reads the XDR-encoded representation of a void value.
//
// VoidType values are skipped by the decoder because they are empty by
// definition, but this function still exists in order to support composite
// types that contain void values.
func (e *Decoder) DecodeVoid() (language.Void, error) {
	// void values are not encoded
	return language.Void{}, nil
}

// DecodeOptional reads the XDR-encoded representation of an optional value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.19
//  RFC Section 4.19 - Optional-Data
//  Union of boolean and encoded value
func (e *Decoder) DecodeOptional(t language.OptionalType) (v language.Optional, err error) {
	hasValue, err := e.DecodeBool()
	if err != nil {
		return v, err
	}

	if hasValue {
		value, err := e.Decode(t.Type)
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
func (e *Decoder) DecodeBool() (v language.Bool, err error) {
	b, _, err := e.dec.DecodeBool()
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
func (e *Decoder) DecodeString() (v language.String, err error) {
	str, _, err := e.dec.DecodeString()
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
func (e *Decoder) DecodeBytes() (v language.Bytes, err error) {
	b, _, err := e.dec.DecodeOpaque()
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
func (e *Decoder) DecodeAddress() (v language.Address, err error) {
	b, _, err := e.dec.DecodeFixedOpaque(20)
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
func (e *Decoder) DecodeInt() (v language.Int, err error) {
	b, _, err := e.dec.DecodeOpaque()
	if err != nil {
		return v, err
	}

	isPositive := b[0] == 1

	i := big.NewInt(0).SetBytes(b[1:])

	if !isPositive {
		i = i.Neg(i)
	}

	return language.NewIntFromBig(i), nil
}

// DecodeInt8 reads the XDR-encoded representation of an int-8 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.1
//  RFC Section 4.1 - Integer
//  32-bit big-endian signed integer in range [-2147483648, 2147483647]
func (e *Decoder) DecodeInt8() (v language.Int8, err error) {
	i, _, err := e.dec.DecodeInt()
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
func (e *Decoder) DecodeInt16() (v language.Int16, err error) {
	i, _, err := e.dec.DecodeInt()
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
func (e *Decoder) DecodeInt32() (v language.Int32, err error) {
	i, _, err := e.dec.DecodeInt()
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
func (e *Decoder) DecodeInt64() (v language.Int64, err error) {
	i, _, err := e.dec.DecodeHyper()
	if err != nil {
		return v, err
	}

	return language.NewInt64(i), nil
}

// DecodeUInt8 reads the XDR-encoded representation of a uint-8 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.2
//  RFC Section 4.2 - Unsigned Integer
//  32-bit big-endian unsigned integer in range [0, 4294967295]
func (e *Decoder) DecodeUInt8() (v language.UInt8, err error) {
	i, _, err := e.dec.DecodeUint()
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
func (e *Decoder) DecodeUInt16() (v language.UInt16, err error) {
	i, _, err := e.dec.DecodeUint()
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
func (e *Decoder) DecodeUInt32() (v language.UInt32, err error) {
	i, _, err := e.dec.DecodeUint()
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
func (e *Decoder) DecodeUInt64() (v language.UInt64, err error) {
	i, _, err := e.dec.DecodeUhyper()
	if err != nil {
		return v, err
	}

	return language.NewUInt64(i), nil
}

// DecodeVariableSizedArray reads the XDR-encoded representation of a
// variable-sized array.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.13
//  RFC Section 4.13 - Variable-Length Array
//  Unsigned integer length followed by individually XDR-encoded array elements
func (e *Decoder) DecodeVariableSizedArray(t language.VariableSizedArrayType) (v language.VariableSizedArray, err error) {
	size, _, err := e.dec.DecodeUint()
	if err != nil {
		return v, err
	}

	vals, err := e.decodeArray(t.ElementType, uint(size))
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
func (e *Decoder) DecodeConstantSizedArray(t language.ConstantSizedArrayType) (v language.ConstantSizedArray, err error) {
	vals, err := e.decodeArray(t.ElementType, t.Size)
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
func (e *Decoder) decodeArray(t language.Type, size uint) ([]language.Value, error) {
	array := make([]language.Value, size)

	var i uint
	for i = 0; i < size; i++ {
		value, err := e.Decode(t)
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
func (e *Decoder) DecodeDictionary(t language.DictionaryType) (v language.Dictionary, err error) {
	size, _, err := e.dec.DecodeUint()
	if err != nil {
		return v, err
	}

	keys, err := e.decodeArray(t.KeyType, uint(size))
	if err != nil {
		return v, err
	}

	elements, err := e.decodeArray(t.ElementType, uint(size))
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
func (e *Decoder) DecodeComposite(t language.CompositeType) (v language.Composite, err error) {
	vals := make([]language.Value, len(t.Fields))

	for i, field := range t.Fields {
		value, err := e.Decode(field.Type)
		if err != nil {
			return v, err
		}

		vals[i] = value
	}

	return language.NewComposite(vals), nil
}
