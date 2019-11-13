package encode

import (
	"bytes"
	"fmt"
	"io"

	xdr "github.com/davecgh/go-xdr/xdr2"

	"github.com/dapperlabs/flow-go/language/runtime/types"
	"github.com/dapperlabs/flow-go/language/runtime/values"
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
		return e.DecodeString(x)
	case types.Composite:
		return e.DecodeComposite(x)
	default:
		return nil, fmt.Errorf("unsupported type: %T", t)
	}

	return nil, nil
}

func (e *Decoder) DecodeString(t types.String) (values.String, error) {
	str, _, err := e.dec.DecodeString()
	if err != nil {
		return "", err
	}

	return values.String(str), nil
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
