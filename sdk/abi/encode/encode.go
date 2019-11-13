package encode

import (
	"bytes"
	"fmt"
	"io"

	xdr "github.com/davecgh/go-xdr/xdr2"

	"github.com/dapperlabs/flow-go/language/runtime/values"
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
	switch x := v.(type) {
	case values.Composite:
		return e.EncodeComposite(x)
	case values.String:
		return e.EncodeString(x)
	default:
		return fmt.Errorf("unsupported value: %T, %v", v, v)
	}

	return nil
}

func (e *Encoder) EncodeString(v values.String) error {
	_, err := e.enc.EncodeString(string(v))
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
