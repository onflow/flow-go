package encoding

import (
	"bytes"
	"fmt"
	"io"

	xdr "github.com/davecgh/go-xdr/xdr2"

	"github.com/dapperlabs/flow-go/sdk/abi/types"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
)

type Decoder struct {
	dec *xdr.Decoder
}

func Decode(t types.Type, b []byte) (values.Value, error) {
	r := bytes.NewReader(b)
	dec := NewDecoder(r)

	v, err := dec.Decode(t)
	if err != nil {
		return nil, err
	}

	return v, nil
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{xdr.NewDecoder(r)}
}

func (e *Decoder) Decode(t types.Type) (values.Value, error) {
	switch x := t.(type) {
	case types.Void:
		return e.DecodeVoid()
	case types.Bool:
		return e.DecodeBool()
	case types.String:
		return e.DecodeString()
	case types.Bytes:
		return e.DecodeBytes()
	case types.Address:
		return e.DecodeAddress()
	case types.Int:
		return e.DecodeInt()
	case types.Int8:
		return e.DecodeInt8()
	case types.Int16:
		return e.DecodeInt16()
	case types.Int32:
		return e.DecodeInt32()
	case types.Int64:
		return e.DecodeInt64()
	case types.Uint8:
		return e.DecodeUint8()
	case types.Uint16:
		return e.DecodeUint16()
	case types.Uint32:
		return e.DecodeUint32()
	case types.Uint64:
		return e.DecodeUint64()
	case types.VariableSizedArray:
		return e.DecodeVariableSizedArray(x)
	case types.ConstantSizedArray:
		return e.DecodeConstantSizedArray(x)
	case types.Dictionary:
		return e.DecodeDictionary(x)
	case types.Composite:
		return e.DecodeComposite(x)
	case types.Event:
		return e.DecodeEvent(x)
	default:
		return nil, fmt.Errorf("unsupported type: %T", t)
	}

	return nil, nil
}

func (e *Decoder) DecodeVoid() (values.Void, error) {
	// void values are not encoded
	return struct{}{}, nil
}

func (e *Decoder) DecodeBool() (values.Bool, error) {
	b, _, err := e.dec.DecodeBool()
	if err != nil {
		return false, err
	}

	return values.Bool(b), nil
}

func (e *Decoder) DecodeString() (values.String, error) {
	str, _, err := e.dec.DecodeString()
	if err != nil {
		return "", err
	}

	return values.String(str), nil
}

func (e *Decoder) DecodeBytes() (values.Bytes, error) {
	b, _, err := e.dec.DecodeOpaque()
	if err != nil {
		return nil, err
	}

	if b == nil {
		b = []byte{}
	}

	return b, nil
}

func (e *Decoder) DecodeAddress() (values.Address, error) {
	b, _, err := e.dec.DecodeFixedOpaque(20)
	if err != nil {
		return values.Address{}, err
	}

	return values.BytesToAddress(b), nil
}

func (e *Decoder) DecodeInt() (values.Int, error) {
	i, _, err := e.dec.DecodeInt()
	if err != nil {
		return 0, err
	}

	return values.Int(i), nil
}

func (e *Decoder) DecodeInt8() (values.Int8, error) {
	i, _, err := e.dec.DecodeInt()
	if err != nil {
		return 0, err
	}

	return values.Int8(i), nil
}

func (e *Decoder) DecodeInt16() (values.Int16, error) {
	i, _, err := e.dec.DecodeInt()
	if err != nil {
		return 0, err
	}

	return values.Int16(i), nil
}

func (e *Decoder) DecodeInt32() (values.Int32, error) {
	i, _, err := e.dec.DecodeInt()
	if err != nil {
		return 0, err
	}

	return values.Int32(i), nil
}

func (e *Decoder) DecodeInt64() (values.Int64, error) {
	i, _, err := e.dec.DecodeHyper()
	if err != nil {
		return 0, err
	}

	return values.Int64(i), nil
}

func (e *Decoder) DecodeUint8() (values.Uint8, error) {
	i, _, err := e.dec.DecodeUint()
	if err != nil {
		return 0, err
	}

	return values.Uint8(i), nil
}

func (e *Decoder) DecodeUint16() (values.Uint16, error) {
	i, _, err := e.dec.DecodeUint()
	if err != nil {
		return 0, err
	}

	return values.Uint16(i), nil
}

func (e *Decoder) DecodeUint32() (values.Uint32, error) {
	i, _, err := e.dec.DecodeUint()
	if err != nil {
		return 0, err
	}

	return values.Uint32(i), nil
}

func (e *Decoder) DecodeUint64() (values.Uint64, error) {
	i, _, err := e.dec.DecodeUhyper()
	if err != nil {
		return 0, err
	}

	return values.Uint64(i), nil
}

// Reference:
// 	RFC Section 4.13 - Variable-Length Array
// 	Unsigned integer length followed by individually XDR encoded array elements
func (e *Decoder) DecodeVariableSizedArray(t types.VariableSizedArray) (values.VariableSizedArray, error) {
	size, _, err := e.dec.DecodeUint()
	if err != nil {
		return nil, err
	}

	constantType := types.ConstantSizedArray{
		Size:        int(size),
		ElementType: t.ElementType,
	}

	array, err := e.DecodeConstantSizedArray(constantType)
	if err != nil {
		return nil, err
	}

	return values.VariableSizedArray(array), nil
}

func (e *Decoder) DecodeConstantSizedArray(t types.ConstantSizedArray) (values.ConstantSizedArray, error) {
	return e.decodeArray(t.ElementType, t.Size)
}

func (e *Decoder) decodeArray(t types.Type, size int) ([]values.Value, error) {
	array := make([]values.Value, size)

	for i := 0; i < size; i++ {
		value, err := e.Decode(t)
		if err != nil {
			return nil, err
		}

		array[i] = value
	}

	return array, nil
}

func (e *Decoder) DecodeDictionary(t types.Dictionary) (values.Dictionary, error) {
	size, _, err := e.dec.DecodeUint()
	if err != nil {
		return nil, err
	}

	keys, err := e.decodeArray(t.KeyType, int(size))
	if err != nil {
		return nil, err
	}

	elements, err := e.decodeArray(t.ElementType, int(size))
	if err != nil {
		return nil, err
	}

	d := make(values.Dictionary, size)

	for i := 0; i < int(size); i++ {
		key := keys[i]
		element := elements[i]

		d[i] = values.KeyValuePair{
			Key:   key,
			Value: element,
		}
	}

	return d, nil
}

func (e *Decoder) DecodeComposite(t types.Composite) (values.Composite, error) {
	fields := make([]values.Value, len(t.FieldTypes))

	for i, fieldType := range t.FieldTypes {
		value, err := e.Decode(fieldType)
		if err != nil {
			return values.Composite{}, err
		}

		fields[i] = value
	}

	return values.Composite{Fields: fields}, nil
}

func (e *Decoder) DecodeEvent(t types.Event) (values.Event, error) {
	fields := make([]values.Value, len(t.FieldTypes))

	for i, field := range t.FieldTypes {
		value, err := e.Decode(field.Type)
		if err != nil {
			return values.Event{}, err
		}

		fields[i] = value
	}

	return values.Event{
		Identifier: t.Identifier,
		Fields:     fields,
	}, nil
}
