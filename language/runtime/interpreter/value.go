package interpreter

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/rivo/uniseg"
	"golang.org/x/text/unicode/norm"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/trampoline"
)

type Value interface {
	IsValue()
	DynamicType(interpreter *Interpreter) DynamicType
	Copy() Value
	GetOwner() *common.Address
	SetOwner(address *common.Address)
}

// ValueIndexableValue

type ValueIndexableValue interface {
	Get(interpreter *Interpreter, locationRange LocationRange, key Value) Value
	Set(interpreter *Interpreter, locationRange LocationRange, key Value, value Value)
}

// MemberAccessibleValue

type MemberAccessibleValue interface {
	GetMember(interpreter *Interpreter, locationRange LocationRange, name string) Value
	SetMember(interpreter *Interpreter, locationRange LocationRange, name string, value Value)
}

// ConcatenatableValue

type ConcatenatableValue interface {
	Concat(other ConcatenatableValue) Value
}

// EquatableValue

type EquatableValue interface {
	Value
	Equal(other Value) BoolValue
}

// DestroyableValue

type DestroyableValue interface {
	Destroy(*Interpreter, LocationPosition) trampoline.Trampoline
}

// HasKeyString

type HasKeyString interface {
	KeyString() string
}

// VoidValue

type VoidValue struct{}

func init() {
	gob.Register(VoidValue{})
}

func (VoidValue) IsValue() {}

func (VoidValue) DynamicType(_ *Interpreter) DynamicType {
	return VoidType{}
}

func (v VoidValue) Copy() Value {
	return v
}

func (VoidValue) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (VoidValue) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (VoidValue) String() string {
	return "()"
}

// BoolValue

type BoolValue bool

func init() {
	gob.Register(BoolValue(true))
}

func (BoolValue) IsValue() {}

func (BoolValue) DynamicType(_ *Interpreter) DynamicType {
	return BoolType{}
}

func (v BoolValue) Copy() Value {
	return v
}

func (BoolValue) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (BoolValue) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v BoolValue) Negate() BoolValue {
	return !v
}

func (v BoolValue) Equal(other Value) BoolValue {
	otherBool, ok := other.(BoolValue)
	if !ok {
		return false
	}
	return bool(v) == bool(otherBool)
}

func (v BoolValue) String() string {
	return strconv.FormatBool(bool(v))
}

func (v BoolValue) KeyString() string {
	return v.String()
}

// StringValue

type StringValue struct {
	Str string
}

func init() {
	gob.Register(&StringValue{})
}

func NewStringValue(str string) *StringValue {
	return &StringValue{str}
}

func (*StringValue) IsValue() {}

func (*StringValue) DynamicType(_ *Interpreter) DynamicType {
	return StringType{}
}

func (v *StringValue) Copy() Value {
	return &StringValue{Str: v.Str}
}

func (*StringValue) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (*StringValue) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v *StringValue) String() string {
	// TODO: quote like in string literal
	return strconv.Quote(v.Str)
}

func (v *StringValue) KeyString() string {
	return v.Str
}

func (v *StringValue) Equal(other Value) BoolValue {
	otherString, ok := other.(*StringValue)
	if !ok {
		return false
	}
	return v.NormalForm() == otherString.NormalForm()
}

func (v *StringValue) NormalForm() string {
	return norm.NFC.String(v.Str)
}

func (v *StringValue) Concat(other ConcatenatableValue) Value {
	otherString := other.(*StringValue)

	var sb strings.Builder

	sb.WriteString(v.Str)
	sb.WriteString(otherString.Str)

	return NewStringValue(sb.String())
}

func (v *StringValue) Slice(from IntValue, to IntValue) Value {
	fromInt := from.IntValue()
	toInt := to.IntValue()
	return NewStringValue(v.Str[fromInt:toInt])
}

func (v *StringValue) Get(_ *Interpreter, _ LocationRange, key Value) Value {
	i := key.(NumberValue).IntValue()

	// TODO: optimize grapheme clusters to prevent unnecessary iteration
	graphemes := uniseg.NewGraphemes(v.Str)
	graphemes.Next()

	for j := 0; j < i; j++ {
		graphemes.Next()
	}

	char := graphemes.Str()

	return NewStringValue(char)
}

func (v *StringValue) Set(_ *Interpreter, _ LocationRange, key Value, value Value) {
	i := key.(NumberValue).IntValue()
	char := value.(*StringValue).Str

	str := v.Str

	// TODO: optimize grapheme clusters to prevent unnecessary iteration
	graphemes := uniseg.NewGraphemes(str)
	graphemes.Next()

	for j := 0; j < i; j++ {
		graphemes.Next()
	}

	start, end := graphemes.Positions()

	var sb strings.Builder

	sb.WriteString(str[:start])
	sb.WriteString(char)
	sb.WriteString(str[end:])

	v.Str = sb.String()
}

func (v *StringValue) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {
	case "length":
		count := uniseg.GraphemeClusterCount(v.Str)
		return NewIntValue(int64(count))

	case "concat":
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				otherValue := invocation.Arguments[0].(ConcatenatableValue)
				result := v.Concat(otherValue)
				return trampoline.Done{Result: result}
			},
		)

	case "slice":
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				from := invocation.Arguments[0].(IntValue)
				to := invocation.Arguments[1].(IntValue)
				result := v.Slice(from, to)
				return trampoline.Done{Result: result}
			},
		)

	case "decodeHex":
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				str := v.Str

				bs, err := hex.DecodeString(str)
				if err != nil {
					panic(err)
				}

				values := make([]Value, len(str)/2)
				for i, b := range bs {
					values[i] = NewIntValue(int64(b))
				}
				result := NewArrayValueUnownedNonCopying(values...)

				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (*StringValue) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// ArrayValue

type ArrayValue struct {
	Values []Value
	Owner  *common.Address
}

func init() {
	gob.Register(&ArrayValue{})
}

func NewArrayValueUnownedNonCopying(values ...Value) *ArrayValue {
	// NOTE: new value has no owner

	for _, value := range values {
		value.SetOwner(nil)
	}

	return &ArrayValue{
		Values: values,
		Owner:  nil,
	}
}

func (*ArrayValue) IsValue() {}

func (v *ArrayValue) DynamicType(interpreter *Interpreter) DynamicType {
	elementTypes := make([]DynamicType, len(v.Values))

	for i, value := range v.Values {
		elementTypes[i] = value.DynamicType(interpreter)
	}

	return ArrayType{
		ElementTypes: elementTypes,
	}
}

func (v *ArrayValue) Copy() Value {
	// TODO: optimize, use copy-on-write
	copies := make([]Value, len(v.Values))
	for i, value := range v.Values {
		copies[i] = value.Copy()
	}
	return NewArrayValueUnownedNonCopying(copies...)
}

func (v *ArrayValue) GetOwner() *common.Address {
	return v.Owner
}

func (v *ArrayValue) SetOwner(owner *common.Address) {
	if v.Owner == owner {
		return
	}

	v.Owner = owner

	for _, value := range v.Values {
		value.SetOwner(owner)
	}
}

func (v *ArrayValue) Destroy(interpreter *Interpreter, location LocationPosition) trampoline.Trampoline {
	var result trampoline.Trampoline = trampoline.Done{}
	for _, value := range v.Values {
		result = result.FlatMap(func(_ interface{}) trampoline.Trampoline {
			return value.(DestroyableValue).Destroy(interpreter, location)
		})
	}
	return result
}

func (v *ArrayValue) GobEncode() ([]byte, error) {
	w := new(bytes.Buffer)
	encoder := gob.NewEncoder(w)
	err := encoder.Encode(v.Values)
	if err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

func (v *ArrayValue) GobDecode(buf []byte) error {
	r := bytes.NewBuffer(buf)
	decoder := gob.NewDecoder(r)
	err := decoder.Decode(&v.Values)
	if err != nil {
		return err
	}
	// NOTE: ensure the `Values` slice is properly allocated
	if v.Values == nil {
		v.Values = make([]Value, 0)
	}
	return nil
}

func (v *ArrayValue) Concat(other ConcatenatableValue) Value {
	otherArray := other.(*ArrayValue)
	concatenated := append(v.Copy().(*ArrayValue).Values, otherArray.Values...)
	return NewArrayValueUnownedNonCopying(concatenated...)
}

func (v *ArrayValue) Get(_ *Interpreter, _ LocationRange, key Value) Value {
	integerKey := key.(NumberValue).IntValue()
	return v.Values[integerKey]
}

func (v *ArrayValue) Set(_ *Interpreter, _ LocationRange, key Value, value Value) {
	value.SetOwner(v.Owner)

	integerKey := key.(NumberValue).IntValue()
	v.Values[integerKey] = value
}

func (v *ArrayValue) String() string {
	var builder strings.Builder
	builder.WriteString("[")
	for i, value := range v.Values {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(fmt.Sprint(value))
	}
	builder.WriteString("]")
	return builder.String()
}

func (v *ArrayValue) Append(element Value) {
	element.SetOwner(v.Owner)
	v.Values = append(v.Values, element)
}

func (v *ArrayValue) Insert(i int, element Value) {
	element.SetOwner(v.Owner)
	v.Values = append(v.Values[:i], append([]Value{element}, v.Values[i:]...)...)
}

// TODO: unset owner?
func (v *ArrayValue) Remove(i int) Value {
	result := v.Values[i]

	lastIndex := len(v.Values) - 1
	copy(v.Values[i:], v.Values[i+1:])

	// avoid memory leaks by explicitly setting value to nil
	v.Values[lastIndex] = nil

	v.Values = v.Values[:lastIndex]

	return result
}

// TODO: unset owner?
func (v *ArrayValue) RemoveFirst() Value {
	var firstElement Value
	firstElement, v.Values = v.Values[0], v.Values[1:]
	return firstElement
}

// TODO: unset owner?
func (v *ArrayValue) RemoveLast() Value {
	var lastElement Value
	lastIndex := len(v.Values) - 1
	lastElement, v.Values = v.Values[lastIndex], v.Values[:lastIndex]
	return lastElement
}

func (v *ArrayValue) Contains(needleValue Value) BoolValue {
	needleEquatable := needleValue.(EquatableValue)

	for _, arrayValue := range v.Values {
		if needleEquatable.Equal(arrayValue) {
			return true
		}
	}

	return false
}

func (v *ArrayValue) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {
	case "length":
		return NewIntValue(int64(v.Count()))

	case "append":
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				v.Append(invocation.Arguments[0])
				return trampoline.Done{Result: VoidValue{}}
			},
		)

	case "concat":
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				otherArray := invocation.Arguments[0].(ConcatenatableValue)
				result := v.Concat(otherArray)
				return trampoline.Done{Result: result}
			},
		)

	case "insert":
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				i := invocation.Arguments[0].(NumberValue).IntValue()
				element := invocation.Arguments[1]
				v.Insert(i, element)
				return trampoline.Done{Result: VoidValue{}}
			},
		)

	case "remove":
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				i := invocation.Arguments[0].(NumberValue).IntValue()
				result := v.Remove(i)
				return trampoline.Done{Result: result}
			},
		)

	case "removeFirst":
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := v.RemoveFirst()
				return trampoline.Done{Result: result}
			},
		)

	case "removeLast":
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := v.RemoveLast()
				return trampoline.Done{Result: result}
			},
		)

	case "contains":
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := v.Contains(invocation.Arguments[0])
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (v *ArrayValue) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

func (v *ArrayValue) Count() int {
	return len(v.Values)
}

// NumberValue

type NumberValue interface {
	EquatableValue
	IntValue() int
	Negate() NumberValue
	Plus(other NumberValue) NumberValue
	Minus(other NumberValue) NumberValue
	Mod(other NumberValue) NumberValue
	Mul(other NumberValue) NumberValue
	Div(other NumberValue) NumberValue
	Less(other NumberValue) BoolValue
	LessEqual(other NumberValue) BoolValue
	Greater(other NumberValue) BoolValue
	GreaterEqual(other NumberValue) BoolValue
}

// IntValue

type IntValue struct {
	Int *big.Int
}

func init() {
	gob.Register(IntValue{})
}

func NewIntValue(value int64) IntValue {
	return IntValue{Int: big.NewInt(value)}
}

func ConvertInt(value Value, _ *Interpreter) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	if intValue, ok := value.(IntValue); ok {
		return intValue.Copy()
	}
	return NewIntValue(int64(value.(NumberValue).IntValue()))
}

func (v IntValue) IsValue() {}

func (IntValue) DynamicType(_ *Interpreter) DynamicType {
	return NumberType{&sema.IntType{}}
}

func (v IntValue) Copy() Value {
	return IntValue{big.NewInt(0).Set(v.Int)}
}

func (IntValue) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (IntValue) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v IntValue) IntValue() int {
	// TODO: handle overflow
	return int(v.Int.Int64())
}

func (v IntValue) String() string {
	return v.Int.String()
}

func (v IntValue) KeyString() string {
	return v.Int.String()
}

func (v IntValue) Negate() NumberValue {
	return IntValue{big.NewInt(0).Neg(v.Int)}
}

func (v IntValue) Plus(other NumberValue) NumberValue {
	o := other.(IntValue)
	res := big.NewInt(0)
	res.Add(v.Int, o.Int)
	return IntValue{res}
}

func (v IntValue) Minus(other NumberValue) NumberValue {
	o := other.(IntValue)
	res := big.NewInt(0)
	res.Sub(v.Int, o.Int)
	return IntValue{res}
}

func (v IntValue) Mod(other NumberValue) NumberValue {
	o := other.(IntValue)
	res := big.NewInt(0)
	// INT33-C
	if o.Int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Mod(v.Int, o.Int)
	return IntValue{res}
}

func (v IntValue) Mul(other NumberValue) NumberValue {
	o := other.(IntValue)
	res := big.NewInt(0)
	res.Mul(v.Int, o.Int)
	return IntValue{res}
}

func (v IntValue) Div(other NumberValue) NumberValue {
	o := other.(IntValue)
	res := big.NewInt(0)
	// INT33-C
	if o.Int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Div(v.Int, o.Int)
	return IntValue{res}
}

func (v IntValue) Less(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(IntValue).Int)
	return cmp == -1
}

func (v IntValue) LessEqual(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(IntValue).Int)
	return cmp <= 0
}

func (v IntValue) Greater(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(IntValue).Int)
	return cmp == 1
}

func (v IntValue) GreaterEqual(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(IntValue).Int)
	return cmp >= 0
}

func (v IntValue) Equal(other Value) BoolValue {
	otherInt, ok := other.(IntValue)
	if !ok {
		return false
	}
	cmp := v.Int.Cmp(otherInt.Int)
	return cmp == 0
}

// Int8Value

type Int8Value int8

func init() {
	gob.Register(Int8Value(0))
}

func (Int8Value) IsValue() {}

func (Int8Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberType{&sema.Int8Type{}}
}

func (v Int8Value) Copy() Value {
	return v
}

func (Int8Value) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (Int8Value) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v Int8Value) String() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int8Value) KeyString() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int8Value) IntValue() int {
	return int(v)
}

func (v Int8Value) Negate() NumberValue {
	// INT32-C
	if v == math.MinInt8 {
		panic(&OverflowError{})
	}
	return -v
}

func (v Int8Value) Plus(other NumberValue) NumberValue {
	o := other.(Int8Value)
	// INT32-C
	if (o > 0) && (v > (math.MaxInt8 - o)) {
		panic(&OverflowError{})
	} else if (o < 0) && (v < (math.MinInt8 - o)) {
		panic(&UnderflowError{})
	}
	return v + o
}

func (v Int8Value) Minus(other NumberValue) NumberValue {
	o := other.(Int8Value)
	// INT32-C
	if (o > 0) && (v < (math.MinInt8 + o)) {
		panic(&OverflowError{})
	} else if (o < 0) && (v > (math.MaxInt8 + o)) {
		panic(&UnderflowError{})
	}
	return v - o
}

func (v Int8Value) Mod(other NumberValue) NumberValue {
	o := other.(Int8Value)
	// INT33-C
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v Int8Value) Mul(other NumberValue) NumberValue {
	o := other.(Int8Value)
	// INT32-C
	if v > 0 {
		if o > 0 {
			if v > (math.MaxInt8 / o) {
				panic(&OverflowError{})
			}
		} else {
			if o < (math.MinInt8 / v) {
				panic(&OverflowError{})
			}
		}
	} else {
		if o > 0 {
			if v < (math.MinInt8 / o) {
				panic(&OverflowError{})
			}
		} else {
			if (v != 0) && (o < (math.MaxInt8 / v)) {
				panic(&OverflowError{})
			}
		}
	}
	return v * o
}

func (v Int8Value) Div(other NumberValue) NumberValue {
	o := other.(Int8Value)
	// INT33-C
	// https://golang.org/ref/spec#Integer_operators
	if o == 0 {
		panic(DivisionByZeroError{})
	} else if (v == math.MinInt8) && (o == -1) {
		panic(OverflowError{})
	}
	return v / o
}

func (v Int8Value) Less(other NumberValue) BoolValue {
	return v < other.(Int8Value)
}

func (v Int8Value) LessEqual(other NumberValue) BoolValue {
	return v <= other.(Int8Value)
}

func (v Int8Value) Greater(other NumberValue) BoolValue {
	return v > other.(Int8Value)
}

func (v Int8Value) GreaterEqual(other NumberValue) BoolValue {
	return v >= other.(Int8Value)
}

func (v Int8Value) Equal(other Value) BoolValue {
	otherInt8, ok := other.(Int8Value)
	if !ok {
		return false
	}
	return v == otherInt8
}

func ConvertInt8(value Value, _ *Interpreter) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return Int8Value(value.(NumberValue).IntValue())
}

// Int16Value

type Int16Value int16

func init() {
	gob.Register(Int16Value(0))
}

func (Int16Value) IsValue() {}

func (Int16Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberType{&sema.Int16Type{}}
}

func (v Int16Value) Copy() Value {
	return v
}

func (Int16Value) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (Int16Value) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v Int16Value) String() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int16Value) KeyString() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int16Value) IntValue() int {
	return int(v)
}

func (v Int16Value) Negate() NumberValue {
	// INT32-C
	if v == math.MinInt16 {
		panic(&OverflowError{})
	}
	return -v
}

func (v Int16Value) Plus(other NumberValue) NumberValue {
	o := other.(Int16Value)
	// INT32-C
	if (o > 0) && (v > (math.MaxInt16 - o)) {
		panic(&OverflowError{})
	} else if (o < 0) && (v < (math.MinInt16 - o)) {
		panic(&UnderflowError{})
	}
	return v + o
}

func (v Int16Value) Minus(other NumberValue) NumberValue {
	o := other.(Int16Value)
	// INT32-C
	if (o > 0) && (v < (math.MinInt16 + o)) {
		panic(&OverflowError{})
	} else if (o < 0) && (v > (math.MaxInt16 + o)) {
		panic(&UnderflowError{})
	}
	return v - o
}

func (v Int16Value) Mod(other NumberValue) NumberValue {
	o := other.(Int16Value)
	// INT33-C
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v Int16Value) Mul(other NumberValue) NumberValue {
	o := other.(Int16Value)
	// INT32-C
	if v > 0 {
		if o > 0 {
			if v > (math.MaxInt16 / o) {
				panic(&OverflowError{})
			}
		} else {
			if o < (math.MinInt16 / v) {
				panic(&OverflowError{})
			}
		}
	} else {
		if o > 0 {
			if v < (math.MinInt16 / o) {
				panic(&OverflowError{})
			}
		} else {
			if (v != 0) && (o < (math.MaxInt16 / v)) {
				panic(&OverflowError{})
			}
		}
	}
	return v * o
}

func (v Int16Value) Div(other NumberValue) NumberValue {
	o := other.(Int16Value)
	// INT33-C
	// https://golang.org/ref/spec#Integer_operators
	if o == 0 {
		panic(DivisionByZeroError{})
	} else if (v == math.MinInt16) && (o == -1) {
		panic(OverflowError{})
	}
	return v / o
}

func (v Int16Value) Less(other NumberValue) BoolValue {
	return v < other.(Int16Value)
}

func (v Int16Value) LessEqual(other NumberValue) BoolValue {
	return v <= other.(Int16Value)
}

func (v Int16Value) Greater(other NumberValue) BoolValue {
	return v > other.(Int16Value)
}

func (v Int16Value) GreaterEqual(other NumberValue) BoolValue {
	return v >= other.(Int16Value)
}

func (v Int16Value) Equal(other Value) BoolValue {
	otherInt16, ok := other.(Int16Value)
	if !ok {
		return false
	}
	return v == otherInt16
}

func ConvertInt16(value Value, _ *Interpreter) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return Int16Value(value.(NumberValue).IntValue())
}

// Int32Value

type Int32Value int32

func init() {
	gob.Register(Int32Value(0))
}

func (Int32Value) IsValue() {}

func (Int32Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberType{&sema.Int32Type{}}
}

func (v Int32Value) Copy() Value {
	return v
}

func (Int32Value) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (Int32Value) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v Int32Value) String() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int32Value) KeyString() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int32Value) IntValue() int {
	return int(v)
}

func (v Int32Value) Negate() NumberValue {
	// INT32-C
	if v == math.MinInt32 {
		panic(&OverflowError{})
	}
	return -v
}

func (v Int32Value) Plus(other NumberValue) NumberValue {
	o := other.(Int32Value)
	// INT32-C
	if (o > 0) && (v > (math.MaxInt32 - o)) {
		panic(&OverflowError{})
	} else if (o < 0) && (v < (math.MinInt32 - o)) {
		panic(&UnderflowError{})
	}
	return v + o
}

func (v Int32Value) Minus(other NumberValue) NumberValue {
	o := other.(Int32Value)
	// INT32-C
	if (o > 0) && (v < (math.MinInt32 + o)) {
		panic(&OverflowError{})
	} else if (o < 0) && (v > (math.MaxInt32 + o)) {
		panic(&UnderflowError{})
	}
	return v - o
}

func (v Int32Value) Mod(other NumberValue) NumberValue {
	o := other.(Int32Value)
	// INT33-C
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v Int32Value) Mul(other NumberValue) NumberValue {
	o := other.(Int32Value)
	// INT32-C
	if v > 0 {
		if o > 0 {
			if v > (math.MaxInt32 / o) {
				panic(&OverflowError{})
			}
		} else {
			if o < (math.MinInt32 / v) {
				panic(&OverflowError{})
			}
		}
	} else {
		if o > 0 {
			if v < (math.MinInt32 / o) {
				panic(&OverflowError{})
			}
		} else {
			if (v != 0) && (o < (math.MaxInt32 / v)) {
				panic(&OverflowError{})
			}
		}
	}
	return v * o
}

func (v Int32Value) Div(other NumberValue) NumberValue {
	o := other.(Int32Value)
	// INT33-C
	// https://golang.org/ref/spec#Integer_operators
	if o == 0 {
		panic(DivisionByZeroError{})
	} else if (v == math.MinInt32) && (o == -1) {
		panic(OverflowError{})
	}
	return v / o
}

func (v Int32Value) Less(other NumberValue) BoolValue {
	return v < other.(Int32Value)
}

func (v Int32Value) LessEqual(other NumberValue) BoolValue {
	return v <= other.(Int32Value)
}

func (v Int32Value) Greater(other NumberValue) BoolValue {
	return v > other.(Int32Value)
}

func (v Int32Value) GreaterEqual(other NumberValue) BoolValue {
	return v >= other.(Int32Value)
}

func (v Int32Value) Equal(other Value) BoolValue {
	otherInt32, ok := other.(Int32Value)
	if !ok {
		return false
	}
	return v == otherInt32
}

func ConvertInt32(value Value, _ *Interpreter) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return Int32Value(value.(NumberValue).IntValue())
}

// Int64Value

type Int64Value int64

func init() {
	gob.Register(Int64Value(0))
}

func (Int64Value) IsValue() {}

func (Int64Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberType{&sema.Int64Type{}}
}

func (v Int64Value) Copy() Value {
	return v
}

func (Int64Value) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (Int64Value) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v Int64Value) String() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int64Value) KeyString() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int64Value) IntValue() int {
	return int(v)
}

func (v Int64Value) Negate() NumberValue {
	// INT32-C
	if v == math.MinInt64 {
		panic(&OverflowError{})
	}
	return -v
}

func safeAddInt64(a, b int64) int64 {
	// INT32-C
	if (b > 0) && (a > (math.MaxInt64 - b)) {
		panic(&OverflowError{})
	} else if (b < 0) && (a < (math.MinInt64 - b)) {
		panic(&UnderflowError{})
	}
	return a + b
}

func (v Int64Value) Plus(other NumberValue) NumberValue {
	o := other.(Int64Value)
	return Int64Value(safeAddInt64(int64(v), int64(o)))
}

func (v Int64Value) Minus(other NumberValue) NumberValue {
	o := other.(Int64Value)
	// INT32-C
	if (o > 0) && (v < (math.MinInt64 + o)) {
		panic(&OverflowError{})
	} else if (o < 0) && (v > (math.MaxInt64 + o)) {
		panic(&UnderflowError{})
	}
	return v - o
}

func (v Int64Value) Mod(other NumberValue) NumberValue {
	o := other.(Int64Value)
	// INT33-C
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v Int64Value) Mul(other NumberValue) NumberValue {
	o := other.(Int64Value)
	// INT32-C
	if v > 0 {
		if o > 0 {
			if v > (math.MaxInt64 / o) {
				panic(&OverflowError{})
			}
		} else {
			if o < (math.MinInt64 / v) {
				panic(&OverflowError{})
			}
		}
	} else {
		if o > 0 {
			if v < (math.MinInt64 / o) {
				panic(&OverflowError{})
			}
		} else {
			if (v != 0) && (o < (math.MaxInt64 / v)) {
				panic(&OverflowError{})
			}
		}
	}
	return v * o
}

func (v Int64Value) Div(other NumberValue) NumberValue {
	o := other.(Int64Value)
	// INT33-C
	// https://golang.org/ref/spec#Integer_operators
	if o == 0 {
		panic(DivisionByZeroError{})
	} else if (v == math.MinInt64) && (o == -1) {
		panic(OverflowError{})
	}
	return v / o
}

func (v Int64Value) Less(other NumberValue) BoolValue {
	return v < other.(Int64Value)
}

func (v Int64Value) LessEqual(other NumberValue) BoolValue {
	return v <= other.(Int64Value)
}

func (v Int64Value) Greater(other NumberValue) BoolValue {
	return v > other.(Int64Value)
}

func (v Int64Value) GreaterEqual(other NumberValue) BoolValue {
	return v >= other.(Int64Value)
}

func (v Int64Value) Equal(other Value) BoolValue {
	otherInt64, ok := other.(Int64Value)
	if !ok {
		return false
	}
	return v == otherInt64
}

func ConvertInt64(value Value, _ *Interpreter) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return Int64Value(value.(NumberValue).IntValue())
}

// Int128Value

type Int128Value struct {
	Int *big.Int
}

func init() {
	gob.Register(Int128Value{})
}

func (v Int128Value) IsValue() {}

func (Int128Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberType{&sema.Int128Type{}}
}

func (v Int128Value) Copy() Value {
	return Int128Value{big.NewInt(0).Set(v.Int)}
}

func (Int128Value) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (Int128Value) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v Int128Value) IntValue() int {
	// TODO: handle overflow
	return int(v.Int.Int64())
}

func (v Int128Value) String() string {
	return v.Int.String()
}

func (v Int128Value) KeyString() string {
	return v.Int.String()
}

func (v Int128Value) Negate() NumberValue {
	// INT32-C
	//   if v == Int128TypeMinInt {
	//       ...
	//   }
	if v.Int.Cmp(sema.Int128TypeMinInt) == 0 {
		panic(&OverflowError{})
	}
	return Int128Value{big.NewInt(0).Neg(v.Int)}
}

func (v Int128Value) Plus(other NumberValue) NumberValue {
	o := other.(Int128Value)
	// Given that this value is backed by an arbitrary size integer,
	// we can just add and check the range of the result.
	//
	// If Go gains a native int128 type and we switch this value
	// to be based on it, then we need to follow INT32-C:
	//
	//   if (o > 0) && (v > (Int128TypeMaxInt - o)) {
	//       ...
	//   } else if (o < 0) && (v < (Int128TypeMinInt - o)) {
	//       ...
	//   }
	//
	res := big.NewInt(0)
	res.Add(v.Int, o.Int)
	if res.Cmp(sema.Int128TypeMinInt) < 0 {
		panic(UnderflowError{})
	} else if res.Cmp(sema.Int128TypeMaxInt) > 0 {
		panic(OverflowError{})
	}
	return Int128Value{res}
}

func (v Int128Value) Minus(other NumberValue) NumberValue {
	o := other.(Int128Value)
	// Given that this value is backed by an arbitrary size integer,
	// we can just subtract and check the range of the result.
	//
	// If Go gains a native int128 type and we switch this value
	// to be based on it, then we need to follow INT32-C:
	//
	//   if (o > 0) && (v < (Int128TypeMinInt + o)) {
	// 	     ...
	//   } else if (o < 0) && (v > (Int128TypeMaxInt + o)) {
	//       ...
	//   }
	//
	res := big.NewInt(0)
	res.Sub(v.Int, o.Int)
	if res.Cmp(sema.Int128TypeMinInt) < 0 {
		panic(UnderflowError{})
	} else if res.Cmp(sema.Int128TypeMaxInt) > 0 {
		panic(OverflowError{})
	}
	return Int128Value{res}
}

func (v Int128Value) Mod(other NumberValue) NumberValue {
	o := other.(Int128Value)
	res := big.NewInt(0)
	// INT33-C
	if o.Int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Mod(v.Int, o.Int)
	return Int128Value{res}
}

func (v Int128Value) Mul(other NumberValue) NumberValue {
	o := other.(Int128Value)
	res := big.NewInt(0)
	res.Mul(v.Int, o.Int)
	if res.Cmp(sema.Int128TypeMinInt) < 0 {
		panic(UnderflowError{})
	} else if res.Cmp(sema.Int128TypeMaxInt) > 0 {
		panic(OverflowError{})
	}
	return Int128Value{res}
}

func (v Int128Value) Div(other NumberValue) NumberValue {
	o := other.(Int128Value)
	res := big.NewInt(0)
	// INT33-C:
	//   if o == 0 {
	//       ...
	//   } else if (v == Int128TypeMinInt) && (o == -1) {
	//       ...
	//   }
	if o.Int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.SetInt64(-1)
	if (v.Int.Cmp(sema.Int128TypeMinInt) == 0) && (o.Int.Cmp(res) == 0) {
		panic(OverflowError{})
	}
	res.Div(v.Int, o.Int)
	return Int128Value{res}
}

func (v Int128Value) Less(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(Int128Value).Int)
	return cmp == -1
}

func (v Int128Value) LessEqual(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(Int128Value).Int)
	return cmp <= 0
}

func (v Int128Value) Greater(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(Int128Value).Int)
	return cmp == 1
}

func (v Int128Value) GreaterEqual(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(Int128Value).Int)
	return cmp >= 0
}

func (v Int128Value) Equal(other Value) BoolValue {
	otherInt, ok := other.(Int128Value)
	if !ok {
		return false
	}
	cmp := v.Int.Cmp(otherInt.Int)
	return cmp == 0
}

func ConvertInt128(value Value, _ *Interpreter) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	intValue := value.(NumberValue).IntValue()
	return Int128Value{big.NewInt(0).SetInt64(int64(intValue))}
}

// Int256Value

type Int256Value struct {
	Int *big.Int
}

func init() {
	gob.Register(Int256Value{})
}

func (v Int256Value) IsValue() {}

func (Int256Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberType{&sema.Int256Type{}}
}

func (v Int256Value) Copy() Value {
	return Int256Value{big.NewInt(0).Set(v.Int)}
}

func (Int256Value) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (Int256Value) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v Int256Value) IntValue() int {
	// TODO: handle overflow
	return int(v.Int.Int64())
}

func (v Int256Value) String() string {
	return v.Int.String()
}

func (v Int256Value) KeyString() string {
	return v.Int.String()
}

func (v Int256Value) Negate() NumberValue {
	// INT32-C
	//   if v == Int256TypeMinInt {
	//       ...
	//   }
	if v.Int.Cmp(sema.Int256TypeMinInt) == 0 {
		panic(&OverflowError{})
	}
	return Int256Value{big.NewInt(0).Neg(v.Int)}
}

func (v Int256Value) Plus(other NumberValue) NumberValue {
	o := other.(Int256Value)
	// Given that this value is backed by an arbitrary size integer,
	// we can just add and check the range of the result.
	//
	// If Go gains a native int256 type and we switch this value
	// to be based on it, then we need to follow INT32-C:
	//
	//   if (o > 0) && (v > (Int256TypeMaxInt - o)) {
	//       ...
	//   } else if (o < 0) && (v < (Int256TypeMinInt - o)) {
	//       ...
	//   }
	//
	res := big.NewInt(0)
	res.Add(v.Int, o.Int)
	if res.Cmp(sema.Int256TypeMinInt) < 0 {
		panic(UnderflowError{})
	} else if res.Cmp(sema.Int256TypeMaxInt) > 0 {
		panic(OverflowError{})
	}
	return Int256Value{res}
}

func (v Int256Value) Minus(other NumberValue) NumberValue {
	o := other.(Int256Value)
	// Given that this value is backed by an arbitrary size integer,
	// we can just subtract and check the range of the result.
	//
	// If Go gains a native int256 type and we switch this value
	// to be based on it, then we need to follow INT32-C:
	//
	//   if (o > 0) && (v < (Int256TypeMinInt + o)) {
	// 	     ...
	//   } else if (o < 0) && (v > (Int256TypeMaxInt + o)) {
	//       ...
	//   }
	//
	res := big.NewInt(0)
	res.Sub(v.Int, o.Int)
	if res.Cmp(sema.Int256TypeMinInt) < 0 {
		panic(UnderflowError{})
	} else if res.Cmp(sema.Int256TypeMaxInt) > 0 {
		panic(OverflowError{})
	}
	return Int256Value{res}
}

func (v Int256Value) Mod(other NumberValue) NumberValue {
	o := other.(Int256Value)
	res := big.NewInt(0)
	// INT33-C
	if o.Int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Mod(v.Int, o.Int)
	return Int256Value{res}
}

func (v Int256Value) Mul(other NumberValue) NumberValue {
	o := other.(Int256Value)
	res := big.NewInt(0)
	res.Mul(v.Int, o.Int)
	if res.Cmp(sema.Int256TypeMinInt) < 0 {
		panic(UnderflowError{})
	} else if res.Cmp(sema.Int256TypeMaxInt) > 0 {
		panic(OverflowError{})
	}
	return Int256Value{res}
}

func (v Int256Value) Div(other NumberValue) NumberValue {
	o := other.(Int256Value)
	res := big.NewInt(0)
	// INT33-C:
	//   if o == 0 {
	//       ...
	//   } else if (v == Int256TypeMinInt) && (o == -1) {
	//       ...
	//   }
	if o.Int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.SetInt64(-1)
	if (v.Int.Cmp(sema.Int256TypeMinInt) == 0) && (o.Int.Cmp(res) == 0) {
		panic(OverflowError{})
	}
	res.Div(v.Int, o.Int)
	return Int256Value{res}
}

func (v Int256Value) Less(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(Int256Value).Int)
	return cmp == -1
}

func (v Int256Value) LessEqual(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(Int256Value).Int)
	return cmp <= 0
}

func (v Int256Value) Greater(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(Int256Value).Int)
	return cmp == 1
}

func (v Int256Value) GreaterEqual(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(Int256Value).Int)
	return cmp >= 0
}

func (v Int256Value) Equal(other Value) BoolValue {
	otherInt, ok := other.(Int256Value)
	if !ok {
		return false
	}
	cmp := v.Int.Cmp(otherInt.Int)
	return cmp == 0
}

func ConvertInt256(value Value, _ *Interpreter) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	intValue := value.(NumberValue).IntValue()
	return Int256Value{big.NewInt(0).SetInt64(int64(intValue))}
}

// UIntValue

type UIntValue struct {
	Int *big.Int
}

func init() {
	gob.Register(UIntValue{})
}

func NewUIntValue(value uint64) UIntValue {
	return UIntValue{Int: big.NewInt(0).SetUint64(value)}
}

func ConvertUInt(value Value, _ *Interpreter) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	if intValue, ok := value.(UIntValue); ok {
		return intValue.Copy()
	}
	return NewUIntValue(uint64(value.(NumberValue).IntValue()))
}

func (v UIntValue) IsValue() {}

func (UIntValue) DynamicType(_ *Interpreter) DynamicType {
	return NumberType{&sema.UIntType{}}
}

func (v UIntValue) Copy() Value {
	return UIntValue{big.NewInt(0).Set(v.Int)}
}

func (UIntValue) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (UIntValue) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v UIntValue) IntValue() int {
	// TODO: handle overflow
	return int(v.Int.Int64())
}

func (v UIntValue) String() string {
	return v.Int.String()
}

func (v UIntValue) KeyString() string {
	return v.Int.String()
}

func (v UIntValue) Negate() NumberValue {
	panic(errors.NewUnreachableError())
}

func (v UIntValue) Plus(other NumberValue) NumberValue {
	o := other.(UIntValue)
	res := big.NewInt(0)
	res.Add(v.Int, o.Int)
	return UIntValue{res}
}

func (v UIntValue) Minus(other NumberValue) NumberValue {
	o := other.(UIntValue)
	res := big.NewInt(0)
	res.Sub(v.Int, o.Int)
	if res.Sign() < 0 {
		panic(&UnderflowError{})
	}
	return UIntValue{res}
}

func (v UIntValue) Mod(other NumberValue) NumberValue {
	o := other.(UIntValue)
	res := big.NewInt(0)
	// INT33-C
	if o.Int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Mod(v.Int, o.Int)
	return UIntValue{res}
}

func (v UIntValue) Mul(other NumberValue) NumberValue {
	o := other.(UIntValue)
	res := big.NewInt(0)
	res.Mul(v.Int, o.Int)
	return UIntValue{res}
}

func (v UIntValue) Div(other NumberValue) NumberValue {
	o := other.(UIntValue)
	res := big.NewInt(0)
	// INT33-C
	if o.Int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Div(v.Int, o.Int)
	return UIntValue{res}
}

func (v UIntValue) Less(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(UIntValue).Int)
	return cmp == -1
}

func (v UIntValue) LessEqual(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(UIntValue).Int)
	return cmp <= 0
}

func (v UIntValue) Greater(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(UIntValue).Int)
	return cmp == 1
}

func (v UIntValue) GreaterEqual(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(UIntValue).Int)
	return cmp >= 0
}

func (v UIntValue) Equal(other Value) BoolValue {
	otherUInt, ok := other.(UIntValue)
	if !ok {
		return false
	}
	cmp := v.Int.Cmp(otherUInt.Int)
	return cmp == 0
}

// UInt8Value

type UInt8Value uint8

func init() {
	gob.Register(UInt8Value(0))
}

func (UInt8Value) IsValue() {}

func (UInt8Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberType{&sema.UInt8Type{}}
}

func (v UInt8Value) Copy() Value {
	return v
}

func (UInt8Value) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (UInt8Value) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v UInt8Value) String() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt8Value) KeyString() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt8Value) IntValue() int {
	return int(v)
}

func (v UInt8Value) Negate() NumberValue {
	panic(errors.NewUnreachableError())
}

func (v UInt8Value) Plus(other NumberValue) NumberValue {
	sum := v + other.(UInt8Value)
	// INT30-C
	if sum < v {
		panic(OverflowError{})
	}
	return sum
}

func (v UInt8Value) Minus(other NumberValue) NumberValue {
	diff := v - other.(UInt8Value)
	// INT30-C
	if diff > v {
		panic(UnderflowError{})
	}
	return diff
}

func (v UInt8Value) Mod(other NumberValue) NumberValue {
	o := other.(UInt8Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v UInt8Value) Mul(other NumberValue) NumberValue {
	o := other.(UInt8Value)
	if (v > 0) && (o > 0) && (v > (math.MaxUint8 / o)) {
		panic(&OverflowError{})
	}
	return v * o
}

func (v UInt8Value) Div(other NumberValue) NumberValue {
	o := other.(UInt8Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v / o
}

func (v UInt8Value) Less(other NumberValue) BoolValue {
	return v < other.(UInt8Value)
}

func (v UInt8Value) LessEqual(other NumberValue) BoolValue {
	return v <= other.(UInt8Value)
}

func (v UInt8Value) Greater(other NumberValue) BoolValue {
	return v > other.(UInt8Value)
}

func (v UInt8Value) GreaterEqual(other NumberValue) BoolValue {
	return v >= other.(UInt8Value)
}

func (v UInt8Value) Equal(other Value) BoolValue {
	otherUInt8, ok := other.(UInt8Value)
	if !ok {
		return false
	}
	return v == otherUInt8
}

func ConvertUInt8(value Value, _ *Interpreter) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return UInt8Value(value.(NumberValue).IntValue())
}

// UInt16Value

type UInt16Value uint16

func init() {
	gob.Register(UInt16Value(0))
}

func (UInt16Value) IsValue() {}

func (UInt16Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberType{&sema.UInt16Type{}}
}

func (v UInt16Value) Copy() Value {
	return v
}
func (UInt16Value) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (UInt16Value) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v UInt16Value) String() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt16Value) KeyString() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt16Value) IntValue() int {
	return int(v)
}
func (v UInt16Value) Negate() NumberValue {
	panic(errors.NewUnreachableError())
}

func (v UInt16Value) Plus(other NumberValue) NumberValue {
	sum := v + other.(UInt16Value)
	// INT30-C
	if sum < v {
		panic(OverflowError{})
	}
	return sum
}

func (v UInt16Value) Minus(other NumberValue) NumberValue {
	diff := v - other.(UInt16Value)
	// INT30-C
	if diff > v {
		panic(UnderflowError{})
	}
	return diff
}

func (v UInt16Value) Mod(other NumberValue) NumberValue {
	o := other.(UInt16Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v UInt16Value) Mul(other NumberValue) NumberValue {
	o := other.(UInt16Value)
	if (v > 0) && (o > 0) && (v > (math.MaxUint16 / o)) {
		panic(&OverflowError{})
	}
	return v * o
}

func (v UInt16Value) Div(other NumberValue) NumberValue {
	o := other.(UInt16Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v / o
}

func (v UInt16Value) Less(other NumberValue) BoolValue {
	return v < other.(UInt16Value)
}

func (v UInt16Value) LessEqual(other NumberValue) BoolValue {
	return v <= other.(UInt16Value)
}

func (v UInt16Value) Greater(other NumberValue) BoolValue {
	return v > other.(UInt16Value)
}

func (v UInt16Value) GreaterEqual(other NumberValue) BoolValue {
	return v >= other.(UInt16Value)
}

func (v UInt16Value) Equal(other Value) BoolValue {
	otherUInt16, ok := other.(UInt16Value)
	if !ok {
		return false
	}
	return v == otherUInt16
}

func ConvertUInt16(value Value, _ *Interpreter) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return UInt16Value(value.(NumberValue).IntValue())
}

// UInt32Value

type UInt32Value uint32

func init() {
	gob.Register(UInt32Value(0))
}

func (UInt32Value) IsValue() {}

func (UInt32Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberType{&sema.UInt32Type{}}
}

func (v UInt32Value) Copy() Value {
	return v
}

func (UInt32Value) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (UInt32Value) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v UInt32Value) String() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt32Value) KeyString() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt32Value) IntValue() int {
	return int(v)
}

func (v UInt32Value) Negate() NumberValue {
	panic(errors.NewUnreachableError())
}

func (v UInt32Value) Plus(other NumberValue) NumberValue {
	sum := v + other.(UInt32Value)
	// INT30-C
	if sum < v {
		panic(OverflowError{})
	}
	return sum
}

func (v UInt32Value) Minus(other NumberValue) NumberValue {
	diff := v - other.(UInt32Value)
	// INT30-C
	if diff > v {
		panic(UnderflowError{})
	}
	return diff
}

func (v UInt32Value) Mod(other NumberValue) NumberValue {
	o := other.(UInt32Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v UInt32Value) Mul(other NumberValue) NumberValue {
	o := other.(UInt32Value)
	if (v > 0) && (o > 0) && (v > (math.MaxUint32 / o)) {
		panic(&OverflowError{})
	}
	return v * o
}

func (v UInt32Value) Div(other NumberValue) NumberValue {
	o := other.(UInt32Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v / o
}

func (v UInt32Value) Less(other NumberValue) BoolValue {
	return v < other.(UInt32Value)
}

func (v UInt32Value) LessEqual(other NumberValue) BoolValue {
	return v <= other.(UInt32Value)
}

func (v UInt32Value) Greater(other NumberValue) BoolValue {
	return v > other.(UInt32Value)
}

func (v UInt32Value) GreaterEqual(other NumberValue) BoolValue {
	return v >= other.(UInt32Value)
}

func (v UInt32Value) Equal(other Value) BoolValue {
	otherUInt32, ok := other.(UInt32Value)
	if !ok {
		return false
	}
	return v == otherUInt32
}

func ConvertUInt32(value Value, _ *Interpreter) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return UInt32Value(value.(NumberValue).IntValue())
}

// UInt64Value

type UInt64Value uint64

func init() {
	gob.Register(UInt64Value(0))
}

func (UInt64Value) IsValue() {}

func (UInt64Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberType{&sema.UInt64Type{}}
}

func (v UInt64Value) Copy() Value {
	return v
}

func (UInt64Value) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (UInt64Value) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v UInt64Value) String() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt64Value) KeyString() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt64Value) IntValue() int {
	return int(v)
}

func (v UInt64Value) Negate() NumberValue {
	panic(errors.NewUnreachableError())
}

func safeAddUint64(a, b uint64) uint64 {
	sum := a + b
	// INT30-C
	if sum < a {
		panic(OverflowError{})
	}
	return sum
}

func (v UInt64Value) Plus(other NumberValue) NumberValue {
	o := other.(UInt64Value)
	return UInt64Value(safeAddUint64(uint64(v), uint64(o)))
}

func (v UInt64Value) Minus(other NumberValue) NumberValue {
	diff := v - other.(UInt64Value)
	// INT30-C
	if diff > v {
		panic(UnderflowError{})
	}
	return diff
}

func (v UInt64Value) Mod(other NumberValue) NumberValue {
	o := other.(UInt64Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v UInt64Value) Mul(other NumberValue) NumberValue {
	o := other.(UInt64Value)
	if (v > 0) && (o > 0) && (v > (math.MaxUint64 / o)) {
		panic(&OverflowError{})
	}
	return v * o
}

func (v UInt64Value) Div(other NumberValue) NumberValue {
	o := other.(UInt64Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v / o
}

func (v UInt64Value) Less(other NumberValue) BoolValue {
	return v < other.(UInt64Value)
}

func (v UInt64Value) LessEqual(other NumberValue) BoolValue {
	return v <= other.(UInt64Value)
}

func (v UInt64Value) Greater(other NumberValue) BoolValue {
	return v > other.(UInt64Value)
}

func (v UInt64Value) GreaterEqual(other NumberValue) BoolValue {
	return v >= other.(UInt64Value)
}

func (v UInt64Value) Equal(other Value) BoolValue {
	otherUInt64, ok := other.(UInt64Value)
	if !ok {
		return false
	}
	return v == otherUInt64
}

func ConvertUInt64(value Value, _ *Interpreter) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return UInt64Value(value.(NumberValue).IntValue())
}

// UInt128Value

type UInt128Value struct {
	Int *big.Int
}

func init() {
	gob.Register(UInt128Value{})
}

func (v UInt128Value) IsValue() {}

func (UInt128Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberType{&sema.UInt128Type{}}
}

func (v UInt128Value) Copy() Value {
	return UInt128Value{big.NewInt(0).Set(v.Int)}
}

func (UInt128Value) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (UInt128Value) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v UInt128Value) IntValue() int {
	// TODO: handle overflow
	return int(v.Int.Int64())
}

func (v UInt128Value) String() string {
	return v.Int.String()
}

func (v UInt128Value) KeyString() string {
	return v.Int.String()
}

func (v UInt128Value) Negate() NumberValue {
	panic(errors.NewUnreachableError())
}

func (v UInt128Value) Plus(other NumberValue) NumberValue {
	sum := big.NewInt(0)
	sum.Add(v.Int, other.(UInt128Value).Int)
	// Given that this value is backed by an arbitrary size integer,
	// we can just add and check the range of the result.
	//
	// If Go gains a native uint128 type and we switch this value
	// to be based on it, then we need to follow INT30-C:
	//
	//  if sum < v {
	//      ...
	//  }
	//
	if sum.Cmp(sema.UInt128TypeMaxInt) > 0 {
		panic(OverflowError{})
	}
	return UInt128Value{sum}
}

func (v UInt128Value) Minus(other NumberValue) NumberValue {
	diff := big.NewInt(0)
	diff.Sub(v.Int, other.(UInt128Value).Int)
	// Given that this value is backed by an arbitrary size integer,
	// we can just subtract and check the range of the result.
	//
	// If Go gains a native uint128 type and we switch this value
	// to be based on it, then we need to follow INT30-C:
	//
	//   if diff > v {
	// 	     ...
	//   }
	//
	if diff.Cmp(sema.UInt128TypeMinInt) < 0 {
		panic(UnderflowError{})
	}
	return UInt128Value{diff}
}

func (v UInt128Value) Mod(other NumberValue) NumberValue {
	o := other.(UInt128Value)
	res := big.NewInt(0)
	if o.Int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Mod(v.Int, o.Int)
	return UInt128Value{res}
}

func (v UInt128Value) Mul(other NumberValue) NumberValue {
	o := other.(UInt128Value)
	res := big.NewInt(0)
	res.Mul(v.Int, o.Int)
	if res.Cmp(sema.UInt128TypeMaxInt) > 0 {
		panic(OverflowError{})
	}
	return UInt128Value{res}
}

func (v UInt128Value) Div(other NumberValue) NumberValue {
	o := other.(UInt128Value)
	res := big.NewInt(0)
	if o.Int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Div(v.Int, o.Int)
	return UInt128Value{res}
}

func (v UInt128Value) Less(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(UInt128Value).Int)
	return cmp == -1
}

func (v UInt128Value) LessEqual(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(UInt128Value).Int)
	return cmp <= 0
}

func (v UInt128Value) Greater(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(UInt128Value).Int)
	return cmp == 1
}

func (v UInt128Value) GreaterEqual(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(UInt128Value).Int)
	return cmp >= 0
}

func (v UInt128Value) Equal(other Value) BoolValue {
	otherInt, ok := other.(UInt128Value)
	if !ok {
		return false
	}
	cmp := v.Int.Cmp(otherInt.Int)
	return cmp == 0
}

func ConvertUInt128(value Value, _ *Interpreter) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	intValue := value.(NumberValue).IntValue()
	return UInt128Value{big.NewInt(0).SetInt64(int64(intValue))}
}

// UInt256Value

type UInt256Value struct {
	Int *big.Int
}

func init() {
	gob.Register(UInt256Value{})
}

func (v UInt256Value) IsValue() {}

func (UInt256Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberType{&sema.UInt256Type{}}
}

func (v UInt256Value) Copy() Value {
	return UInt256Value{big.NewInt(0).Set(v.Int)}
}

func (UInt256Value) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (UInt256Value) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v UInt256Value) IntValue() int {
	// TODO: handle overflow
	return int(v.Int.Int64())
}

func (v UInt256Value) String() string {
	return v.Int.String()
}

func (v UInt256Value) KeyString() string {
	return v.Int.String()
}

func (v UInt256Value) Negate() NumberValue {
	panic(errors.NewUnreachableError())
}

func (v UInt256Value) Plus(other NumberValue) NumberValue {
	sum := big.NewInt(0)
	sum.Add(v.Int, other.(UInt256Value).Int)
	// Given that this value is backed by an arbitrary size integer,
	// we can just add and check the range of the result.
	//
	// If Go gains a native uint256 type and we switch this value
	// to be based on it, then we need to follow INT30-C:
	//
	//  if sum < v {
	//      ...
	//  }
	//
	if sum.Cmp(sema.UInt256TypeMaxInt) > 0 {
		panic(OverflowError{})
	}
	return UInt256Value{sum}
}

func (v UInt256Value) Minus(other NumberValue) NumberValue {
	diff := big.NewInt(0)
	diff.Sub(v.Int, other.(UInt256Value).Int)
	// Given that this value is backed by an arbitrary size integer,
	// we can just subtract and check the range of the result.
	//
	// If Go gains a native uint256 type and we switch this value
	// to be based on it, then we need to follow INT30-C:
	//
	//   if diff > v {
	// 	     ...
	//   }
	//
	if diff.Cmp(sema.UInt256TypeMinInt) < 0 {
		panic(UnderflowError{})
	}
	return UInt256Value{diff}
}

func (v UInt256Value) Mod(other NumberValue) NumberValue {
	o := other.(UInt256Value)
	res := big.NewInt(0)
	if o.Int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Mod(v.Int, o.Int)
	return UInt256Value{res}
}

func (v UInt256Value) Mul(other NumberValue) NumberValue {
	o := other.(UInt256Value)
	res := big.NewInt(0)
	res.Mul(v.Int, o.Int)
	if res.Cmp(sema.UInt256TypeMaxInt) > 0 {
		panic(OverflowError{})
	}
	return UInt256Value{res}
}

func (v UInt256Value) Div(other NumberValue) NumberValue {
	o := other.(UInt256Value)
	res := big.NewInt(0)
	if o.Int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Div(v.Int, o.Int)
	return UInt256Value{res}
}

func (v UInt256Value) Less(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(UInt256Value).Int)
	return cmp == -1
}

func (v UInt256Value) LessEqual(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(UInt256Value).Int)
	return cmp <= 0
}

func (v UInt256Value) Greater(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(UInt256Value).Int)
	return cmp == 1
}

func (v UInt256Value) GreaterEqual(other NumberValue) BoolValue {
	cmp := v.Int.Cmp(other.(UInt256Value).Int)
	return cmp >= 0
}

func (v UInt256Value) Equal(other Value) BoolValue {
	otherInt, ok := other.(UInt256Value)
	if !ok {
		return false
	}
	cmp := v.Int.Cmp(otherInt.Int)
	return cmp == 0
}

func ConvertUInt256(value Value, _ *Interpreter) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	intValue := value.(NumberValue).IntValue()
	return UInt256Value{big.NewInt(0).SetInt64(int64(intValue))}
}

// Word8Value

type Word8Value uint8

func init() {
	gob.Register(Word8Value(0))
}

func (Word8Value) IsValue() {}

func (Word8Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberType{&sema.Word8Type{}}
}

func (v Word8Value) Copy() Value {
	return v
}

func (Word8Value) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (Word8Value) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v Word8Value) String() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v Word8Value) KeyString() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v Word8Value) IntValue() int {
	return int(v)
}

func (v Word8Value) Negate() NumberValue {
	panic(errors.NewUnreachableError())
}

func (v Word8Value) Plus(other NumberValue) NumberValue {
	return v + other.(Word8Value)
}

func (v Word8Value) Minus(other NumberValue) NumberValue {
	return v - other.(Word8Value)
}

func (v Word8Value) Mod(other NumberValue) NumberValue {
	o := other.(Word8Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v Word8Value) Mul(other NumberValue) NumberValue {
	return v * other.(Word8Value)
}

func (v Word8Value) Div(other NumberValue) NumberValue {
	o := other.(Word8Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v / o
}

func (v Word8Value) Less(other NumberValue) BoolValue {
	return v < other.(Word8Value)
}

func (v Word8Value) LessEqual(other NumberValue) BoolValue {
	return v <= other.(Word8Value)
}

func (v Word8Value) Greater(other NumberValue) BoolValue {
	return v > other.(Word8Value)
}

func (v Word8Value) GreaterEqual(other NumberValue) BoolValue {
	return v >= other.(Word8Value)
}

func (v Word8Value) Equal(other Value) BoolValue {
	otherWord8, ok := other.(Word8Value)
	if !ok {
		return false
	}
	return v == otherWord8
}

func ConvertWord8(value Value, _ *Interpreter) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return Word8Value(value.(NumberValue).IntValue())
}

// Word16Value

type Word16Value uint16

func init() {
	gob.Register(Word16Value(0))
}

func (Word16Value) IsValue() {}

func (Word16Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberType{&sema.Word16Type{}}
}

func (v Word16Value) Copy() Value {
	return v
}
func (Word16Value) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (Word16Value) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v Word16Value) String() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v Word16Value) KeyString() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v Word16Value) IntValue() int {
	return int(v)
}
func (v Word16Value) Negate() NumberValue {
	panic(errors.NewUnreachableError())
}

func (v Word16Value) Plus(other NumberValue) NumberValue {
	return v + other.(Word16Value)
}

func (v Word16Value) Minus(other NumberValue) NumberValue {
	return v - other.(Word16Value)
}

func (v Word16Value) Mod(other NumberValue) NumberValue {
	o := other.(Word16Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v Word16Value) Mul(other NumberValue) NumberValue {
	return v * other.(Word16Value)
}

func (v Word16Value) Div(other NumberValue) NumberValue {
	o := other.(Word16Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v / o
}

func (v Word16Value) Less(other NumberValue) BoolValue {
	return v < other.(Word16Value)
}

func (v Word16Value) LessEqual(other NumberValue) BoolValue {
	return v <= other.(Word16Value)
}

func (v Word16Value) Greater(other NumberValue) BoolValue {
	return v > other.(Word16Value)
}

func (v Word16Value) GreaterEqual(other NumberValue) BoolValue {
	return v >= other.(Word16Value)
}

func (v Word16Value) Equal(other Value) BoolValue {
	otherWord16, ok := other.(Word16Value)
	if !ok {
		return false
	}
	return v == otherWord16
}

func ConvertWord16(value Value, _ *Interpreter) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return Word16Value(value.(NumberValue).IntValue())
}

// Word32Value

type Word32Value uint32

func init() {
	gob.Register(Word32Value(0))
}

func (Word32Value) IsValue() {}

func (Word32Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberType{&sema.Word32Type{}}
}

func (v Word32Value) Copy() Value {
	return v
}

func (Word32Value) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (Word32Value) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v Word32Value) String() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v Word32Value) KeyString() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v Word32Value) IntValue() int {
	return int(v)
}

func (v Word32Value) Negate() NumberValue {
	panic(errors.NewUnreachableError())
}

func (v Word32Value) Plus(other NumberValue) NumberValue {
	return v + other.(Word32Value)
}

func (v Word32Value) Minus(other NumberValue) NumberValue {
	return v - other.(Word32Value)
}

func (v Word32Value) Mod(other NumberValue) NumberValue {
	o := other.(Word32Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v Word32Value) Mul(other NumberValue) NumberValue {
	return v * other.(Word32Value)
}

func (v Word32Value) Div(other NumberValue) NumberValue {
	o := other.(Word32Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v / o
}

func (v Word32Value) Less(other NumberValue) BoolValue {
	return v < other.(Word32Value)
}

func (v Word32Value) LessEqual(other NumberValue) BoolValue {
	return v <= other.(Word32Value)
}

func (v Word32Value) Greater(other NumberValue) BoolValue {
	return v > other.(Word32Value)
}

func (v Word32Value) GreaterEqual(other NumberValue) BoolValue {
	return v >= other.(Word32Value)
}

func (v Word32Value) Equal(other Value) BoolValue {
	otherWord32, ok := other.(Word32Value)
	if !ok {
		return false
	}
	return v == otherWord32
}

func ConvertWord32(value Value, _ *Interpreter) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return Word32Value(value.(NumberValue).IntValue())
}

// Word64Value

type Word64Value uint64

func init() {
	gob.Register(Word64Value(0))
}

func (Word64Value) IsValue() {}

func (Word64Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberType{&sema.Word64Type{}}
}

func (v Word64Value) Copy() Value {
	return v
}

func (Word64Value) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (Word64Value) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v Word64Value) String() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v Word64Value) KeyString() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v Word64Value) IntValue() int {
	return int(v)
}

func (v Word64Value) Negate() NumberValue {
	panic(errors.NewUnreachableError())
}

func (v Word64Value) Plus(other NumberValue) NumberValue {
	return v + other.(Word64Value)
}

func (v Word64Value) Minus(other NumberValue) NumberValue {
	return v - other.(Word64Value)
}

func (v Word64Value) Mod(other NumberValue) NumberValue {
	o := other.(Word64Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v Word64Value) Mul(other NumberValue) NumberValue {
	return v * other.(Word64Value)
}

func (v Word64Value) Div(other NumberValue) NumberValue {
	o := other.(Word64Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v / o
}

func (v Word64Value) Less(other NumberValue) BoolValue {
	return v < other.(Word64Value)
}

func (v Word64Value) LessEqual(other NumberValue) BoolValue {
	return v <= other.(Word64Value)
}

func (v Word64Value) Greater(other NumberValue) BoolValue {
	return v > other.(Word64Value)
}

func (v Word64Value) GreaterEqual(other NumberValue) BoolValue {
	return v >= other.(Word64Value)
}

func (v Word64Value) Equal(other Value) BoolValue {
	otherWord64, ok := other.(Word64Value)
	if !ok {
		return false
	}
	return v == otherWord64
}

func ConvertWord64(value Value, _ *Interpreter) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return Word64Value(value.(NumberValue).IntValue())
}

// Fix64Value

type Fix64Value int64

func init() {
	gob.Register(Fix64Value(0))
}

func (Fix64Value) IsValue() {}

func (Fix64Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberType{&sema.Fix64Type{}}
}

func (v Fix64Value) Copy() Value {
	return v
}

func (Fix64Value) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (Fix64Value) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v Fix64Value) String() string {
	integer := int64(v) / sema.Fix64Factor
	fraction := int64(v) % sema.Fix64Factor
	negative := fraction < 0
	var builder strings.Builder
	if negative {
		fraction = -fraction
		if integer == 0 {
			builder.WriteRune('-')
		}
	}
	builder.WriteString(fmt.Sprint(integer))
	builder.WriteRune('.')
	builder.WriteString(PadLeft(strconv.Itoa(int(fraction)), '0', sema.Fix64Scale))
	return builder.String()
}

func (v Fix64Value) KeyString() string {
	return v.String()
}

func (v Fix64Value) IntValue() int {
	return int(v)
}

func (v Fix64Value) Negate() NumberValue {
	// INT32-C
	if v == math.MinInt64 {
		panic(&OverflowError{})
	}
	return -v
}

func (v Fix64Value) Plus(other NumberValue) NumberValue {
	o := other.(Fix64Value)
	return Fix64Value(safeAddInt64(int64(v), int64(o)))
}

func (v Fix64Value) Minus(other NumberValue) NumberValue {
	o := other.(Fix64Value)
	// INT32-C
	if (o > 0) && (v < (math.MinInt64 + o)) {
		panic(&OverflowError{})
	} else if (o < 0) && (v > (math.MaxInt64 + o)) {
		panic(&UnderflowError{})
	}
	return v - o
}

var Fix64MulPrecision = int64(math.Sqrt(float64(sema.Fix64Factor)))

func (v Fix64Value) Mul(other NumberValue) NumberValue {
	o := other.(Fix64Value)

	x1 := int64(v) / sema.Fix64Factor
	x2 := int64(v) % sema.Fix64Factor

	y1 := int64(o) / sema.Fix64Factor
	y2 := int64(o) % sema.Fix64Factor

	x1y1 := x1 * y1
	if x1 != 0 && x1y1/x1 != y1 {
		panic(&OverflowError{})
	}

	x1y1Fixed := x1y1 * sema.Fix64Factor
	if x1y1 != 0 && x1y1Fixed/x1y1 != sema.Fix64Factor {
		panic(&OverflowError{})
	}
	x1y1 = x1y1Fixed

	x2y1 := x2 * y1
	if x2 != 0 && x2y1/x2 != y1 {
		panic(&OverflowError{})
	}

	x1y2 := x1 * y2
	if x1 != 0 && x1y2/x1 != y2 {
		panic(&OverflowError{})
	}

	x2 = x2 / Fix64MulPrecision
	y2 = y2 / Fix64MulPrecision
	x2y2 := x2 * y2
	if x2 != 0 && x2y2/x2 != y2 {
		panic(&OverflowError{})
	}

	result := x1y1
	result = safeAddInt64(result, x2y1)
	result = safeAddInt64(result, x1y2)
	result = safeAddInt64(result, x2y2)
	return Fix64Value(result)
}

func (v Fix64Value) Div(other NumberValue) NumberValue {
	// TODO:
	panic("TODO")
}

func (v Fix64Value) Mod(other NumberValue) NumberValue {
	// TODO:
	panic("TODO")
}

func (v Fix64Value) Less(other NumberValue) BoolValue {
	return v < other.(Fix64Value)
}

func (v Fix64Value) LessEqual(other NumberValue) BoolValue {
	return v <= other.(Fix64Value)
}

func (v Fix64Value) Greater(other NumberValue) BoolValue {
	return v > other.(Fix64Value)
}

func (v Fix64Value) GreaterEqual(other NumberValue) BoolValue {
	return v >= other.(Fix64Value)
}

func (v Fix64Value) Equal(other Value) BoolValue {
	otherFix64, ok := other.(Fix64Value)
	if !ok {
		return false
	}
	return v == otherFix64
}

const Fix64MaxValue = math.MaxInt64

func ConvertFix64(value Value, interpreter *Interpreter) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141

	switch value := value.(type) {
	case UFix64Value:
		if int(value) > Fix64MaxValue {
			panic("UFix64 value is larger than maximum value for Fix64")
		}
		return Fix64Value(value)

	case Fix64Value:
		return value

	case NumberValue:
		return Fix64Value(value.IntValue() * sema.Fix64Factor)

	default:
		panic(fmt.Sprintf("can't convert %s to Fix64", value.DynamicType(interpreter)))
	}
}

// UFix64Value

type UFix64Value uint64

func init() {
	gob.Register(UFix64Value(0))
}

func NewUFix64ValueWithFraction(integer, fraction uint64) UFix64Value {
	return UFix64Value(integer*sema.Fix64Factor + fraction)
}

func (UFix64Value) IsValue() {}

func (UFix64Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberType{&sema.UFix64Type{}}
}

func (v UFix64Value) Copy() Value {
	return v
}

func (UFix64Value) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (UFix64Value) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v UFix64Value) String() string {
	factor := uint64(sema.Fix64Factor)
	integer := uint64(v) / factor
	fraction := uint64(v) % factor
	return fmt.Sprintf(
		"%d.%s",
		integer,
		PadLeft(strconv.Itoa(int(fraction)), '0', sema.Fix64Scale),
	)
}

func (v UFix64Value) KeyString() string {
	return v.String()
}

func (v UFix64Value) IntValue() int {
	return int(v)
}

func (v UFix64Value) Negate() NumberValue {
	panic(errors.NewUnreachableError())
}

func (v UFix64Value) Plus(other NumberValue) NumberValue {
	o := other.(UFix64Value)
	return UFix64Value(safeAddUint64(uint64(v), uint64(o)))
}

func (v UFix64Value) Minus(other NumberValue) NumberValue {
	diff := v - other.(UFix64Value)
	// INT30-C
	if diff > v {
		panic(UnderflowError{})
	}
	return diff
}

var UFix64MulPrecision = uint64(math.Sqrt(float64(sema.Fix64Factor)))

func (v UFix64Value) Mul(other NumberValue) NumberValue {
	o := other.(UFix64Value)

	factor := uint64(sema.Fix64Factor)

	x1 := uint64(v) / factor
	x2 := uint64(v) % factor

	y1 := uint64(o) / factor
	y2 := uint64(o) % factor

	x1y1 := x1 * y1
	if x1 != 0 && x1y1/x1 != y1 {
		panic(&OverflowError{})
	}

	x1y1Fixed := x1y1 * factor
	if x1y1 != 0 && x1y1Fixed/x1y1 != factor {
		panic(&OverflowError{})
	}
	x1y1 = x1y1Fixed

	x2y1 := x2 * y1
	if x2 != 0 && x2y1/x2 != y1 {
		panic(&OverflowError{})
	}

	x1y2 := x1 * y2
	if x1 != 0 && x1y2/x1 != y2 {
		panic(&OverflowError{})
	}

	x2 = x2 / UFix64MulPrecision
	y2 = y2 / UFix64MulPrecision
	x2y2 := x2 * y2
	if x2 != 0 && x2y2/x2 != y2 {
		panic(&OverflowError{})
	}

	result := x1y1
	result = safeAddUint64(result, x2y1)
	result = safeAddUint64(result, x1y2)
	result = safeAddUint64(result, x2y2)
	return UFix64Value(result)
}

func (v UFix64Value) Div(other NumberValue) NumberValue {
	// TODO:
	panic("TODO")
}

func (v UFix64Value) Mod(other NumberValue) NumberValue {
	// TODO:
	panic("TODO")
}

func (v UFix64Value) Less(other NumberValue) BoolValue {
	return v < other.(UFix64Value)
}

func (v UFix64Value) LessEqual(other NumberValue) BoolValue {
	return v <= other.(UFix64Value)
}

func (v UFix64Value) Greater(other NumberValue) BoolValue {
	return v > other.(UFix64Value)
}

func (v UFix64Value) GreaterEqual(other NumberValue) BoolValue {
	return v >= other.(UFix64Value)
}

func (v UFix64Value) Equal(other Value) BoolValue {
	otherUFix64, ok := other.(UFix64Value)
	if !ok {
		return false
	}
	return v == otherUFix64
}

func ConvertUFix64(value Value, interpreter *Interpreter) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141

	switch value := value.(type) {
	case Fix64Value:
		if value < 0 {
			panic("can't convert negative Fix64 to UFix64")
		}
		return UFix64Value(value)

	case UFix64Value:
		return value

	case NumberValue:
		return UFix64Value(value.IntValue() * sema.Fix64Factor)

	default:
		panic(fmt.Sprintf("can't convert %s to UFix64", value.DynamicType(interpreter)))
	}
}

// CompositeValue

type CompositeValue struct {
	Location       ast.Location
	TypeID         sema.TypeID
	Kind           common.CompositeKind
	Fields         map[string]Value
	InjectedFields map[string]Value
	NestedValues   map[string]Value
	Functions      map[string]FunctionValue
	Destructor     FunctionValue
	Owner          *common.Address
	Destroyed      bool
}

func init() {
	gob.Register(&CompositeValue{})
}

func (v *CompositeValue) Destroy(interpreter *Interpreter, location LocationPosition) trampoline.Trampoline {

	// if composite was deserialized, dynamically link in the destructor
	if v.Destructor == nil {
		v.Destructor = interpreter.typeCodes.compositeCodes[v.TypeID].destructorFunction
	}

	destructor := v.Destructor

	var tramp trampoline.Trampoline

	if destructor == nil {
		tramp = trampoline.Done{Result: VoidValue{}}
	} else {
		invocation := Invocation{
			Self:          v,
			Arguments:     nil,
			ArgumentTypes: nil,
			Location:      location,
			Interpreter:   interpreter,
		}

		tramp = destructor.Invoke(invocation)
	}

	return tramp.Then(func(_ interface{}) {
		v.Destroyed = true
	})
}

func (*CompositeValue) IsValue() {}

func (v *CompositeValue) DynamicType(interpreter *Interpreter) DynamicType {
	locationID, qualifiedIdentifier := sema.SplitCompositeTypeID(v.TypeID)
	if locationID == "" {
		panic("failed to split composite type ID")
	}

	checker := interpreter.allCheckers[locationID]
	staticType := checker.Elaboration.CompositeTypes[qualifiedIdentifier]

	return CompositeType{StaticType: staticType}
}

func (v *CompositeValue) Copy() Value {
	// Resources and contracts are not copied
	switch v.Kind {
	case common.CompositeKindResource, common.CompositeKindContract:
		return v

	default:
		break
	}

	newFields := make(map[string]Value, len(v.Fields))
	for field, value := range v.Fields {
		newFields[field] = value.Copy()
	}

	// NOTE: not copying functions or destructor – they are linked in

	return &CompositeValue{
		Location:       v.Location,
		TypeID:         v.TypeID,
		Kind:           v.Kind,
		Fields:         newFields,
		InjectedFields: v.InjectedFields,
		NestedValues:   v.NestedValues,
		Functions:      v.Functions,
		Destructor:     v.Destructor,
		Destroyed:      v.Destroyed,
		// NOTE: new value has no owner
		Owner: nil,
	}
}

func (v *CompositeValue) checkStatus(locationRange LocationRange) {
	if v.Destroyed {
		panic(&DestroyedCompositeError{
			CompositeKind: v.Kind,
			LocationRange: locationRange,
		})
	}
}

func (v *CompositeValue) GetOwner() *common.Address {
	return v.Owner
}

func (v *CompositeValue) SetOwner(owner *common.Address) {
	if v.Owner == owner {
		return
	}

	v.Owner = owner

	for _, value := range v.Fields {
		value.SetOwner(owner)
	}
}

func (v *CompositeValue) GetMember(interpreter *Interpreter, locationRange LocationRange, name string) Value {
	v.checkStatus(locationRange)

	if v.Kind == common.CompositeKindResource &&
		name == "owner" {

		if v.Owner == nil {
			return NilValue{}
		}

		address := AddressValue(*v.Owner)

		return NewSomeValueOwningNonCopying(NewPublicAccountValue(address))
	}

	value, ok := v.Fields[name]
	if ok {
		return value
	}

	value, ok = v.NestedValues[name]
	if ok {
		return value
	}

	// Get the correct interpreter. The program code might need to be loaded.
	// NOTE: standard library values have no location

	if v.Location != nil && !ast.LocationsMatch(interpreter.Checker.Location, v.Location) {
		interpreter = interpreter.ensureLoaded(v.Location, func() *ast.Program {
			return interpreter.importProgramHandler(interpreter, v.Location)
		})
	}

	// If the composite value was deserialized, dynamically link in the functions
	// and get injected fields

	if v.Functions == nil {
		v.Functions = interpreter.typeCodes.compositeCodes[v.TypeID].compositeFunctions
	}

	if v.InjectedFields == nil && interpreter.injectedCompositeFieldsHandler != nil {
		v.InjectedFields = interpreter.injectedCompositeFieldsHandler(
			interpreter,
			v.Location,
			v.TypeID,
			v.Kind,
		)
	}

	if v.InjectedFields != nil {
		value, ok = v.InjectedFields[name]
		if ok {
			return value
		}
	}

	function, ok := v.Functions[name]
	if ok {
		return BoundFunctionValue{
			Self:     v,
			Function: function,
		}
	}

	return nil
}

func (v *CompositeValue) SetMember(_ *Interpreter, locationRange LocationRange, name string, value Value) {
	v.checkStatus(locationRange)

	value.SetOwner(v.Owner)

	v.Fields[name] = value
}

func (v *CompositeValue) GobEncode() ([]byte, error) {
	w := new(bytes.Buffer)
	encoder := gob.NewEncoder(w)
	// NOTE: important: decode as pointer, so gob sees
	// the interface, not the concrete type
	err := encoder.Encode(&v.Location)
	if err != nil {
		return nil, err
	}
	err = encoder.Encode(v.TypeID)
	if err != nil {
		return nil, err
	}
	err = encoder.Encode(v.Kind)
	if err != nil {
		return nil, err
	}
	err = encoder.Encode(v.Fields)
	if err != nil {
		return nil, err
	}
	// NOTE: *not* encoding functions and destructor – linked in on-demand
	return w.Bytes(), nil
}

func (v *CompositeValue) GobDecode(buf []byte) error {
	r := bytes.NewBuffer(buf)
	decoder := gob.NewDecoder(r)
	err := decoder.Decode(&v.Location)
	if err != nil {
		return err
	}
	err = decoder.Decode(&v.TypeID)
	if err != nil {
		return err
	}
	err = decoder.Decode(&v.Kind)
	if err != nil {
		return err
	}
	err = decoder.Decode(&v.Fields)
	if err != nil {
		return err
	}
	// NOTE: *not* decoding functions – linked in on-demand
	return nil
}

func (v *CompositeValue) String() string {
	var builder strings.Builder
	builder.WriteString(string(v.TypeID))
	builder.WriteString("(")
	i := 0
	for name, value := range v.Fields {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(name)
		builder.WriteString(": ")
		builder.WriteString(fmt.Sprint(value))
		i++
	}
	builder.WriteString(")")
	return builder.String()
}

func (v *CompositeValue) GetField(name string) Value {
	return v.Fields[name]
}

// DictionaryValue

type DictionaryValue struct {
	Keys    *ArrayValue
	Entries map[string]Value
	Owner   *common.Address
}

func NewDictionaryValueUnownedNonCopying(keysAndValues ...Value) *DictionaryValue {
	keysAndValuesCount := len(keysAndValues)
	if keysAndValuesCount%2 != 0 {
		panic("uneven number of keys and values")
	}

	result := &DictionaryValue{
		Keys:    NewArrayValueUnownedNonCopying(),
		Entries: make(map[string]Value, keysAndValuesCount/2),
		// NOTE: new value has no owner
		Owner: nil,
	}

	for i := 0; i < keysAndValuesCount; i += 2 {
		result.Insert(keysAndValues[i], keysAndValues[i+1])
	}

	return result
}

func init() {
	gob.Register(&DictionaryValue{})
}

func (*DictionaryValue) IsValue() {}

func (v *DictionaryValue) DynamicType(interpreter *Interpreter) DynamicType {
	entryTypes := make([]struct{ KeyType, ValueType DynamicType }, len(v.Keys.Values))

	for i, key := range v.Keys.Values {
		entryTypes[i] =
			struct{ KeyType, ValueType DynamicType }{
				KeyType:   key.DynamicType(interpreter),
				ValueType: v.Entries[dictionaryKey(key)].DynamicType(interpreter),
			}
	}

	return DictionaryType{
		EntryTypes: entryTypes,
	}
}

func (v *DictionaryValue) Copy() Value {
	newKeys := v.Keys.Copy().(*ArrayValue)

	newEntries := make(map[string]Value, len(v.Entries))
	for name, value := range v.Entries {
		newEntries[name] = value.Copy()
	}

	return &DictionaryValue{
		Keys:    newKeys,
		Entries: newEntries,
		// NOTE: new value has no owner
		Owner: nil,
	}
}

func (v *DictionaryValue) GetOwner() *common.Address {
	return v.Owner
}

func (v *DictionaryValue) SetOwner(owner *common.Address) {
	if v.Owner == owner {
		return
	}

	v.Owner = owner

	v.Keys.SetOwner(owner)

	for _, value := range v.Entries {
		value.SetOwner(owner)
	}
}

func (v *DictionaryValue) Destroy(interpreter *Interpreter, location LocationPosition) trampoline.Trampoline {
	var result trampoline.Trampoline = trampoline.Done{}

	maybeDestroy := func(value interface{}) {
		destroyableValue, ok := value.(DestroyableValue)
		if !ok {
			return
		}

		result = result.
			FlatMap(func(_ interface{}) trampoline.Trampoline {
				return destroyableValue.Destroy(interpreter, location)
			})
	}

	for _, keyValue := range v.Keys.Values {
		maybeDestroy(keyValue)
	}

	for _, value := range v.Entries {
		maybeDestroy(value)
	}

	return result
}

func (v *DictionaryValue) Get(_ *Interpreter, _ LocationRange, keyValue Value) Value {
	value, ok := v.Entries[dictionaryKey(keyValue)]
	if !ok {
		return NilValue{}
	}
	return NewSomeValueOwningNonCopying(value)
}

func dictionaryKey(keyValue Value) string {
	hasKeyString, ok := keyValue.(HasKeyString)
	if !ok {
		panic(errors.NewUnreachableError())
	}
	return hasKeyString.KeyString()
}

func (v *DictionaryValue) Set(_ *Interpreter, _ LocationRange, keyValue Value, value Value) {
	switch typedValue := value.(type) {
	case *SomeValue:
		v.Insert(keyValue, typedValue.Value)

	case NilValue:
		v.Remove(keyValue)
		return

	default:
		panic(errors.NewUnreachableError())
	}
}

func (v *DictionaryValue) String() string {
	var builder strings.Builder
	builder.WriteString("{")
	i := 0
	for _, keyValue := range v.Keys.Values {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(fmt.Sprint(keyValue))
		builder.WriteString(": ")

		key := dictionaryKey(keyValue)
		value := v.Entries[key]
		builder.WriteString(fmt.Sprint(value))

		i++
	}
	builder.WriteString("}")
	return builder.String()
}

func (v *DictionaryValue) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {
	case "length":
		return NewIntValue(int64(v.Count()))

	// TODO: is returning copies correct?
	case "keys":
		return v.Keys.Copy()

	// TODO: is returning copies correct?
	case "values":
		dictionaryValues := make([]Value, v.Count())
		i := 0
		for _, keyValue := range v.Keys.Values {
			key := dictionaryKey(keyValue)
			dictionaryValues[i] = v.Entries[key].Copy()
			i++
		}
		return NewArrayValueUnownedNonCopying(dictionaryValues...)

	case "remove":
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				keyValue := invocation.Arguments[0]

				existingValue := v.Remove(keyValue)

				var returnValue Value
				if existingValue == nil {
					returnValue = NilValue{}
				} else {
					returnValue = NewSomeValueOwningNonCopying(existingValue)
				}

				return trampoline.Done{
					Result: returnValue,
				}
			},
		)

	case "insert":
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				keyValue := invocation.Arguments[0]
				newValue := invocation.Arguments[1]

				existingValue := v.Insert(keyValue, newValue)

				var returnValue Value
				if existingValue == nil {
					returnValue = NilValue{}
				} else {
					returnValue = NewSomeValueOwningNonCopying(existingValue)
				}

				return trampoline.Done{
					Result: returnValue,
				}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (v *DictionaryValue) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	// Dictionaries have no settable members (fields / functions)
	panic(errors.NewUnreachableError())
}

func (v *DictionaryValue) Count() int {
	return v.Keys.Count()
}

// TODO: unset owner?
func (v *DictionaryValue) Remove(keyValue Value) (existingValue Value) {
	key := dictionaryKey(keyValue)
	existingValue, exists := v.Entries[key]

	if !exists {
		return nil
	}

	delete(v.Entries, key)

	// TODO: optimize linear scan
	for i, keyValue := range v.Keys.Values {
		if dictionaryKey(keyValue) == key {
			v.Keys.Remove(i)
			return existingValue
		}
	}

	panic(errors.NewUnreachableError())
}

func (v *DictionaryValue) Insert(keyValue Value, value Value) (existingValue Value) {
	key := dictionaryKey(keyValue)
	existingValue, existed := v.Entries[key]

	if !existed {
		v.Keys.Append(keyValue)
	}

	value.SetOwner(v.Owner)

	v.Entries[key] = value

	if !existed {
		return nil
	}

	return existingValue
}

type DictionaryEntryValues struct {
	Key   Value
	Value Value
}

// ToValue converts a Go value into an interpreter value
func ToValue(value interface{}) (Value, error) {
	// TODO: support more types
	switch value := value.(type) {
	case *big.Int:
		return IntValue{value}, nil
	case int:
		return NewIntValue(int64(value)), nil
	case int8:
		return Int8Value(value), nil
	case int16:
		return Int16Value(value), nil
	case int32:
		return Int32Value(value), nil
	case int64:
		return Int64Value(value), nil
	case uint8:
		return UInt8Value(value), nil
	case uint16:
		return UInt16Value(value), nil
	case uint32:
		return UInt32Value(value), nil
	case uint64:
		return UInt64Value(value), nil
	case bool:
		return BoolValue(value), nil
	case string:
		return NewStringValue(value), nil
	case nil:
		return NilValue{}, nil
	}

	return nil, fmt.Errorf("cannot convert Go value to value: %#+v", value)
}

func ToValues(inputs []interface{}) ([]Value, error) {
	var newValues []Value
	for _, argument := range inputs {
		value, ok := argument.(Value)
		if !ok {
			var err error
			value, err = ToValue(argument)
			if err != nil {
				return nil, err
			}
		}
		newValues = append(
			newValues,
			value,
		)
	}
	return newValues, nil
}

// OptionalValue

type OptionalValue interface {
	Value
	isOptionalValue()
}

// NilValue

type NilValue struct{}

func init() {
	gob.Register(NilValue{})
}

func (NilValue) IsValue() {}

func (NilValue) DynamicType(_ *Interpreter) DynamicType {
	return NilType{}
}

func (NilValue) isOptionalValue() {}

func (v NilValue) Copy() Value {
	return v
}

func (NilValue) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (NilValue) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v NilValue) Destroy(_ *Interpreter, _ LocationPosition) trampoline.Trampoline {
	return trampoline.Done{}
}

func (NilValue) String() string {
	return "nil"
}

// SomeValue

type SomeValue struct {
	Value Value
	Owner *common.Address
}

func init() {
	gob.Register(&SomeValue{})
}

func NewSomeValueOwningNonCopying(value Value) *SomeValue {
	return &SomeValue{
		Value: value,
		Owner: value.GetOwner(),
	}
}

func (*SomeValue) IsValue() {}

func (v *SomeValue) DynamicType(interpreter *Interpreter) DynamicType {
	innerType := v.Value.DynamicType(interpreter)
	return SomeType{InnerType: innerType}
}

func (*SomeValue) isOptionalValue() {}

func (v *SomeValue) Copy() Value {
	return &SomeValue{
		Value: v.Value.Copy(),
		// NOTE: new value has no owner
		Owner: nil,
	}
}

func (v *SomeValue) GetOwner() *common.Address {
	return v.Owner
}

func (v *SomeValue) SetOwner(owner *common.Address) {
	if v.Owner == owner {
		return
	}

	v.Owner = owner

	v.Value.SetOwner(owner)
}

func (v *SomeValue) Destroy(interpreter *Interpreter, location LocationPosition) trampoline.Trampoline {
	return v.Value.(DestroyableValue).Destroy(interpreter, location)
}

func (v *SomeValue) String() string {
	return fmt.Sprint(v.Value)
}

// StorageValue

type StorageValue struct {
	Address common.Address
}

func (StorageValue) IsValue() {}

func (v StorageValue) DynamicType(_ *Interpreter) DynamicType {
	return StorageType{}
}

func (v StorageValue) Copy() Value {
	return StorageValue{
		Address: v.Address,
	}
}

func (v StorageValue) GetOwner() *common.Address {
	return &v.Address
}

func (StorageValue) SetOwner(_ *common.Address) {
	// NO-OP: ownership cannot be changed
}

// PublishedValue

type PublishedValue struct {
	Address common.Address
}

func (PublishedValue) IsValue() {}

func (v PublishedValue) DynamicType(_ *Interpreter) DynamicType {
	return PublishedType{}
}

func (v PublishedValue) Copy() Value {
	return PublishedValue{
		Address: v.Address,
	}
}

func (v PublishedValue) GetOwner() *common.Address {
	return &v.Address
}

func (PublishedValue) SetOwner(_ *common.Address) {
	// NO-OP: ownership cannot be changed
}

// StorageReferenceValue

type StorageReferenceValue struct {
	Authorized           bool
	TargetStorageAddress common.Address
	TargetKey            string
	Owner                *common.Address
}

func init() {
	gob.Register(&StorageReferenceValue{})
}

func (*StorageReferenceValue) IsValue() {}

func (v *StorageReferenceValue) DynamicType(interpreter *Interpreter) DynamicType {
	referencedValue := v.referencedValue(interpreter)
	if referencedValue == nil {
		panic(&DereferenceError{})
	}

	innerType := (*referencedValue).DynamicType(interpreter)

	return StorageReferenceType{
		authorized: v.Authorized,
		innerType:  innerType,
	}
}

func (v *StorageReferenceValue) Copy() Value {
	return &StorageReferenceValue{
		Authorized:           v.Authorized,
		TargetStorageAddress: v.TargetStorageAddress,
		TargetKey:            v.TargetKey,
		// NOTE: new value has no owner
		Owner: nil,
	}
}

func (v *StorageReferenceValue) GetOwner() *common.Address {
	return v.Owner
}

func (v *StorageReferenceValue) SetOwner(owner *common.Address) {
	v.Owner = owner
}

func (v *StorageReferenceValue) referencedValue(interpreter *Interpreter) *Value {
	key := PrefixedStorageKey(v.TargetKey, AccessLevelPrivate)

	switch referenced := interpreter.readStored(v.TargetStorageAddress, key).(type) {
	case *SomeValue:
		return &referenced.Value
	case NilValue:
		return nil
	default:
		panic(errors.NewUnreachableError())
	}
}

func (v *StorageReferenceValue) GetMember(interpreter *Interpreter, locationRange LocationRange, name string) Value {
	referencedValue := v.referencedValue(interpreter)
	if referencedValue == nil {
		panic(&DereferenceError{
			LocationRange: locationRange,
		})
	}

	return (*referencedValue).(MemberAccessibleValue).
		GetMember(interpreter, locationRange, name)
}

func (v *StorageReferenceValue) SetMember(interpreter *Interpreter, locationRange LocationRange, name string, value Value) {
	referencedValue := v.referencedValue(interpreter)
	if referencedValue == nil {
		panic(&DereferenceError{
			LocationRange: locationRange,
		})
	}

	(*referencedValue).(MemberAccessibleValue).
		SetMember(interpreter, locationRange, name, value)
}

func (v *StorageReferenceValue) Get(interpreter *Interpreter, locationRange LocationRange, key Value) Value {
	referencedValue := v.referencedValue(interpreter)
	if referencedValue == nil {
		panic(&DereferenceError{
			LocationRange: locationRange,
		})
	}

	return (*referencedValue).(ValueIndexableValue).
		Get(interpreter, locationRange, key)
}

func (v *StorageReferenceValue) Set(interpreter *Interpreter, locationRange LocationRange, key Value, value Value) {
	referencedValue := v.referencedValue(interpreter)
	if referencedValue == nil {
		panic(&DereferenceError{
			LocationRange: locationRange,
		})
	}

	(*referencedValue).(ValueIndexableValue).
		Set(interpreter, locationRange, key, value)
}

func (v *StorageReferenceValue) Equal(other Value) BoolValue {
	otherReference, ok := other.(*StorageReferenceValue)
	if !ok {
		return false
	}

	return v.TargetStorageAddress == otherReference.TargetStorageAddress &&
		v.TargetKey == otherReference.TargetKey &&
		v.Authorized == otherReference.Authorized
}

// EphemeralReferenceValue

type EphemeralReferenceValue struct {
	Authorized bool
	Value      Value
}

func (*EphemeralReferenceValue) IsValue() {}

func (v *EphemeralReferenceValue) DynamicType(interpreter *Interpreter) DynamicType {
	innerType := v.Value.DynamicType(interpreter)

	return EphemeralReferenceType{
		authorized: v.Authorized,
		innerType:  innerType,
	}
}

func (v *EphemeralReferenceValue) Copy() Value {
	return v
}

func (v *EphemeralReferenceValue) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (v *EphemeralReferenceValue) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v *EphemeralReferenceValue) GetMember(interpreter *Interpreter, locationRange LocationRange, name string) Value {
	return v.Value.(MemberAccessibleValue).
		GetMember(interpreter, locationRange, name)
}

func (v *EphemeralReferenceValue) SetMember(interpreter *Interpreter, locationRange LocationRange, name string, value Value) {
	v.Value.(MemberAccessibleValue).
		SetMember(interpreter, locationRange, name, value)
}

func (v *EphemeralReferenceValue) Get(interpreter *Interpreter, locationRange LocationRange, key Value) Value {
	return v.Value.(ValueIndexableValue).
		Get(interpreter, locationRange, key)
}

func (v *EphemeralReferenceValue) Set(interpreter *Interpreter, locationRange LocationRange, key Value, value Value) {
	v.Value.(ValueIndexableValue).
		Set(interpreter, locationRange, key, value)
}

func (v *EphemeralReferenceValue) Equal(other Value) BoolValue {
	otherReference, ok := other.(*EphemeralReferenceValue)
	if !ok {
		return false
	}

	return v.Value == otherReference.Value &&
		v.Authorized == otherReference.Authorized
}

// AddressValue

type AddressValue common.Address

func init() {
	gob.Register(AddressValue{})
}

func NewAddressValue(a common.Address) AddressValue {
	return NewAddressValueFromBytes(a[:])
}

func NewAddressValueFromBytes(b []byte) AddressValue {
	result := AddressValue{}
	copy(result[common.AddressLength-len(b):], b)
	return result
}

func ConvertAddress(value Value, _ *Interpreter) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	result := AddressValue{}
	if intValue, ok := value.(IntValue); ok {
		bigEndianBytes := intValue.Int.Bytes()
		copy(
			result[common.AddressLength-len(bigEndianBytes):common.AddressLength],
			bigEndianBytes,
		)
	} else {
		binary.BigEndian.PutUint64(
			result[common.AddressLength-8:common.AddressLength],
			uint64(value.(NumberValue).IntValue()),
		)
	}
	return result
}

func (AddressValue) IsValue() {}

func (AddressValue) DynamicType(_ *Interpreter) DynamicType {
	return AddressType{}
}

func (v AddressValue) Copy() Value {
	return v
}

func (v AddressValue) KeyString() string {
	return v.String()
}

func (v AddressValue) String() string {
	return fmt.Sprintf("%x", [common.AddressLength]byte(v))
}

func (AddressValue) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (AddressValue) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v AddressValue) Equal(other Value) BoolValue {
	otherAddress, ok := other.(AddressValue)
	if !ok {
		return false
	}
	return [common.AddressLength]byte(v) == [common.AddressLength]byte(otherAddress)
}

func (v AddressValue) Hex() string {
	return v.ToAddress().Hex()
}

func (v AddressValue) ToAddress() common.Address {
	return common.Address(v)
}

// AccountValue

func NewAccountValue(addressValue AddressValue, setCode, addPublicKey, removePublicKey FunctionValue) *CompositeValue {
	address := addressValue.ToAddress()

	return &CompositeValue{
		Kind:   common.CompositeKindStructure,
		TypeID: (&sema.AccountType{}).ID(),
		InjectedFields: map[string]Value{
			"address":         addressValue,
			"storage":         StorageValue{Address: address},
			"published":       PublishedValue{Address: address},
			"setCode":         setCode,
			"addPublicKey":    addPublicKey,
			"removePublicKey": removePublicKey,
		},
	}
}

// PublicAccountValue

func NewPublicAccountValue(addressValue AddressValue) *CompositeValue {
	address := addressValue.ToAddress()

	return &CompositeValue{
		Kind:   common.CompositeKindStructure,
		TypeID: (&sema.PublicAccountType{}).ID(),
		InjectedFields: map[string]Value{
			"address": addressValue,
			"published": PublishedValue{
				Address: address,
			},
		},
	}
}

// PathValue

type PathValue struct {
	Domain     string
	Identifier string
}

func init() {
	gob.Register(PathValue{})
}

func (PathValue) IsValue() {}

func (PathValue) DynamicType(_ *Interpreter) DynamicType {
	return NilType{}
}

func (PathValue) isOptionalValue() {}

func (v PathValue) Copy() Value {
	return v
}

func (PathValue) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (PathValue) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v PathValue) Destroy(_ *Interpreter, _ LocationPosition) trampoline.Trampoline {
	return trampoline.Done{}
}

func (v PathValue) String() string {
	return fmt.Sprintf("/%s/%s", v.Domain, v.Identifier)
}
