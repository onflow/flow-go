package encode

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
	case types.String:
		return e.DecodeString()
	case types.Int:
		return e.DecodeInt()
	case types.Bytes:
		return e.DecodeBytes()
	case types.Composite:
		return e.DecodeComposite(x)
	case types.Event:
		return e.DecodeEvent(x)
	case types.Address:
		return e.DecodeAddress()
	default:
		return nil, fmt.Errorf("unsupported type: %T", t)
	}

	return nil, nil
}

func (e *Decoder) DecodeString() (values.String, error) {
	str, _, err := e.dec.DecodeString()
	if err != nil {
		return "", err
	}

	return values.String(str), nil
}

func (e *Decoder) DecodeInt() (values.Int, error) {
	i, _, err := e.dec.DecodeInt()
	if err != nil {
		return 0, err
	}

	return values.Int(i), nil
}

func (e *Decoder) DecodeBytes() (values.Bytes, error) {
	b, _, err := e.dec.DecodeOpaque()
	if err != nil {
		return nil, err
	}

	return b, nil
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
	fields := make([]values.EventField, len(t.FieldTypes))

	for i, field := range t.FieldTypes {
		value, err := e.Decode(field.Type)
		if err != nil {
			return values.Event{}, err
		}

		fields[i] = values.EventField{
			Identifier: field.Identifier,
			Value:      value,
		}
	}

	return values.Event{
		// TODO: is this field needed?
		Identifier: "",
		Fields:     fields,
	}, nil
}

func (e *Decoder) DecodeAddress() (values.Address, error) {
	b, _, err := e.dec.DecodeFixedOpaque(20)
	if err != nil {
		return values.Address{}, err
	}

	return values.BytesToAddress(b), nil
}
