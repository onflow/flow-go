package encoding

import (
	"bytes"
	"fmt"
	"io"

	xdr "github.com/davecgh/go-xdr/xdr2"

	"github.com/dapperlabs/flow-go/sdk/abi/values"
)

type Encoder struct {
	enc *xdr.Encoder
}

func Encode(v values.Value) ([]byte, error) {
	var w bytes.Buffer
	enc := NewEncoder(&w)

	err := enc.Encode(v)
	if err != nil {
		return nil, err
	}

	return w.Bytes(), nil
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{xdr.NewEncoder(w)}
}

func (e *Encoder) Encode(v values.Value) error {
	// TODO: implement remaining types
	switch x := v.(type) {
	case values.String:
		return e.EncodeString(x)
	case values.Int:
		return e.EncodeInt(x)
	case values.Bytes:
		return e.EncodeBytes(x)
	case values.Composite:
		return e.EncodeComposite(x)
	case values.Event:
		return e.EncodeEvent(x)
	case values.Address:
		return e.EncodeAddress(x)
	default:
		return fmt.Errorf("unsupported value: %T, %v", v, v)
	}

	return nil
}

func (e *Encoder) EncodeString(v values.String) error {
	_, err := e.enc.EncodeString(string(v))
	return err
}

func (e *Encoder) EncodeInt(v values.Int) error {
	_, err := e.enc.EncodeInt(int32(v))
	return err
}

func (e *Encoder) EncodeBytes(v values.Bytes) error {
	_, err := e.enc.EncodeOpaque(v)
	return err
}

func (e *Encoder) EncodeComposite(v values.Composite) error {
	for _, value := range v.Fields {
		if err := e.Encode(value); err != nil {
			return err
		}
	}

	return nil
}

func (e *Encoder) EncodeEvent(v values.Event) error {
	for _, field := range v.Fields {
		if err := e.Encode(field); err != nil {
			return err
		}
	}

	return nil
}

func (e *Encoder) EncodeAddress(v values.Address) error {
	_, err := e.enc.EncodeFixedOpaque(v[:])
	return err
}
