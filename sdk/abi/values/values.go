package values

import "math/big"

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

func (v Int) ToInt() int {
	return int(v.Int.Int64())
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

type Uint8 uint8

func (Uint8) isValue() {}
func (i Uint8) ToGoValue() interface{} {
	return uint8(i)
}

type Uint16 uint16

func (Uint16) isValue() {}
func (i Uint16) ToGoValue() interface{} {
	return uint16(i)
}

type Uint32 uint32

func (Uint32) isValue() {}
func (i Uint32) ToGoValue() interface{} {
	return uint32(i)
}

type Uint64 uint64

func (Uint64) isValue() {}
func (i Uint64) ToGoValue() interface{} {
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
