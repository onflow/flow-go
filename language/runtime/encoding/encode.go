package encoding

import (
	"bytes"
	"fmt"
	"io"
	"math/big"

	xdr "github.com/davecgh/go-xdr/xdr2"

	"github.com/dapperlabs/flow-go/language/runtime/values"
)

// An Encoder converts Cadence values into XDR-encoded bytes.
type Encoder struct {
	enc *xdr.Encoder
}

// Encode returns the XDR-encoded representation of the given value.
func Encode(v values.Value) ([]byte, error) {
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
func (e *Encoder) Encode(v values.Value) error {
	switch x := v.(type) {
	case values.Void:
		return e.EncodeVoid()
	case values.Optional:
		return e.EncodeOptional(x)
	case values.Bool:
		return e.EncodeBool(x)
	case values.String:
		return e.EncodeString(x)
	case values.Bytes:
		return e.EncodeBytes(x)
	case values.Address:
		return e.EncodeAddress(x)
	case values.Int:
		return e.EncodeInt(x)
	case values.Int8:
		return e.EncodeInt8(x)
	case values.Int16:
		return e.EncodeInt16(x)
	case values.Int32:
		return e.EncodeInt32(x)
	case values.Int64:
		return e.EncodeInt64(x)
	case values.UInt8:
		return e.EncodeUint8(x)
	case values.UInt16:
		return e.EncodeUint16(x)
	case values.UInt32:
		return e.EncodeUint32(x)
	case values.UInt64:
		return e.EncodeUint64(x)
	case values.VariableSizedArray:
		return e.EncodeVariableSizedArray(x)
	case values.ConstantSizedArray:
		return e.EncodeConstantSizedArray(x)
	case values.Dictionary:
		return e.EncodeDictionary(x)
	case values.Composite:
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
func (e *Encoder) EncodeOptional(v values.Optional) error {
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
func (e *Encoder) EncodeBool(v values.Bool) error {
	_, err := e.enc.EncodeBool(bool(v))
	return err
}

// EncodeString writes the XDR-encoded representation of a string value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.11
//  RFC Section 4.11 - String
//  Unsigned integer length followed by bytes zero-padded to a multiple of four
func (e *Encoder) EncodeString(v values.String) error {
	_, err := e.enc.EncodeString(string(v))
	return err
}

// EncodeBytes writes the XDR-encoded representation of a byte array.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.10
//  RFC Section 4.10 - Variable-Length Opaque Data
//  Unsigned integer length followed by fixed opaque data of that length
func (e *Encoder) EncodeBytes(v values.Bytes) error {
	_, err := e.enc.EncodeOpaque(v)
	return err
}

// EncodeAddress writes the XDR-encoded representation of an address.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.9
//  RFC Section 4.9 - Fixed-Length Opaque Data
//  Fixed-length uninterpreted data zero-padded to a multiple of four
func (e *Encoder) EncodeAddress(v values.Address) error {
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
func (e *Encoder) EncodeInt(v values.Int) error {
	val := v.Big()

	isPositive := val.Cmp(big.NewInt(0)) >= 0

	var b []byte

	if isPositive {
		b = []byte{1}
	} else {
		b = []byte{0}
	}

	b = append(b, val.Bytes()...)

	_, err := e.enc.EncodeOpaque(b)
	return err
}

// EncodeInt8 writes the XDR-encoded representation of an int-8 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.1
//  RFC Section 4.1 - Integer
//  32-bit big-endian signed integer in range [-2147483648, 2147483647]
func (e *Encoder) EncodeInt8(v values.Int8) error {
	_, err := e.enc.EncodeInt(int32(v))
	return err
}

// EncodeInt16 writes the XDR-encoded representation of an int-16 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.1
//  RFC Section 4.1 - Integer
//  32-bit big-endian signed integer in range [-2147483648, 2147483647]
func (e *Encoder) EncodeInt16(v values.Int16) error {
	_, err := e.enc.EncodeInt(int32(v))
	return err
}

// EncodeInt32 writes the XDR-encoded representation of an int-32 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.1
//  RFC Section 4.1 - Integer
//  32-bit big-endian signed integer in range [-2147483648, 2147483647]
func (e *Encoder) EncodeInt32(v values.Int32) error {
	_, err := e.enc.EncodeInt(int32(v))
	return err
}

// EncodeInt64 writes the XDR-encoded representation of an int-64 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.5
//  RFC Section 4.5 - Hyper Integer
//  64-bit big-endian signed integer in range [-9223372036854775808, 9223372036854775807]
func (e *Encoder) EncodeInt64(v values.Int64) error {
	_, err := e.enc.EncodeHyper(int64(v))
	return err
}

// EncodeUint8 writes the XDR-encoded representation of a uint-8 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.2
//  RFC Section 4.2 - Unsigned Integer
//  32-bit big-endian unsigned integer in range [0, 4294967295]
func (e *Encoder) EncodeUint8(v values.UInt8) error {
	_, err := e.enc.EncodeUint(uint32(v))
	return err
}

// EncodeUint16 writes the XDR-encoded representation of a uint-16 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.2
//  RFC Section 4.2 - Unsigned Integer
//  32-bit big-endian unsigned integer in range [0, 4294967295]
func (e *Encoder) EncodeUint16(v values.UInt16) error {
	_, err := e.enc.EncodeUint(uint32(v))
	return err
}

// EncodeUint32 writes the XDR-encoded representation of a uint-32 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.2
//  RFC Section 4.2 - Unsigned Integer
//  32-bit big-endian unsigned integer in range [0, 4294967295]
func (e *Encoder) EncodeUint32(v values.UInt32) error {
	_, err := e.enc.EncodeUint(uint32(v))
	return err
}

// EncodeUint64 writes the XDR-encoded representation of a uint-64 value.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.5
//  RFC Section 4.5 - Unsigned Hyper Integer
//  64-bit big-endian unsigned integer in range [0, 18446744073709551615]
func (e *Encoder) EncodeUint64(v values.UInt64) error {
	_, err := e.enc.EncodeUhyper(uint64(v))
	return err
}

// EncodeVariableSizedArray writes the XDR-encoded representation of a
// variable-sized array.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.13
//  RFC Section 4.13 - Variable-Length Array
//  Unsigned integer length followed by individually XDR-encoded array elements
func (e *Encoder) EncodeVariableSizedArray(v values.VariableSizedArray) error {
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
func (e *Encoder) EncodeConstantSizedArray(v values.ConstantSizedArray) error {
	return e.encodeArray(v.Values)
}

// encodeArray writes the XDR-encoded representation of a constant-sized array.
//
// Reference: https://tools.ietf.org/html/rfc4506#section-4.12
//  RFC Section 4.12 - Fixed-Length Array
//  Individually XDR-encoded array elements
func (e *Encoder) encodeArray(v []values.Value) error {
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
func (e *Encoder) EncodeDictionary(v values.Dictionary) error {
	size := uint32(len(v.Pairs))

	// size is encoded as an unsigned integer
	_, err := e.enc.EncodeUint(size)
	if err != nil {
		return err
	}

	// keys and elements are encoded as separate fixed-length arrays
	keys := make([]values.Value, size)
	elements := make([]values.Value, size)

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
func (e *Encoder) EncodeComposite(v values.Composite) error {
	return e.encodeArray(v.Fields)
}
