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
	switch x := v.(type) {
	case values.Void:
		return e.EncodeVoid()
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
	case values.Uint8:
		return e.EncodeUint8(x)
	case values.Uint16:
		return e.EncodeUint16(x)
	case values.Uint32:
		return e.EncodeUint32(x)
	case values.Uint64:
		return e.EncodeUint64(x)
	case values.VariableSizedArray:
		return e.EncodeVariableSizedArray(x)
	case values.ConstantSizedArray:
		return e.EncodeConstantSizedArray(x)
	case values.Dictionary:
		return e.EncodeDictionary(x)
	case values.Composite:
		return e.EncodeComposite(x)
	case values.Event:
		return e.EncodeEvent(x)
	default:
		return fmt.Errorf("unsupported value: %T, %v", v, v)
	}

	return nil
}

func (e *Encoder) EncodeVoid() error {
	// void values are not encoded
	return nil
}

func (e *Encoder) EncodeBool(v values.Bool) error {
	_, err := e.enc.EncodeBool(bool(v))
	return err
}

func (e *Encoder) EncodeString(v values.String) error {
	_, err := e.enc.EncodeString(string(v))
	return err
}

func (e *Encoder) EncodeInt(v values.Int) error {
	_, err := e.enc.EncodeInt(int32(v))
	return err
}

func (e *Encoder) EncodeInt8(v values.Int8) error {
	_, err := e.enc.EncodeInt(int32(v))
	return err
}

func (e *Encoder) EncodeInt16(v values.Int16) error {
	_, err := e.enc.EncodeInt(int32(v))
	return err
}

func (e *Encoder) EncodeInt32(v values.Int32) error {
	_, err := e.enc.EncodeInt(int32(v))
	return err
}

func (e *Encoder) EncodeInt64(v values.Int64) error {
	_, err := e.enc.EncodeHyper(int64(v))
	return err
}

func (e *Encoder) EncodeUint8(v values.Uint8) error {
	_, err := e.enc.EncodeUint(uint32(v))
	return err
}

func (e *Encoder) EncodeUint16(v values.Uint16) error {
	_, err := e.enc.EncodeUint(uint32(v))
	return err
}

func (e *Encoder) EncodeUint32(v values.Uint32) error {
	_, err := e.enc.EncodeUint(uint32(v))
	return err
}

func (e *Encoder) EncodeUint64(v values.Uint64) error {
	_, err := e.enc.EncodeUhyper(uint64(v))
	return err
}

// Reference:
// 	RFC Section 4.13 - Variable-Length Array
// 	Unsigned integer length followed by individually XDR encoded array elements
func (e *Encoder) EncodeVariableSizedArray(v values.VariableSizedArray) error {
	size := uint32(len(v))

	_, err := e.enc.EncodeUint(size)
	if err != nil {
		return err
	}

	return e.EncodeConstantSizedArray(values.ConstantSizedArray(v))
}

func (e *Encoder) EncodeConstantSizedArray(v values.ConstantSizedArray) error {
	for _, value := range v {
		if err := e.Encode(value); err != nil {
			return err
		}
	}

	return nil
}

func (e *Encoder) EncodeBytes(v values.Bytes) error {
	_, err := e.enc.EncodeOpaque(v)
	return err
}

func (e *Encoder) EncodeAddress(v values.Address) error {
	_, err := e.enc.EncodeFixedOpaque(v[:])
	return err
}

func (e *Encoder) EncodeDictionary(v values.Dictionary) error {
	size := uint32(len(v))

	// keys and values are encoded as separate fixed-length arrays
	keys := make(values.ConstantSizedArray, 0, size)
	elements := make(values.ConstantSizedArray, 0, size)

	for key, element := range v {
		keys = append(keys, key)
		elements = append(elements, element)
	}

	_, err := e.enc.EncodeUint(size)
	if err != nil {
		return err
	}

	// encode keys
	if err := e.EncodeConstantSizedArray(keys); err != nil {
		return err
	}

	// encode elements
	if err := e.EncodeConstantSizedArray(elements); err != nil {
		return err
	}

	return nil
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
