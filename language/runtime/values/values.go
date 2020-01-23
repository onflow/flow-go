package values

import (
	"fmt"
	"math/big"

	"github.com/dapperlabs/flow-go/language/runtime/types"
)

type Value interface {
	isValue()
	Type() types.Type
	ToGoValue() interface{}
}

type Void struct{}

func NewVoid() Void {
	return Void{}
}

func (Void) isValue() {}

func (Void) Type() types.Type {
	return types.Void{}
}

func (v Void) WithType(types.Type) Value { return v }

func (Void) ToGoValue() interface{} {
	return nil
}

type Nil struct{}

func NewNil() Nil {
	return Nil{}
}

func (Nil) isValue() {}

func (Nil) Type() types.Type {
	return nil
}

func (v Nil) WithType(types.Type) Value { return v }

func (Nil) ToGoValue() interface{} {
	return nil
}

type Optional struct {
	Value Value
}

func NewOptional(value Value) Optional {
	return Optional{Value: value}
}

func (Optional) isValue() {}

func (Optional) Type() types.Type {
	return nil
}

func (o Optional) ToGoValue() interface{} {
	if o.Value == nil {
		return nil
	}

	value := o.Value.ToGoValue()

	return value
}

type Bool bool

func NewBool(b bool) Bool {
	return Bool(b)
}

func (Bool) isValue() {}

func (Bool) Type() types.Type {
	return types.Bool{}
}

func (v Bool) WithType(types.Type) Value { return v }

func (v Bool) ToGoValue() interface{} {
	return bool(v)
}

type String string

func NewString(s string) String {
	return String(s)
}

func (String) isValue() {}

func (String) Type() types.Type {
	return types.String{}
}

func (v String) WithType(types.Type) Value { return v }

func (v String) ToGoValue() interface{} {
	return string(v)
}

type Bytes []byte

func NewBytes(b []byte) Bytes {
	return b
}

func (Bytes) isValue() {}

func (Bytes) Type() types.Type {
	return types.Bytes{}
}

func (v Bytes) ToGoValue() interface{} {
	return []byte(v)
}

func (v Bytes) WithType(types.Type) Value { return v }

const AddressLength = 20

type Address [AddressLength]byte

func NewAddress(b [AddressLength]byte) Address {
	return b
}

func NewAddressFromBytes(b []byte) Address {
	var a Address
	copy(a[:], b)
	return a
}

func (Address) isValue() {}

func (Address) Type() types.Type {
	return types.Address{}
}

func (v Address) WithType(types.Type) Value { return v }

func (v Address) ToGoValue() interface{} {
	return [20]byte(v)
}

func (v Address) Bytes() []byte {
	return v[:]
}

func (v Address) String() string {
	return v.Hex()
}

func (v Address) Hex() string {
	return fmt.Sprintf("%x", [AddressLength]byte(v))
}

func BytesToAddress(b []byte) Address {
	var a Address
	copy(a[AddressLength-len(b):AddressLength], b)
	return a
}

type Int struct {
	Value *big.Int
}

func NewInt(i int) Int {
	return Int{big.NewInt(int64(i))}
}

func NewIntFromBig(i *big.Int) Int {
	return Int{i}
}

func (Int) isValue() {}

func (Int) Type() types.Type {
	return nil
}

func (v Int) WithType(types.Type) Value { return v }

func (v Int) ToGoValue() interface{} {
	return v.Int()
}

func (v Int) Int() int {
	return int(v.Value.Int64())
}

func (v Int) Big() *big.Int {
	return v.Value
}

type Int8 int8

func NewInt8(v int8) Int8 {
	return Int8(v)
}

func (Int8) isValue() {}

func (v Int8) ToGoValue() interface{} {
	return int8(v)
}

func (Int8) Type() types.Type {
	return types.Int8{}
}

func (v Int8) WithType(types.Type) Value { return v }

type Int16 int16

func NewInt16(v int16) Int16 {
	return Int16(v)
}

func (Int16) isValue() {}

func (Int16) Type() types.Type {
	return types.Int16{}
}

func (v Int16) WithType(types.Type) Value { return v }

func (v Int16) ToGoValue() interface{} {
	return int16(v)
}

type Int32 int32

func NewInt32(v int32) Int32 {
	return Int32(v)
}

func (Int32) isValue() {}

func (Int32) Type() types.Type {
	return types.Int32{}
}

func (v Int32) WithType(types.Type) Value { return v }

func (v Int32) ToGoValue() interface{} {
	return int32(v)
}

type Int64 int64

func NewInt64(i int64) Int64 {
	return Int64(i)
}

func (Int64) isValue() {}

func (Int64) Type() types.Type {
	return types.Int64{}
}

func (v Int64) WithType(types.Type) Value { return v }

func (v Int64) ToGoValue() interface{} {
	return int64(v)
}

type UInt8 uint8

func NewUInt8(v uint8) UInt8 {
	return UInt8(v)
}

func (UInt8) isValue() {}

func (UInt8) Type() types.Type {
	return types.UInt8{}
}

func (v UInt8) WithType(types.Type) Value { return v }

func (v UInt8) ToGoValue() interface{} {
	return uint8(v)
}

type UInt16 uint16

func NewUInt16(v uint16) UInt16 {
	return UInt16(v)
}

func (UInt16) isValue() {}

func (UInt16) Type() types.Type {
	return types.UInt16{}
}

func (v UInt16) WithType(types.Type) Value { return v }

func (v UInt16) ToGoValue() interface{} {
	return uint16(v)
}

type UInt32 uint32

func NewUInt32(v uint32) UInt32 {
	return UInt32(v)
}

func (UInt32) isValue() {}

func (UInt32) Type() types.Type {
	return types.UInt32{}
}

func (v UInt32) WithType(types.Type) Value { return v }

func (v UInt32) ToGoValue() interface{} {
	return uint32(v)
}

type UInt64 uint64

func NewUInt64(v uint64) UInt64 {
	return UInt64(v)
}

func (UInt64) isValue() {}

func (UInt64) Type() types.Type {
	return types.UInt64{}
}

func (v UInt64) WithType(types.Type) Value { return v }

func (v UInt64) ToGoValue() interface{} {
	return uint64(v)
}

type VariableSizedArray struct {
	typ    types.Type
	Values []Value
}

func NewVariableSizedArray(values []Value) VariableSizedArray {
	return VariableSizedArray{Values: values}
}

func (VariableSizedArray) isValue() {}

func (v VariableSizedArray) Type() types.Type { return v.typ }

func (v VariableSizedArray) WithType(typ types.Type) VariableSizedArray {
	v.typ = typ
	return v
}

func (v VariableSizedArray) ToGoValue() interface{} {
	ret := make([]interface{}, len(v.Values))

	for i, e := range v.Values {
		ret[i] = e.ToGoValue()
	}

	return ret
}

type ConstantSizedArray struct {
	typ    types.Type
	Values []Value
}

func NewConstantSizedArray(values []Value) ConstantSizedArray {
	return ConstantSizedArray{Values: values}
}

func (ConstantSizedArray) isValue() {}

func (v ConstantSizedArray) Type() types.Type { return v.typ }

func (v ConstantSizedArray) WithType(typ types.Type) ConstantSizedArray {
	v.typ = typ
	return v
}

func (v ConstantSizedArray) ToGoValue() interface{} {
	ret := make([]interface{}, len(v.Values))

	for i, e := range v.Values {
		ret[i] = e.ToGoValue()
	}

	return ret
}

type Dictionary struct {
	typ   types.Type
	Pairs []KeyValuePair
}

func NewDictionary(pairs []KeyValuePair) Dictionary {
	return Dictionary{Pairs: pairs}
}

func (Dictionary) isValue() {}

func (v Dictionary) Type() types.Type { return v.typ }

func (v Dictionary) WithType(typ types.Type) Dictionary {
	v.typ = typ
	return v
}

func (v Dictionary) ToGoValue() interface{} {
	ret := map[interface{}]interface{}{}

	for _, p := range v.Pairs {
		ret[p.Key.ToGoValue()] = p.Value.ToGoValue()
	}

	return ret
}

type KeyValuePair struct {
	Key   Value
	Value Value
}

type Composite struct {
	typ    types.Type
	Fields []Value
}

func NewComposite(fields []Value) Composite {
	return Composite{Fields: fields}
}

func (Composite) isValue() {}

func (v Composite) Type() types.Type { return v.typ }

func (v Composite) WithType(typ types.Type) Composite {
	v.typ = typ
	return v
}

func (v Composite) ToGoValue() interface{} {
	ret := make([]interface{}, len(v.Fields))

	for i, field := range v.Fields {
		ret[i] = field.ToGoValue()
	}

	return ret
}
