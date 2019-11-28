package values

import (
	"math/big"

	"github.com/dapperlabs/flow-go/sdk/abi/types"
)

type Value interface {
	isValue()
	Type() types.Type
	setType(p types.Type)
}

// WithType annotates a value with a type.
func WithType(t types.Type, v Value) Value {
	v.setType(t)
	return v
}

type baseValue struct {
	valueType types.Type
}

func (baseValue) isValue() {}

func (v baseValue) Type() types.Type { return v.valueType }
func (v baseValue) setType(t types.Type) {
	v.valueType = t
}

type Void struct{ baseValue }

type Nil struct{ baseValue }

type Bool struct {
	baseValue
	val bool
}

func NewBool(b bool) Bool {
	return Bool{val: b}
}

func (v Bool) Bool() bool {
	return v.val
}

type String struct {
	baseValue
	val string
}

func NewString(s string) String {
	return String{val: s}
}

func (v String) String() string {
	return v.val
}

type Bytes struct {
	baseValue
	val []byte
}

func NewBytes(b []byte) Bytes {
	return Bytes{val: b}
}

func (v Bytes) Bytes() []byte {
	return v.val
}

type Int struct {
	baseValue
	val *big.Int
}

func NewInt(i int) Int {
	return Int{
		val: big.NewInt(int64(i)),
	}
}

func NewIntFromBig(i *big.Int) Int {
	return Int{val: i}
}

func (v Int) Int() int {
	return int(v.val.Int64())
}

func (v Int) Big() *big.Int {
	return v.val
}

type Int8 struct {
	baseValue
	val int8
}

func NewInt8(i int8) Int8 {
	return Int8{val: i}
}

func (v Int8) Int8() int8 {
	return v.val
}

type Int16 struct {
	baseValue
	val int16
}

func NewInt16(i int16) Int16 {
	return Int16{val: i}
}

func (v Int16) Int16() int16 {
	return v.val
}

type Int32 struct {
	baseValue
	val int32
}

func NewInt32(i int32) Int32 {
	return Int32{val: i}
}

func (v Int32) Int32() int32 {
	return v.val
}

type Int64 struct {
	baseValue
	val int64
}

func NewInt64(i int64) Int64 {
	return Int64{val: i}
}

func (v Int64) Int64() int64 {
	return v.val
}

type Uint8 struct {
	baseValue
	val uint8
}

func NewUint8(i uint8) Uint8 {
	return Uint8{val: i}
}

func (v Uint8) Uint8() uint8 {
	return v.val
}

type Uint16 struct {
	baseValue
	val uint16
}

func NewUint16(i uint16) Uint16 {
	return Uint16{val: i}
}

func (v Uint16) Uint16() uint16 {
	return v.val
}

type Uint32 struct {
	baseValue
	val uint32
}

func NewUint32(i uint32) Uint32 {
	return Uint32{val: i}
}

func (v Uint32) Uint32() uint32 {
	return v.val
}

type Uint64 struct {
	baseValue
	val uint64
}

func NewUint64(i uint64) Uint64 {
	return Uint64{val: i}
}

func (v Uint64) Uint64() uint64 {
	return v.val
}

type VariableSizedArray struct {
	baseValue
	val []Value
}

func NewVariableSizedArray(values []Value) VariableSizedArray {
	return VariableSizedArray{val: values}
}

func (v VariableSizedArray) Values() []Value {
	return v.val
}

type ConstantSizedArray struct {
	baseValue
	val []Value
}

func NewConstantSizedArray(values []Value) ConstantSizedArray {
	return ConstantSizedArray{val: values}
}

func (v ConstantSizedArray) Values() []Value {
	return v.val
}

type Dictionary struct {
	baseValue
	pairs []KeyValuePair
}

func NewDictionary(pairs []KeyValuePair) Dictionary {
	return Dictionary{pairs: pairs}
}

func (v Dictionary) Pairs() []KeyValuePair {
	return v.pairs
}

type KeyValuePair struct {
	Key   Value
	Value Value
}

type Composite struct {
	baseValue
	fields []Value
}

func NewComposite(fields []Value) Composite {
	return Composite{fields: fields}
}

func (v Composite) Fields() []Value {
	return v.fields
}

type Event struct {
	baseValue
	fields []Value
}

func NewEvent(fields []Value) Event {
	return Event{fields: fields}
}

func (v Event) Fields() []Value {
	return v.fields
}

type Address struct {
	baseValue
	val [20]byte
}

func (v Address) Bytes() []byte {
	return v.val[:]
}

func NewAddress(b []byte) Address {
	var a [20]byte
	copy(a[:], b)
	return Address{val: a}
}
