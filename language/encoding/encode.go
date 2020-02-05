package encoding

import (
	"bytes"
	"fmt"
	"io"
	"math/big"

	xdr "github.com/davecgh/go-xdr/xdr2"

	"github.com/dapperlabs/flow-go/language"
)

// An Encoder converts Cadence values into XDR-encoded bytes.
type Encoder struct {
	enc *xdr.Encoder
}

// Encode returns the XDR-encoded representation of the given value.
func Encode(v language.Value) ([]byte, error) {
	var w bytes.Buffer
	enc := NewEncoder(&w)

	err := enc.Encode(v)
	if err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

// NewEncoder initializes an Encoder that will write XDR-encoded bytes to the
// given io.Writer.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{xdr.NewEncoder(w)}
}

// Encode writes the XDR-encoded representation of the given value to this
// encoder's io.Writer.
//
// This function returns an error if the given value's type is not supported
// by this encoder.
func (e *Encoder) Encode(v language.Value) error {
	switch x := v.(type) {
	case language.Void:
		return e.EncodeVoid()
	case language.Optional:
		return e.EncodeOptional(x)
	case language.Bool:
		return e.EncodeBool(x)
	case language.String:
		return e.EncodeString(x)
	case language.Bytes:
		return e.EncodeBytes(x)
	case language.Address:
		return e.EncodeAddress(x)
	case language.Int:
		return e.EncodeInt(x)
	case language.Int8:
		return e.EncodeInt8(x)
	case language.Int16:
		return e.EncodeInt16(x)
	case language.Int32:
		return e.EncodeInt32(x)
	case language.Int64:
		return e.EncodeInt64(x)
	case language.Int128:
		return e.EncodeInt128(x)
	case language.Int256:
		return e.EncodeInt256(x)
	case language.UInt:
		return e.EncodeUInt(x)
	case language.UInt8:
		return e.EncodeUInt8(x)
	case language.UInt16:
		return e.EncodeUInt16(x)
	case language.UInt32:
		return e.EncodeUInt32(x)
	case language.UInt64:
		return e.EncodeUInt64(x)
	case language.UInt128:
		return e.EncodeUInt128(x)
	case language.UInt256:
		return e.EncodeUInt256(x)
	case language.Word8:
		return e.EncodeWord8(x)
	case language.Word16:
		return e.EncodeWord16(x)
	case language.Word32:
		return e.EncodeWord32(x)
	case language.Word64:
		return e.EncodeWord64(x)
	case language.VariableSizedArray:
		return e.EncodeVariableSizedArray(x)
	case language.ConstantSizedArray:
		return e.EncodeConstantSizedArray(x)
	case language.Dictionary:
		return e.EncodeDictionary(x)
	case language.Composite:
		return e.EncodeComposite(x)
	default:
		return fmt.Errorf("unsupported value: %T, %v", v, v)
	}
}

// EncodeVoid writes the XDR-encoded representation of a void value.
//
// Void values are skipped by the encoder because they are empty by
// definition, but this function still exists in order to support composite
// types that contain void values.
func (e *Encoder) EncodeVoid() error {
	// void values are not encoded
	return nil
}

// EncodeOptional writes the XDR-encoded representation of an optional value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.19
//  RFC Section 4.19 - Optional-Data
//  Union of boolean and encoded value
func (e *Encoder) EncodeOptional(v language.Optional) error {
	hasValue := v.Value != nil
	_, err := e.enc.EncodeBool(hasValue)
	if err != nil {
		return err
	}

	if hasValue {
		return e.Encode(v.Value)
	}

	return nil
}

// EncodeBool writes the XDR-encoded representation of a boolean value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.4
//  RFC Section 4.4 - Boolean
//  Represented as an XDR encoded enumeration where 0 is false and 1 is true
func (e *Encoder) EncodeBool(v language.Bool) error {
	_, err := e.enc.EncodeBool(bool(v))
	return err
}

// EncodeString writes the XDR-encoded representation of a string value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.11
//  RFC Section 4.11 - String
//  Unsigned integer length followed by bytes zero-padded to a multiple of four
func (e *Encoder) EncodeString(v language.String) error {
	_, err := e.enc.EncodeString(string(v))
	return err
}

// EncodeBytes writes the XDR-encoded representation of a byte array.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.10
//  RFC Section 4.10 - Variable-Length Opaque Data
//  Unsigned integer length followed by fixed opaque data of that length
func (e *Encoder) EncodeBytes(v language.Bytes) error {
	_, err := e.enc.EncodeOpaque(v)
	return err
}

// EncodeAddress writes the XDR-encoded representation of an address.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.9
//  RFC Section 4.9 - Fixed-Length Opaque Data
//  Fixed-length uninterpreted data zero-padded to a multiple of four
func (e *Encoder) EncodeAddress(v language.Address) error {
	_, err := e.enc.EncodeFixedOpaque(v.Bytes())
	return err
}

// EncodeInt writes the XDR-encoded representation of an arbitrary-precision
// integer value.
//
// An arbitrary-precision integer is encoded as follows:
//   Sign as a byte flag (positive is 1, negative is 0)
//   Absolute value as variable-length big-endian byte array
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.10
//  RFC Section 4.10 - Variable-Length Opaque Data
//  Unsigned integer length followed by fixed opaque data of that length
func (e *Encoder) EncodeInt(v language.Int) error {
	return e.encodeBig(v.Big())
}

// encodeBig writes the XDR-encoded representation of an arbitrary-precision
// integer value.
//
// An arbitrary-precision integer is encoded as follows:
//   Sign as a byte flag (positive is 1, negative is 0)
//   Absolute value as variable-length big-endian byte array
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.10
//  RFC Section 4.10 - Variable-Length Opaque Data
//  Unsigned integer length followed by fixed opaque data of that length
func (e *Encoder) encodeBig(v *big.Int) error {
	var b []byte

	if v.Sign() >= 0 {
		b = []byte{1}
	} else {
		b = []byte{0}
	}

	b = append(b, v.Bytes()...)

	_, err := e.enc.EncodeOpaque(b)
	return err
}

// EncodeInt8 writes the XDR-encoded representation of an int-8 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.1
//  RFC Section 4.1 - Integer
//  32-bit big-endian signed integer in range [-2147483648, 2147483647]
func (e *Encoder) EncodeInt8(v language.Int8) error {
	_, err := e.enc.EncodeInt(int32(v))
	return err
}

// EncodeInt16 writes the XDR-encoded representation of an int-16 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.1
//  RFC Section 4.1 - Integer
//  32-bit big-endian signed integer in range [-2147483648, 2147483647]
func (e *Encoder) EncodeInt16(v language.Int16) error {
	_, err := e.enc.EncodeInt(int32(v))
	return err
}

// EncodeInt32 writes the XDR-encoded representation of an int-32 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.1
//  RFC Section 4.1 - Integer
//  32-bit big-endian signed integer in range [-2147483648, 2147483647]
func (e *Encoder) EncodeInt32(v language.Int32) error {
	_, err := e.enc.EncodeInt(int32(v))
	return err
}

// EncodeInt64 writes the XDR-encoded representation of an int-64 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.5
//  RFC Section 4.5 - Hyper Integer
//  64-bit big-endian signed integer in range [-9223372036854775808, 9223372036854775807]
func (e *Encoder) EncodeInt64(v language.Int64) error {
	_, err := e.enc.EncodeHyper(int64(v))
	return err
}

// EncodeInt128 writes the XDR-encoded representation of an arbitrary-precision
// integer value.
//
// An arbitrary-precision integer is encoded as follows:
//   Sign as a byte flag (positive is 1, negative is 0)
//   Absolute value as variable-length big-endian byte array
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.10
//  RFC Section 4.10 - Variable-Length Opaque Data
//  Unsigned integer length followed by fixed opaque data of that length
func (e *Encoder) EncodeInt128(v language.Int128) error {
	return e.encodeBig(v.Big())
}

// EncodeInt256 writes the XDR-encoded representation of an arbitrary-precision
// integer value.
//
// An arbitrary-precision integer is encoded as follows:
//   Sign as a byte flag (positive is 1, negative is 0)
//   Absolute value as variable-length big-endian byte array
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.10
//  RFC Section 4.10 - Variable-Length Opaque Data
//  Unsigned integer length followed by fixed opaque data of that length
func (e *Encoder) EncodeInt256(v language.Int256) error {
	return e.encodeBig(v.Big())
}

// EncodeUInt writes the XDR-encoded representation of an arbitrary-precision
// integer value.
//
// An arbitrary-precision integer is encoded as follows:
//   Sign as a byte flag (positive is 1, negative is 0)
//   Absolute value as variable-length big-endian byte array
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.10
//  RFC Section 4.10 - Variable-Length Opaque Data
//  Unsigned integer length followed by fixed opaque data of that length
func (e *Encoder) EncodeUInt(v language.UInt) error {
	return e.encodeBig(v.Big())
}

// EncodeUInt8 writes the XDR-encoded representation of a uint-8 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.2
//  RFC Section 4.2 - Unsigned Integer
//  32-bit big-endian unsigned integer in range [0, 4294967295]
func (e *Encoder) EncodeUInt8(v language.UInt8) error {
	_, err := e.enc.EncodeUint(uint32(v))
	return err
}

// EncodeUInt16 writes the XDR-encoded representation of a uint-16 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.2
//  RFC Section 4.2 - Unsigned Integer
//  32-bit big-endian unsigned integer in range [0, 4294967295]
func (e *Encoder) EncodeUInt16(v language.UInt16) error {
	_, err := e.enc.EncodeUint(uint32(v))
	return err
}

// EncodeUInt32 writes the XDR-encoded representation of a uint-32 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.2
//  RFC Section 4.2 - Unsigned Integer
//  32-bit big-endian unsigned integer in range [0, 4294967295]
func (e *Encoder) EncodeUInt32(v language.UInt32) error {
	_, err := e.enc.EncodeUint(uint32(v))
	return err
}

// EncodeUInt64 writes the XDR-encoded representation of a uint-64 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.5
//  RFC Section 4.5 - Unsigned Hyper Integer
//  64-bit big-endian unsigned integer in range [0, 18446744073709551615]
func (e *Encoder) EncodeUInt64(v language.UInt64) error {
	_, err := e.enc.EncodeUhyper(uint64(v))
	return err
}

// EncodeUInt128 writes the XDR-encoded representation of an arbitrary-precision
// integer value.
//
// An arbitrary-precision integer is encoded as follows:
//   Sign as a byte flag (positive is 1, negative is 0)
//   Absolute value as variable-length big-endian byte array
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.10
//  RFC Section 4.10 - Variable-Length Opaque Data
//  Unsigned integer length followed by fixed opaque data of that length
func (e *Encoder) EncodeUInt128(v language.UInt128) error {
	return e.encodeBig(v.Big())
}

// EncodeUInt256 writes the XDR-encoded representation of an arbitrary-precision
// integer value.
//
// An arbitrary-precision integer is encoded as follows:
//   Sign as a byte flag (positive is 1, negative is 0)
//   Absolute value as variable-length big-endian byte array
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.10
//  RFC Section 4.10 - Variable-Length Opaque Data
//  Unsigned integer length followed by fixed opaque data of that length
func (e *Encoder) EncodeUInt256(v language.UInt256) error {
	return e.encodeBig(v.Big())
}

// EncodeWord8 writes the XDR-encoded representation of a uint-8 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.2
//  RFC Section 4.2 - Unsigned Integer
//  32-bit big-endian unsigned integer in range [0, 4294967295]
func (e *Encoder) EncodeWord8(v language.Word8) error {
	_, err := e.enc.EncodeUint(uint32(v))
	return err
}

// EncodeWord16 writes the XDR-encoded representation of a uint-16 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.2
//  RFC Section 4.2 - Unsigned Integer
//  32-bit big-endian unsigned integer in range [0, 4294967295]
func (e *Encoder) EncodeWord16(v language.Word16) error {
	_, err := e.enc.EncodeUint(uint32(v))
	return err
}

// EncodeWord32 writes the XDR-encoded representation of a uint-32 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.2
//  RFC Section 4.2 - Unsigned Integer
//  32-bit big-endian unsigned integer in range [0, 4294967295]
func (e *Encoder) EncodeWord32(v language.Word32) error {
	_, err := e.enc.EncodeUint(uint32(v))
	return err
}

// EncodeWord64 writes the XDR-encoded representation of a uint-64 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.5
//  RFC Section 4.5 - Unsigned Hyper Integer
//  64-bit big-endian unsigned integer in range [0, 18446744073709551615]
func (e *Encoder) EncodeWord64(v language.Word64) error {
	_, err := e.enc.EncodeUhyper(uint64(v))
	return err
}

// EncodeVariableSizedArray writes the XDR-encoded representation of a
// variable-sized array.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.13
//  RFC Section 4.13 - Variable-Length Array
//  Unsigned integer length followed by individually XDR-encoded array elements
func (e *Encoder) EncodeVariableSizedArray(v language.VariableSizedArray) error {
	size := uint32(len(v.Values))

	_, err := e.enc.EncodeUint(size)
	if err != nil {
		return err
	}

	return e.encodeArray(v.Values)
}

// EncodeConstantSizedArray writes the XDR-encoded representation of a
// constant-sized array.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.12
//  RFC Section 4.12 - Fixed-Length Array
//  Individually XDR-encoded array elements
func (e *Encoder) EncodeConstantSizedArray(v language.ConstantSizedArray) error {
	return e.encodeArray(v.Values)
}

// encodeArray writes the XDR-encoded representation of a constant-sized array.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.12
//  RFC Section 4.12 - Fixed-Length Array
//  Individually XDR-encoded array elements
func (e *Encoder) encodeArray(v []language.Value) error {
	for _, value := range v {
		if err := e.Encode(value); err != nil {
			return err
		}
	}

	return nil
}

// EncodeDictionary writes the XDR-encoded representation of a dictionary.
//
// The size of the dictionary is encoded as an unsigned integer, followed by
// the dictionary keys, then elements, each represented as individually
// XDR-encoded array elements.
func (e *Encoder) EncodeDictionary(v language.Dictionary) error {
	size := uint32(len(v.Pairs))

	// size is encoded as an unsigned integer
	_, err := e.enc.EncodeUint(size)
	if err != nil {
		return err
	}

	// keys and elements are encoded as separate fixed-length arrays
	keys := make([]language.Value, size)
	elements := make([]language.Value, size)

	for i, pair := range v.Pairs {
		keys[i] = pair.Key
		elements[i] = pair.Value
	}

	// encode keys
	if err := e.encodeArray(keys); err != nil {
		return err
	}

	// encode elements
	return e.encodeArray(elements)
}

// EncodeComposite writes the XDR-encoded representation of a composite value.
//
// A composite is encoded as a fixed-length array of its field values.
func (e *Encoder) EncodeComposite(v language.Composite) error {
	return e.encodeArray(v.Fields)
}
