/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package xdr

import (
	"bytes"
	"fmt"
	"io"
	"math/big"

	xdr "github.com/davecgh/go-xdr/xdr2"

	cadence "github.com/onflow/cadence-initial"
)

// A Decoder decodes XDR-encoded representations of Cadence values.
type Decoder struct {
	dec *xdr.Decoder
}

// Decode returns a Cadence value decoded from its XDR-encoded representation.
//
// This function returns an error if the bytes do not match the given type
// definition.
func Decode(t cadence.Type, b []byte) (cadence.Value, error) {
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
func (d *Decoder) Decode(t cadence.Type) (cadence.Value, error) {
	switch x := t.(type) {
	case cadence.VoidType:
		return d.DecodeVoid()
	case cadence.OptionalType:
		return d.DecodeOptional(x)
	case cadence.BoolType:
		return d.DecodeBool()
	case cadence.StringType:
		return d.DecodeString()
	case cadence.BytesType:
		return d.DecodeBytes()
	case cadence.AddressType:
		return d.DecodeAddress()
	case cadence.IntType:
		return d.DecodeInt()
	case cadence.Int8Type:
		return d.DecodeInt8()
	case cadence.Int16Type:
		return d.DecodeInt16()
	case cadence.Int32Type:
		return d.DecodeInt32()
	case cadence.Int64Type:
		return d.DecodeInt64()
	case cadence.Int128Type:
		return d.DecodeInt128()
	case cadence.Int256Type:
		return d.DecodeInt256()
	case cadence.UIntType:
		return d.DecodeUInt()
	case cadence.UInt8Type:
		return d.DecodeUInt8()
	case cadence.UInt16Type:
		return d.DecodeUInt16()
	case cadence.UInt32Type:
		return d.DecodeUInt32()
	case cadence.UInt64Type:
		return d.DecodeUInt64()
	case cadence.UInt128Type:
		return d.DecodeUInt128()
	case cadence.UInt256Type:
		return d.DecodeUInt256()
	case cadence.Word8Type:
		return d.DecodeWord8()
	case cadence.Word16Type:
		return d.DecodeWord16()
	case cadence.Word32Type:
		return d.DecodeWord32()
	case cadence.Word64Type:
		return d.DecodeWord64()
	case cadence.Fix64Type:
		return d.DecodeFix64()
	case cadence.UFix64Type:
		return d.DecodeUFix64()
	case cadence.ArrayType:
		return d.DecodeArray(x)
	case cadence.DictionaryType:
		return d.DecodeDictionary(x)
	case cadence.ResourceType:
		return d.DecodeResource(x)
	case cadence.StructType:
		return d.DecodeStruct(x)
	case cadence.EventType:
		return d.DecodeEvent(x)

	default:
		return nil, fmt.Errorf("unsupported type: %T", t)
	}
}

// DecodeVoid reads the XDR-encoded representation of a void value.
//
// VoidType values are skipped by the decoder because they are empty by
// definition, but this function still exists in order to support composite
// types that contain void values.
func (d *Decoder) DecodeVoid() (cadence.Void, error) {
	// void values are not encoded
	return cadence.Void{}, nil
}

// DecodeOptional reads the XDR-encoded representation of an optional value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.19
//  RFC Section 4.19 - Optional-Data
//  Union of boolean and encoded value
func (d *Decoder) DecodeOptional(t cadence.OptionalType) (v cadence.Optional, err error) {
	hasValue, err := d.DecodeBool()
	if err != nil {
		return v, err
	}

	if hasValue {
		value, err := d.Decode(t.Type)
		if err != nil {
			return v, err
		}

		return cadence.Optional{Value: value}, nil
	}

	return cadence.Optional{Value: nil}, nil
}

// DecodeBool reads the XDR-encoded representation of a boolean value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.4
//  RFC Section 4.4 - Boolean
//  Represented as an XDR encoded enumeration where 0 is false and 1 is true
func (d *Decoder) DecodeBool() (v cadence.Bool, err error) {
	b, _, err := d.dec.DecodeBool()
	if err != nil {
		return v, err
	}

	return cadence.NewBool(b), nil
}

// DecodeString reads the XDR-encoded representation of a string value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.11
//  RFC Section 4.11 - StringType
//  Unsigned integer length followed by bytes zero-padded to a multiple of four
func (d *Decoder) DecodeString() (v cadence.String, err error) {
	str, _, err := d.dec.DecodeString()
	if err != nil {
		return v, err
	}

	return cadence.NewString(str), nil
}

// DecodeBytes reads the XDR-encoded representation of a byte array.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.10
//  RFC Section 4.10 - Variable-Length Opaque Data
//  Unsigned integer length followed by fixed opaque data of that length
func (d *Decoder) DecodeBytes() (v cadence.Bytes, err error) {
	b, _, err := d.dec.DecodeOpaque()
	if err != nil {
		return v, err
	}

	if b == nil {
		b = []byte{}
	}

	return cadence.NewBytes(b), nil
}

// DecodeAddress reads the XDR-encoded representation of an address.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.9
//  RFC Section 4.9 - Fixed-Length Opaque Data
//  Fixed-length uninterpreted data zero-padded to a multiple of four
func (d *Decoder) DecodeAddress() (v cadence.Address, err error) {
	b, _, err := d.dec.DecodeFixedOpaque(cadence.AddressLength)
	if err != nil {
		return v, err
	}

	return cadence.BytesToAddress(b), nil
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
func (d *Decoder) DecodeInt() (v cadence.Int, err error) {
	i, err := d.decodeBig()
	if err != nil {
		return v, err
	}
	return cadence.NewIntFromBig(i), nil
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

	i = new(big.Int).SetBytes(b[1:])

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
func (d *Decoder) DecodeInt8() (v cadence.Int8, err error) {
	i, _, err := d.dec.DecodeInt()
	if err != nil {
		return v, err
	}

	return cadence.NewInt8(int8(i)), nil
}

// DecodeInt16 reads the XDR-encoded representation of an int-16 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.1
//  RFC Section 4.1 - Integer
//  32-bit big-endian signed integer in range [-2147483648, 2147483647]
func (d *Decoder) DecodeInt16() (v cadence.Int16, err error) {
	i, _, err := d.dec.DecodeInt()
	if err != nil {
		return v, err
	}

	return cadence.NewInt16(int16(i)), nil
}

// DecodeInt32 reads the XDR-encoded representation of an int-32 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.1
//  RFC Section 4.1 - Integer
//  32-bit big-endian signed integer in range [-2147483648, 2147483647]
func (d *Decoder) DecodeInt32() (v cadence.Int32, err error) {
	i, _, err := d.dec.DecodeInt()
	if err != nil {
		return v, err
	}

	return cadence.NewInt32(i), nil
}

// DecodeInt64 reads the XDR-encoded representation of an int-64 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.5
//  RFC Section 4.5 - Hyper Integer
//  64-bit big-endian signed integer in range [-9223372036854775808, 9223372036854775807]
func (d *Decoder) DecodeInt64() (v cadence.Int64, err error) {
	i, _, err := d.dec.DecodeHyper()
	if err != nil {
		return v, err
	}

	return cadence.NewInt64(i), nil
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
func (d *Decoder) DecodeInt128() (v cadence.Int128, err error) {
	i, err := d.decodeBig()
	if err != nil {
		return v, err
	}
	return cadence.NewInt128FromBig(i), nil
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
func (d *Decoder) DecodeInt256() (v cadence.Int256, err error) {
	i, err := d.decodeBig()
	if err != nil {
		return v, err
	}
	return cadence.NewInt256FromBig(i), nil
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
func (d *Decoder) DecodeUInt() (v cadence.UInt, err error) {
	i, err := d.decodeBig()
	if err != nil {
		return v, err
	}
	return cadence.NewUIntFromBig(i), nil
}

// DecodeUInt8 reads the XDR-encoded representation of a uint-8 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.2
//  RFC Section 4.2 - Unsigned Integer
//  32-bit big-endian unsigned integer in range [0, 4294967295]
func (d *Decoder) DecodeUInt8() (v cadence.UInt8, err error) {
	i, _, err := d.dec.DecodeUint()
	if err != nil {
		return v, err
	}

	return cadence.NewUInt8(uint8(i)), nil
}

// DecodeUInt16 reads the XDR-encoded representation of a uint-16 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.2
//  RFC Section 4.2 - Unsigned Integer
//  32-bit big-endian unsigned integer in range [0, 4294967295]
func (d *Decoder) DecodeUInt16() (v cadence.UInt16, err error) {
	i, _, err := d.dec.DecodeUint()
	if err != nil {
		return v, err
	}

	return cadence.NewUInt16(uint16(i)), nil
}

// DecodeUInt32 reads the XDR-encoded representation of a uint-32 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.2
//  RFC Section 4.2 - Unsigned Integer
//  32-bit big-endian unsigned integer in range [0, 4294967295]
func (d *Decoder) DecodeUInt32() (v cadence.UInt32, err error) {
	i, _, err := d.dec.DecodeUint()
	if err != nil {
		return v, err
	}

	return cadence.NewUInt32(i), nil
}

// DecodeUInt64 reads the XDR-encoded representation of a uint-64 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.5
//  RFC Section 4.5 - Unsigned Hyper Integer
//  64-bit big-endian unsigned integer in range [0, 18446744073709551615]
func (d *Decoder) DecodeUInt64() (v cadence.UInt64, err error) {
	i, _, err := d.dec.DecodeUhyper()
	if err != nil {
		return v, err
	}

	return cadence.NewUInt64(i), nil
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
func (d *Decoder) DecodeUInt128() (v cadence.UInt128, err error) {
	i, err := d.decodeBig()
	if err != nil {
		return v, err
	}
	return cadence.NewUInt128FromBig(i), nil
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
func (d *Decoder) DecodeUInt256() (v cadence.UInt256, err error) {
	i, err := d.decodeBig()
	if err != nil {
		return v, err
	}
	return cadence.NewUInt256FromBig(i), nil
}

// DecodeWord8 reads the XDR-encoded representation of a uint-8 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.2
//  RFC Section 4.2 - Unsigned Integer
//  32-bit big-endian unsigned integer in range [0, 4294967295]
func (d *Decoder) DecodeWord8() (v cadence.Word8, err error) {
	i, _, err := d.dec.DecodeUint()
	if err != nil {
		return v, err
	}

	return cadence.NewWord8(uint8(i)), nil
}

// DecodeWord16 reads the XDR-encoded representation of a uint-16 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.2
//  RFC Section 4.2 - Unsigned Integer
//  32-bit big-endian unsigned integer in range [0, 4294967295]
func (d *Decoder) DecodeWord16() (v cadence.Word16, err error) {
	i, _, err := d.dec.DecodeUint()
	if err != nil {
		return v, err
	}

	return cadence.NewWord16(uint16(i)), nil
}

// DecodeWord32 reads the XDR-encoded representation of a uint-32 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.2
//  RFC Section 4.2 - Unsigned Integer
//  32-bit big-endian unsigned integer in range [0, 4294967295]
func (d *Decoder) DecodeWord32() (v cadence.Word32, err error) {
	i, _, err := d.dec.DecodeUint()
	if err != nil {
		return v, err
	}

	return cadence.NewWord32(i), nil
}

// DecodeWord64 reads the XDR-encoded representation of a uint-64 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.5
//  RFC Section 4.5 - Unsigned Hyper Integer
//  64-bit big-endian unsigned integer in range [0, 18446744073709551615]
func (d *Decoder) DecodeWord64() (v cadence.Word64, err error) {
	i, _, err := d.dec.DecodeUhyper()
	if err != nil {
		return v, err
	}

	return cadence.NewWord64(i), nil
}

// DecodeFix64 reads the XDR-encoded representation of an int-64 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.5
//  RFC Section 4.5 - Hyper Integer
//  64-bit big-endian signed integer in range [-9223372036854775808, 9223372036854775807]
func (d *Decoder) DecodeFix64() (v cadence.Fix64, err error) {
	i, _, err := d.dec.DecodeHyper()
	if err != nil {
		return v, err
	}

	return cadence.NewFix64(i), nil
}

// DecodeUFix64 reads the XDR-encoded representation of a uint-64 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.5
//  RFC Section 4.5 - Unsigned Hyper Integer
//  64-bit big-endian unsigned integer in range [0, 18446744073709551615]
func (d *Decoder) DecodeUFix64() (v cadence.UFix64, err error) {
	i, _, err := d.dec.DecodeUhyper()
	if err != nil {
		return v, err
	}

	return cadence.NewUFix64(i), nil
}

// DecodeArray reads the XDR-encoded representation of an array.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.13
//  RFC Section 4.13 - Variable-Length Array
//  Unsigned integer length followed by individually XDR-encoded array elements
func (d *Decoder) DecodeArray(t cadence.ArrayType) (v cadence.Array, err error) {
	size, _, err := d.dec.DecodeUint()
	if err != nil {
		return v, err
	}

	vals, err := d.decodeArray(t.Element(), uint(size))
	if err != nil {
		return v, err
	}

	return cadence.NewArray(vals), nil
}

// decodeArray reads the XDR-encoded representation of a constant-sized array.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.12
//  RFC Section 4.12 - Fixed-Length Array
//  Individually XDR-encoded array elements
func (d *Decoder) decodeArray(t cadence.Type, size uint) ([]cadence.Value, error) {
	array := make([]cadence.Value, size)

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
func (d *Decoder) DecodeDictionary(t cadence.DictionaryType) (v cadence.Dictionary, err error) {
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

	pairs := make([]cadence.KeyValuePair, size)

	for i := 0; i < int(size); i++ {
		key := keys[i]
		element := elements[i]

		pairs[i] = cadence.KeyValuePair{
			Key:   key,
			Value: element,
		}
	}

	return cadence.NewDictionary(pairs), nil
}

// DecodeStruct reads the XDR-encoded representation of a struct value.
//
// A struct is encoded as a fixed-length array of its field values.
func (d *Decoder) DecodeStruct(t cadence.StructType) (v cadence.Struct, err error) {
	fields, err := d.decodeComposite(t.Fields)
	if err != nil {
		return v, err
	}

	return cadence.Struct{
		StructType: t,
		Fields:     fields,
	}, nil
}

// DecodeResource reads the XDR-encoded representation of a resource value.
//
// A resource is encoded as a fixed-length array of its field values.
func (d *Decoder) DecodeResource(t cadence.ResourceType) (v cadence.Resource, err error) {
	fields, err := d.decodeComposite(t.Fields)
	if err != nil {
		return v, err
	}

	return cadence.Resource{
		ResourceType: t,
		Fields:       fields,
	}, nil
}

// DecodeEvent reads the XDR-encoded representation of an event value.
//
// An event is encoded as a fixed-length array of its field values.
func (d *Decoder) DecodeEvent(t cadence.EventType) (v cadence.Event, err error) {
	fields, err := d.decodeComposite(t.Fields)
	if err != nil {
		return v, err
	}

	return cadence.Event{
		EventType: t,
		Fields:    fields,
	}, nil
}

// decodeComposite reads the XDR-encoded representation of a composite value.
//
// A composite is encoded as a fixed-length array of its field values.
func (d *Decoder) decodeComposite(fields []cadence.Field) (vals []cadence.Value, err error) {
	vals = make([]cadence.Value, len(fields))

	for i, field := range fields {
		value, err := d.Decode(field.Type)
		if err != nil {
			return nil, err
		}

		vals[i] = value
	}

	return vals, nil
}
