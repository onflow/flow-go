package values

import (
	"fmt"
	"math/big"
)

type Value interface {
	isValue()
	ToGoValue() interface{}
}

type Void struct{}

func (Void) isValue() {}
func (Void) ToGoValue() interface{} {
	return nil
}

type Nil struct{}

func (Nil) isValue() {}

func (Nil) ToGoValue() interface{} {
	return nil
}

type Bool bool

func (Bool) isValue() {}

func (b Bool) ToGoValue() interface{} {
	return bool(b)
}

type String string

func (String) isValue() {}

func (s String) ToGoValue() interface{} {
	return string(s)
}

type Bytes []byte

func (Bytes) isValue() {}

func (b Bytes) ToGoValue() interface{} {
	return []byte(b)
}

type Int struct {
	Int *big.Int
}

func NewInt(v int) Int {
	return Int{big.NewInt(int64(v))}
}

func NewIntFromBig(v *big.Int) Int {
	return Int{v}
}

func (i Int) ToInt() int {
	return int(i.Int.Int64())
}

func (Int) isValue() {}
func (i Int) ToGoValue() interface{} {
	return i.ToInt()
}

type Int8 int8

func (Int8) isValue() {}
func (i Int8) ToGoValue() interface{} {
	return int8(i)
}

type Int16 int16

func (Int16) isValue() {}
func (i Int16) ToGoValue() interface{} {
	return int16(i)
}

type Int32 int32

func (Int32) isValue() {}
func (i Int32) ToGoValue() interface{} {
	return int32(i)
}

type Int64 int64

func (Int64) isValue() {}
func (i Int64) ToGoValue() interface{} {
	return int64(i)
}

type UInt8 uint8

func (UInt8) isValue() {}
func (i UInt8) ToGoValue() interface{} {
	return uint8(i)
}

type UInt16 uint16

func (UInt16) isValue() {}
func (i UInt16) ToGoValue() interface{} {
	return uint16(i)
}

type UInt32 uint32

func (UInt32) isValue() {}
func (i UInt32) ToGoValue() interface{} {
	return uint32(i)
}

type UInt64 uint64

func (UInt64) isValue() {}
func (i UInt64) ToGoValue() interface{} {
	return uint64(i)
}

type VariableSizedArray []Value

func (VariableSizedArray) isValue() {}
func (a VariableSizedArray) ToGoValue() interface{} {
	ret := make([]interface{}, len(a))
	for i, v := range a {
		ret[i] = v.ToGoValue()
	}
	return ret
}

type ConstantSizedArray []Value

func (ConstantSizedArray) isValue() {}
func (a ConstantSizedArray) ToGoValue() interface{} {
	ret := make([]interface{}, len(a))
	for i, v := range a {
		ret[i] = v.ToGoValue()
	}
	return ret
}

type Dictionary []KeyValuePair

func (Dictionary) isValue() {}
func (d Dictionary) ToGoValue() interface{} {
	ret := map[interface{}]interface{}{}
	for _, v := range d {
		ret[v.Key.ToGoValue()] = v.Value.ToGoValue()
	}
	return ret
}

type KeyValuePair struct {
	Key   Value
	Value Value
}

type Event struct {
	// TODO: is the Identifier field needed here?
	Identifier string
	Fields     []Value
}

func (e Event) ToGoValue() interface{} {
	ret := make([]interface{}, len(e.Fields))
	for i, v := range e.Fields {
		ret[i] = v.ToGoValue()
	}
	return ret
}

type Optional struct {
	Value Value
}

func (Optional) isValue() {}

func (o Optional) ToGoValue() interface{} {
	if o.Value == nil {
		return nil
	}
	return o.Value.ToGoValue()
}

type Composite struct {
	Fields []Value
}

func (Composite) isValue() {}
func (c Composite) ToGoValue() interface{} {
	ret := make([]interface{}, len(c.Fields))
	for i, v := range c.Fields {
		ret[i] = v.ToGoValue()
	}
	return ret
}

func (Event) isValue() {}

type Address [20]byte

func (Address) isValue() {}
func (a Address) ToGoValue() interface{} {
	return [20]byte(a)
}

func BytesToAddress(b []byte) Address {
	var a Address
	copy(a[:], b)
	return a
}

func NewValue(value interface{}) (Value, error) {

	switch v := value.(type) {
	case string:
		ret := String(v)
		return &ret, nil
	case int:
		ret := NewInt(v)
		return &ret, nil
	case int8:
		ret := Int8(v)
		return &ret, nil
	case int16:
		ret := Int16(v)
		return &ret, nil
	case int32:
		ret := Int32(v)
		return &ret, nil
	case int64:
		ret := Int64(v)
		return &ret, nil
	case uint8:
		ret := UInt8(v)
		return &ret, nil
	case uint16:
		ret := UInt16(v)
		return &ret, nil
	case uint32:
		ret := UInt32(v)
		return &ret, nil
	case uint64:
		ret := UInt64(v)
		return &ret, nil
	case []interface{}:
		values := make([]Value, len(v))
		for i, v := range v {
			t, err := NewValue(v)
			if err != nil {
				return nil, err
			}
			values[i] = t
		}
		ret := VariableSizedArray(values)
		return &ret, nil
	case nil:
		ret := Nil{}
		return &ret, nil

	}

	return nil, fmt.Errorf("value type %T cannot be converted to ABI Value type", value)
}

// NewValueOrPanic is convenience function when failure is really unexpected
// like generated Go code
func NewValueOrPanic(value interface{}) Value {
	ret, err := NewValue(value)
	if err != nil {
		panic(err)
	}
	return ret
}
