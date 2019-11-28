package values

import (
	"math/big"

	"github.com/dapperlabs/flow-go/sdk/abi/types"
)

type Value interface {
	isValue()
	Type() types.Type
}

type TypeAssignableValue interface {
	Value
	SetType(types.Type)
}

type typeAssignableValue struct {
	typ types.Type
}

func (typeAssignableValue) isValue() {}

func (v typeAssignableValue) Type() types.Type {
	return v.typ
}

func (v typeAssignableValue) SetType(typ types.Type) {
	v.typ = typ
}

// WithType assigns a type to a type-assignable value.
func WithType(typ types.Type, v TypeAssignableValue) Value {
	v.SetType(typ)
	return v
}

type Void struct{}

func NewVoid() Void {
	return Void{}
}

func (Void) isValue() {}

func (Void) Type() types.Type {
	return types.Void{}
}

type Nil struct{}

func NewNil() Nil {
	return Nil{}
}

func (Nil) isValue() {}

func (Nil) Type() types.Type {
	return nil
}

type Bool bool

func NewBool(b bool) Bool {
	return Bool(b)
}

func (Bool) isValue() {}

func (Bool) Type() types.Type {
	return types.Bool{}
}

type String string

func NewString(s string) String {
	return String(s)
}

func (String) isValue() {}

func (String) Type() types.Type {
	return types.String{}
}

type Bytes []byte

func NewBytes(b []byte) Bytes {
	return b
}

func (Bytes) isValue() {}

func (Bytes) Type() types.Type {
	return types.Bytes{}
}

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

func (v Address) Bytes() []byte {
	return v[:]
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

func (v Int) Int() int {
	return int(v.Value.Int64())
}

func (v Int) Big() *big.Int {
	return v.Value
}

type Int8 int8

func NewInt8(i int8) Int8 {
	return Int8(i)
}

func (Int8) isValue() {}

func (Int8) Type() types.Type {
	return types.Int8{}
}

type Int16 int16

func NewInt16(i int16) Int16 {
	return Int16(i)
}

func (Int16) isValue() {}

func (Int16) Type() types.Type {
	return types.Int16{}
}

type Int32 int32

func NewInt32(i int32) Int32 {
	return Int32(i)
}

func (Int32) isValue() {}

func (Int32) Type() types.Type {
	return types.Int32{}
}

type Int64 int64

func NewInt64(i int64) Int64 {
	return Int64(i)
}

func (Int64) isValue() {}

func (Int64) Type() types.Type {
	return types.Int64{}
}

type Uint8 uint8

func NewUint8(i uint8) Uint8 {
	return Uint8(i)
}

func (Uint8) isValue() {}

func (Uint8) Type() types.Type {
	return types.Uint8{}
}

type Uint16 uint16

func NewUint16(i uint16) Uint16 {
	return Uint16(i)
}

func (Uint16) isValue() {}

func (Uint16) Type() types.Type {
	return types.Uint16{}
}

type Uint32 uint32

func NewUint32(i uint32) Uint32 {
	return Uint32(i)
}

func (Uint32) isValue() {}

func (Uint32) Type() types.Type {
	return types.Uint32{}
}

type Uint64 uint64

func NewUint64(i uint64) Uint64 {
	return Uint64(i)
}

func (Uint64) isValue() {}

func (Uint64) Type() types.Type {
	return types.Uint64{}
}

type VariableSizedArray struct {
	typeAssignableValue
	Values []Value
}

func NewVariableSizedArray(values []Value) VariableSizedArray {
	return VariableSizedArray{Values: values}
}

type ConstantSizedArray struct {
	typeAssignableValue
	Values []Value
}

func NewConstantSizedArray(values []Value) ConstantSizedArray {
	return ConstantSizedArray{Values: values}
}

type Dictionary struct {
	typeAssignableValue
	Pairs []KeyValuePair
}

func NewDictionary(pairs []KeyValuePair) Dictionary {
	return Dictionary{Pairs: pairs}
}

type KeyValuePair struct {
	Key   Value
	Value Value
}

type Event struct {
	typeAssignableValue
	Fields []Value
}

func NewEvent(fields []Value) Event {
	return Event{Fields: fields}
}

type Composite struct {
	typeAssignableValue
	Fields []Value
}

func NewComposite(fields []Value) Composite {
	return Composite{Fields: fields}
}
