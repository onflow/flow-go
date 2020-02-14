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
	Copy() Value
	GetOwner() string
	SetOwner(owner string)
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

func (v VoidValue) Copy() Value {
	return v
}

func (VoidValue) GetOwner() string {
	// value is never owned
	return ""
}

func (VoidValue) SetOwner(_ string) {
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

func (v BoolValue) Copy() Value {
	return v
}

func (BoolValue) GetOwner() string {
	// value is never owned
	return ""
}

func (BoolValue) SetOwner(_ string) {
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

func (v *StringValue) Copy() Value {
	return &StringValue{Str: v.Str}
}

func (*StringValue) GetOwner() string {
	// value is never owned
	return ""
}

func (*StringValue) SetOwner(_ string) {
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
	i := key.(IntegerValue).IntValue()

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
	i := key.(IntegerValue).IntValue()
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
	Owner  string
}

func init() {
	gob.Register(&ArrayValue{})
}

func NewArrayValueUnownedNonCopying(values ...Value) *ArrayValue {
	// NOTE: new value has no owner
	const noOwner = ""

	for _, value := range values {
		value.SetOwner(noOwner)
	}

	return &ArrayValue{
		Values: values,
		Owner:  noOwner,
	}
}

func (*ArrayValue) IsValue() {}

func (v *ArrayValue) Copy() Value {
	// TODO: optimize, use copy-on-write
	copies := make([]Value, len(v.Values))
	for i, value := range v.Values {
		copies[i] = value.Copy()
	}
	return NewArrayValueUnownedNonCopying(copies...)
}

func (v *ArrayValue) GetOwner() string {
	return v.Owner
}

func (v *ArrayValue) SetOwner(owner string) {
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
	integerKey := key.(IntegerValue).IntValue()
	return v.Values[integerKey]
}

func (v *ArrayValue) Set(_ *Interpreter, _ LocationRange, key Value, value Value) {
	value.SetOwner(v.Owner)

	integerKey := key.(IntegerValue).IntValue()
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
				i := invocation.Arguments[0].(IntegerValue).IntValue()
				element := invocation.Arguments[1]
				v.Insert(i, element)
				return trampoline.Done{Result: VoidValue{}}
			},
		)

	case "remove":
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				i := invocation.Arguments[0].(IntegerValue).IntValue()
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

// IntegerValue

type IntegerValue interface {
	Value
	IntValue() int
	Negate() IntegerValue
	Plus(other IntegerValue) IntegerValue
	Minus(other IntegerValue) IntegerValue
	Mod(other IntegerValue) IntegerValue
	Mul(other IntegerValue) IntegerValue
	Div(other IntegerValue) IntegerValue
	Less(other IntegerValue) BoolValue
	LessEqual(other IntegerValue) BoolValue
	Greater(other IntegerValue) BoolValue
	GreaterEqual(other IntegerValue) BoolValue
	Equal(other Value) BoolValue
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

func ConvertInt(value Value) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	if intValue, ok := value.(IntValue); ok {
		return intValue.Copy()
	}
	return NewIntValue(int64(value.(IntegerValue).IntValue()))
}

func (v IntValue) IsValue() {}

func (v IntValue) Copy() Value {
	return IntValue{big.NewInt(0).Set(v.Int)}
}

func (IntValue) GetOwner() string {
	// value is never owned
	return ""
}

func (IntValue) SetOwner(_ string) {
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

func (v IntValue) Negate() IntegerValue {
	return IntValue{big.NewInt(0).Neg(v.Int)}
}

func (v IntValue) Plus(other IntegerValue) IntegerValue {
	o := other.(IntValue)
	res := big.NewInt(0)
	res.Add(v.Int, o.Int)
	return IntValue{res}
}

func (v IntValue) Minus(other IntegerValue) IntegerValue {
	o := other.(IntValue)
	res := big.NewInt(0)
	res.Sub(v.Int, o.Int)
	return IntValue{res}
}

func (v IntValue) Mod(other IntegerValue) IntegerValue {
	o := other.(IntValue)
	res := big.NewInt(0)
	// INT33-C
	if o.Int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Mod(v.Int, o.Int)
	return IntValue{res}
}

func (v IntValue) Mul(other IntegerValue) IntegerValue {
	o := other.(IntValue)
	res := big.NewInt(0)
	res.Mul(v.Int, o.Int)
	return IntValue{res}
}

func (v IntValue) Div(other IntegerValue) IntegerValue {
	o := other.(IntValue)
	res := big.NewInt(0)
	// INT33-C
	if o.Int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Div(v.Int, o.Int)
	return IntValue{res}
}

func (v IntValue) Less(other IntegerValue) BoolValue {
	cmp := v.Int.Cmp(other.(IntValue).Int)
	return cmp == -1
}

func (v IntValue) LessEqual(other IntegerValue) BoolValue {
	cmp := v.Int.Cmp(other.(IntValue).Int)
	return cmp <= 0
}

func (v IntValue) Greater(other IntegerValue) BoolValue {
	cmp := v.Int.Cmp(other.(IntValue).Int)
	return cmp == 1
}

func (v IntValue) GreaterEqual(other IntegerValue) BoolValue {
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

func (v Int8Value) Copy() Value {
	return v
}

func (Int8Value) GetOwner() string {
	// value is never owned
	return ""
}

func (Int8Value) SetOwner(_ string) {
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

func (v Int8Value) Negate() IntegerValue {
	// INT32-C
	if v == math.MinInt8 {
		panic(&OverflowError{})
	}
	return -v
}

func (v Int8Value) Plus(other IntegerValue) IntegerValue {
	o := other.(Int8Value)
	// INT32-C
	if (o > 0) && (v > (math.MaxInt8 - o)) {
		panic(&OverflowError{})
	} else if (o < 0) && (v < (math.MinInt8 - o)) {
		panic(&UnderflowError{})
	}
	return v + o
}

func (v Int8Value) Minus(other IntegerValue) IntegerValue {
	o := other.(Int8Value)
	// INT32-C
	if (o > 0) && (v < (math.MinInt8 + o)) {
		panic(&OverflowError{})
	} else if (o < 0) && (v > (math.MaxInt8 + o)) {
		panic(&UnderflowError{})
	}
	return v - o
}

func (v Int8Value) Mod(other IntegerValue) IntegerValue {
	o := other.(Int8Value)
	// INT33-C
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v Int8Value) Mul(other IntegerValue) IntegerValue {
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

func (v Int8Value) Div(other IntegerValue) IntegerValue {
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

func (v Int8Value) Less(other IntegerValue) BoolValue {
	return v < other.(Int8Value)
}

func (v Int8Value) LessEqual(other IntegerValue) BoolValue {
	return v <= other.(Int8Value)
}

func (v Int8Value) Greater(other IntegerValue) BoolValue {
	return v > other.(Int8Value)
}

func (v Int8Value) GreaterEqual(other IntegerValue) BoolValue {
	return v >= other.(Int8Value)
}

func (v Int8Value) Equal(other Value) BoolValue {
	otherInt8, ok := other.(Int8Value)
	if !ok {
		return false
	}
	return v == otherInt8
}

func ConvertInt8(value Value) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return Int8Value(value.(IntegerValue).IntValue())
}

// Int16Value

type Int16Value int16

func init() {
	gob.Register(Int16Value(0))
}

func (Int16Value) IsValue() {}

func (v Int16Value) Copy() Value {
	return v
}

func (Int16Value) GetOwner() string {
	// value is never owned
	return ""
}

func (Int16Value) SetOwner(_ string) {
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

func (v Int16Value) Negate() IntegerValue {
	// INT32-C
	if v == math.MinInt16 {
		panic(&OverflowError{})
	}
	return -v
}

func (v Int16Value) Plus(other IntegerValue) IntegerValue {
	o := other.(Int16Value)
	// INT32-C
	if (o > 0) && (v > (math.MaxInt16 - o)) {
		panic(&OverflowError{})
	} else if (o < 0) && (v < (math.MinInt16 - o)) {
		panic(&UnderflowError{})
	}
	return v + o
}

func (v Int16Value) Minus(other IntegerValue) IntegerValue {
	o := other.(Int16Value)
	// INT32-C
	if (o > 0) && (v < (math.MinInt16 + o)) {
		panic(&OverflowError{})
	} else if (o < 0) && (v > (math.MaxInt16 + o)) {
		panic(&UnderflowError{})
	}
	return v - o
}

func (v Int16Value) Mod(other IntegerValue) IntegerValue {
	o := other.(Int16Value)
	// INT33-C
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v Int16Value) Mul(other IntegerValue) IntegerValue {
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

func (v Int16Value) Div(other IntegerValue) IntegerValue {
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

func (v Int16Value) Less(other IntegerValue) BoolValue {
	return v < other.(Int16Value)
}

func (v Int16Value) LessEqual(other IntegerValue) BoolValue {
	return v <= other.(Int16Value)
}

func (v Int16Value) Greater(other IntegerValue) BoolValue {
	return v > other.(Int16Value)
}

func (v Int16Value) GreaterEqual(other IntegerValue) BoolValue {
	return v >= other.(Int16Value)
}

func (v Int16Value) Equal(other Value) BoolValue {
	otherInt16, ok := other.(Int16Value)
	if !ok {
		return false
	}
	return v == otherInt16
}

func ConvertInt16(value Value) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return Int16Value(value.(IntegerValue).IntValue())
}

// Int32Value

type Int32Value int32

func init() {
	gob.Register(Int32Value(0))
}

func (Int32Value) IsValue() {}

func (v Int32Value) Copy() Value {
	return v
}

func (Int32Value) GetOwner() string {
	// value is never owned
	return ""
}

func (Int32Value) SetOwner(_ string) {
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

func (v Int32Value) Negate() IntegerValue {
	// INT32-C
	if v == math.MinInt32 {
		panic(&OverflowError{})
	}
	return -v
}

func (v Int32Value) Plus(other IntegerValue) IntegerValue {
	o := other.(Int32Value)
	// INT32-C
	if (o > 0) && (v > (math.MaxInt32 - o)) {
		panic(&OverflowError{})
	} else if (o < 0) && (v < (math.MinInt32 - o)) {
		panic(&UnderflowError{})
	}
	return v + o
}

func (v Int32Value) Minus(other IntegerValue) IntegerValue {
	o := other.(Int32Value)
	// INT32-C
	if (o > 0) && (v < (math.MinInt32 + o)) {
		panic(&OverflowError{})
	} else if (o < 0) && (v > (math.MaxInt32 + o)) {
		panic(&UnderflowError{})
	}
	return v - o
}

func (v Int32Value) Mod(other IntegerValue) IntegerValue {
	o := other.(Int32Value)
	// INT33-C
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v Int32Value) Mul(other IntegerValue) IntegerValue {
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

func (v Int32Value) Div(other IntegerValue) IntegerValue {
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

func (v Int32Value) Less(other IntegerValue) BoolValue {
	return v < other.(Int32Value)
}

func (v Int32Value) LessEqual(other IntegerValue) BoolValue {
	return v <= other.(Int32Value)
}

func (v Int32Value) Greater(other IntegerValue) BoolValue {
	return v > other.(Int32Value)
}

func (v Int32Value) GreaterEqual(other IntegerValue) BoolValue {
	return v >= other.(Int32Value)
}

func (v Int32Value) Equal(other Value) BoolValue {
	otherInt32, ok := other.(Int32Value)
	if !ok {
		return false
	}
	return v == otherInt32
}

func ConvertInt32(value Value) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return Int32Value(value.(IntegerValue).IntValue())
}

// Int64Value

type Int64Value int64

func init() {
	gob.Register(Int64Value(0))
}

func (Int64Value) IsValue() {}

func (v Int64Value) Copy() Value {
	return v
}

func (Int64Value) GetOwner() string {
	// value is never owned
	return ""
}

func (Int64Value) SetOwner(_ string) {
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

func (v Int64Value) Negate() IntegerValue {
	// INT32-C
	if v == math.MinInt64 {
		panic(&OverflowError{})
	}
	return -v
}

func (v Int64Value) Plus(other IntegerValue) IntegerValue {
	o := other.(Int64Value)
	// INT32-C
	if (o > 0) && (v > (math.MaxInt64 - o)) {
		panic(&OverflowError{})
	} else if (o < 0) && (v < (math.MinInt64 - o)) {
		panic(&UnderflowError{})
	}
	return v + o
}

func (v Int64Value) Minus(other IntegerValue) IntegerValue {
	o := other.(Int64Value)
	// INT32-C
	if (o > 0) && (v < (math.MinInt64 + o)) {
		panic(&OverflowError{})
	} else if (o < 0) && (v > (math.MaxInt64 + o)) {
		panic(&UnderflowError{})
	}
	return v - o
}

func (v Int64Value) Mod(other IntegerValue) IntegerValue {
	o := other.(Int64Value)
	// INT33-C
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v Int64Value) Mul(other IntegerValue) IntegerValue {
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

func (v Int64Value) Div(other IntegerValue) IntegerValue {
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

func (v Int64Value) Less(other IntegerValue) BoolValue {
	return v < other.(Int64Value)
}

func (v Int64Value) LessEqual(other IntegerValue) BoolValue {
	return v <= other.(Int64Value)
}

func (v Int64Value) Greater(other IntegerValue) BoolValue {
	return v > other.(Int64Value)
}

func (v Int64Value) GreaterEqual(other IntegerValue) BoolValue {
	return v >= other.(Int64Value)
}

func (v Int64Value) Equal(other Value) BoolValue {
	otherInt64, ok := other.(Int64Value)
	if !ok {
		return false
	}
	return v == otherInt64
}

func ConvertInt64(value Value) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return Int64Value(value.(IntegerValue).IntValue())
}

// Int128Value

type Int128Value struct {
	int *big.Int
}

func init() {
	gob.Register(Int128Value{})
}

func (v Int128Value) IsValue() {}

func (v Int128Value) Copy() Value {
	return Int128Value{big.NewInt(0).Set(v.int)}
}

func (Int128Value) GetOwner() string {
	// value is never owned
	return ""
}

func (Int128Value) SetOwner(_ string) {
	// NO-OP: value cannot be owned
}

func (v Int128Value) IntValue() int {
	// TODO: handle overflow
	return int(v.int.Int64())
}

func (v Int128Value) String() string {
	return v.int.String()
}

func (v Int128Value) KeyString() string {
	return v.int.String()
}

func (v Int128Value) Negate() IntegerValue {
	// INT32-C
	//   if v == Int128TypeMin {
	//       ...
	//   }
	if v.int.Cmp(sema.Int128TypeMin) == 0 {
		panic(&OverflowError{})
	}
	return Int128Value{big.NewInt(0).Neg(v.int)}
}

func (v Int128Value) Plus(other IntegerValue) IntegerValue {
	o := other.(Int128Value)
	// Given that this value is backed by an arbitrary size integer,
	// we can just add and check the range of the result.
	//
	// If Go gains a native int128 type and we switch this value
	// to be based on it, then we need to follow INT32-C:
	//
	//   if (o > 0) && (v > (Int128TypeMax - o)) {
	//       ...
	//   } else if (o < 0) && (v < (Int128TypeMin - o)) {
	//       ...
	//   }
	//
	res := big.NewInt(0)
	res.Add(v.int, o.int)
	if res.Cmp(sema.Int128TypeMin) < 0 {
		panic(UnderflowError{})
	} else if res.Cmp(sema.Int128TypeMax) > 0 {
		panic(OverflowError{})
	}
	return Int128Value{res}
}

func (v Int128Value) Minus(other IntegerValue) IntegerValue {
	o := other.(Int128Value)
	// Given that this value is backed by an arbitrary size integer,
	// we can just subtract and check the range of the result.
	//
	// If Go gains a native int128 type and we switch this value
	// to be based on it, then we need to follow INT32-C:
	//
	//   if (o > 0) && (v < (Int128TypeMin + o)) {
	// 	     ...
	//   } else if (o < 0) && (v > (Int128TypeMax + o)) {
	//       ...
	//   }
	//
	res := big.NewInt(0)
	res.Sub(v.int, o.int)
	if res.Cmp(sema.Int128TypeMin) < 0 {
		panic(UnderflowError{})
	} else if res.Cmp(sema.Int128TypeMax) > 0 {
		panic(OverflowError{})
	}
	return Int128Value{res}
}

func (v Int128Value) Mod(other IntegerValue) IntegerValue {
	o := other.(Int128Value)
	res := big.NewInt(0)
	// INT33-C
	if o.int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Mod(v.int, o.int)
	return Int128Value{res}
}

func (v Int128Value) Mul(other IntegerValue) IntegerValue {
	o := other.(Int128Value)
	res := big.NewInt(0)
	res.Mul(v.int, o.int)
	if res.Cmp(sema.Int128TypeMin) < 0 {
		panic(UnderflowError{})
	} else if res.Cmp(sema.Int128TypeMax) > 0 {
		panic(OverflowError{})
	}
	return Int128Value{res}
}

func (v Int128Value) Div(other IntegerValue) IntegerValue {
	o := other.(Int128Value)
	res := big.NewInt(0)
	// INT33-C:
	//   if o == 0 {
	//       ...
	//   } else if (v == Int128TypeMin) && (o == -1) {
	//       ...
	//   }
	if o.int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.SetInt64(-1)
	if (v.int.Cmp(sema.Int128TypeMin) == 0) && (o.int.Cmp(res) == 0) {
		panic(OverflowError{})
	}
	res.Div(v.int, o.int)
	return Int128Value{res}
}

func (v Int128Value) Less(other IntegerValue) BoolValue {
	cmp := v.int.Cmp(other.(Int128Value).int)
	return cmp == -1
}

func (v Int128Value) LessEqual(other IntegerValue) BoolValue {
	cmp := v.int.Cmp(other.(Int128Value).int)
	return cmp <= 0
}

func (v Int128Value) Greater(other IntegerValue) BoolValue {
	cmp := v.int.Cmp(other.(Int128Value).int)
	return cmp == 1
}

func (v Int128Value) GreaterEqual(other IntegerValue) BoolValue {
	cmp := v.int.Cmp(other.(Int128Value).int)
	return cmp >= 0
}

func (v Int128Value) Equal(other Value) BoolValue {
	otherInt, ok := other.(Int128Value)
	if !ok {
		return false
	}
	cmp := v.int.Cmp(otherInt.int)
	return cmp == 0
}

func ConvertInt128(value Value) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	intValue := value.(IntegerValue).IntValue()
	return Int128Value{big.NewInt(0).SetInt64(int64(intValue))}
}

// Int256Value

type Int256Value struct {
	int *big.Int
}

func init() {
	gob.Register(Int256Value{})
}

func (v Int256Value) IsValue() {}

func (v Int256Value) Copy() Value {
	return Int256Value{big.NewInt(0).Set(v.int)}
}

func (Int256Value) GetOwner() string {
	// value is never owned
	return ""
}

func (Int256Value) SetOwner(_ string) {
	// NO-OP: value cannot be owned
}

func (v Int256Value) IntValue() int {
	// TODO: handle overflow
	return int(v.int.Int64())
}

func (v Int256Value) String() string {
	return v.int.String()
}

func (v Int256Value) KeyString() string {
	return v.int.String()
}

func (v Int256Value) Negate() IntegerValue {
	// INT32-C
	//   if v == Int256TypeMin {
	//       ...
	//   }
	if v.int.Cmp(sema.Int256TypeMin) == 0 {
		panic(&OverflowError{})
	}
	return Int256Value{big.NewInt(0).Neg(v.int)}
}

func (v Int256Value) Plus(other IntegerValue) IntegerValue {
	o := other.(Int256Value)
	// Given that this value is backed by an arbitrary size integer,
	// we can just add and check the range of the result.
	//
	// If Go gains a native int256 type and we switch this value
	// to be based on it, then we need to follow INT32-C:
	//
	//   if (o > 0) && (v > (Int256TypeMax - o)) {
	//       ...
	//   } else if (o < 0) && (v < (Int256TypeMin - o)) {
	//       ...
	//   }
	//
	res := big.NewInt(0)
	res.Add(v.int, o.int)
	if res.Cmp(sema.Int256TypeMin) < 0 {
		panic(UnderflowError{})
	} else if res.Cmp(sema.Int256TypeMax) > 0 {
		panic(OverflowError{})
	}
	return Int256Value{res}
}

func (v Int256Value) Minus(other IntegerValue) IntegerValue {
	o := other.(Int256Value)
	// Given that this value is backed by an arbitrary size integer,
	// we can just subtract and check the range of the result.
	//
	// If Go gains a native int256 type and we switch this value
	// to be based on it, then we need to follow INT32-C:
	//
	//   if (o > 0) && (v < (Int256TypeMin + o)) {
	// 	     ...
	//   } else if (o < 0) && (v > (Int256TypeMax + o)) {
	//       ...
	//   }
	//
	res := big.NewInt(0)
	res.Sub(v.int, o.int)
	if res.Cmp(sema.Int256TypeMin) < 0 {
		panic(UnderflowError{})
	} else if res.Cmp(sema.Int256TypeMax) > 0 {
		panic(OverflowError{})
	}
	return Int256Value{res}
}

func (v Int256Value) Mod(other IntegerValue) IntegerValue {
	o := other.(Int256Value)
	res := big.NewInt(0)
	// INT33-C
	if o.int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Mod(v.int, o.int)
	return Int256Value{res}
}

func (v Int256Value) Mul(other IntegerValue) IntegerValue {
	o := other.(Int256Value)
	res := big.NewInt(0)
	res.Mul(v.int, o.int)
	if res.Cmp(sema.Int256TypeMin) < 0 {
		panic(UnderflowError{})
	} else if res.Cmp(sema.Int256TypeMax) > 0 {
		panic(OverflowError{})
	}
	return Int256Value{res}
}

func (v Int256Value) Div(other IntegerValue) IntegerValue {
	o := other.(Int256Value)
	res := big.NewInt(0)
	// INT33-C:
	//   if o == 0 {
	//       ...
	//   } else if (v == Int256TypeMin) && (o == -1) {
	//       ...
	//   }
	if o.int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.SetInt64(-1)
	if (v.int.Cmp(sema.Int256TypeMin) == 0) && (o.int.Cmp(res) == 0) {
		panic(OverflowError{})
	}
	res.Div(v.int, o.int)
	return Int256Value{res}
}

func (v Int256Value) Less(other IntegerValue) BoolValue {
	cmp := v.int.Cmp(other.(Int256Value).int)
	return cmp == -1
}

func (v Int256Value) LessEqual(other IntegerValue) BoolValue {
	cmp := v.int.Cmp(other.(Int256Value).int)
	return cmp <= 0
}

func (v Int256Value) Greater(other IntegerValue) BoolValue {
	cmp := v.int.Cmp(other.(Int256Value).int)
	return cmp == 1
}

func (v Int256Value) GreaterEqual(other IntegerValue) BoolValue {
	cmp := v.int.Cmp(other.(Int256Value).int)
	return cmp >= 0
}

func (v Int256Value) Equal(other Value) BoolValue {
	otherInt, ok := other.(Int256Value)
	if !ok {
		return false
	}
	cmp := v.int.Cmp(otherInt.int)
	return cmp == 0
}

func ConvertInt256(value Value) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	intValue := value.(IntegerValue).IntValue()
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

func ConvertUInt(value Value) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	if intValue, ok := value.(UIntValue); ok {
		return intValue.Copy()
	}
	return NewUIntValue(uint64(value.(IntegerValue).IntValue()))
}

func (v UIntValue) IsValue() {}

func (v UIntValue) Copy() Value {
	return UIntValue{big.NewInt(0).Set(v.Int)}
}

func (UIntValue) GetOwner() string {
	// value is never owned
	return ""
}

func (UIntValue) SetOwner(_ string) {
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

func (v UIntValue) Negate() IntegerValue {
	panic(errors.NewUnreachableError())
}

func (v UIntValue) Plus(other IntegerValue) IntegerValue {
	o := other.(UIntValue)
	res := big.NewInt(0)
	res.Add(v.Int, o.Int)
	return UIntValue{res}
}

func (v UIntValue) Minus(other IntegerValue) IntegerValue {
	o := other.(UIntValue)
	res := big.NewInt(0)
	res.Sub(v.Int, o.Int)
	if res.Sign() < 0 {
		panic(&UnderflowError{})
	}
	return UIntValue{res}
}

func (v UIntValue) Mod(other IntegerValue) IntegerValue {
	o := other.(UIntValue)
	res := big.NewInt(0)
	// INT33-C
	if o.Int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Mod(v.Int, o.Int)
	return UIntValue{res}
}

func (v UIntValue) Mul(other IntegerValue) IntegerValue {
	o := other.(UIntValue)
	res := big.NewInt(0)
	res.Mul(v.Int, o.Int)
	return UIntValue{res}
}

func (v UIntValue) Div(other IntegerValue) IntegerValue {
	o := other.(UIntValue)
	res := big.NewInt(0)
	// INT33-C
	if o.Int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Div(v.Int, o.Int)
	return UIntValue{res}
}

func (v UIntValue) Less(other IntegerValue) BoolValue {
	cmp := v.Int.Cmp(other.(UIntValue).Int)
	return cmp == -1
}

func (v UIntValue) LessEqual(other IntegerValue) BoolValue {
	cmp := v.Int.Cmp(other.(UIntValue).Int)
	return cmp <= 0
}

func (v UIntValue) Greater(other IntegerValue) BoolValue {
	cmp := v.Int.Cmp(other.(UIntValue).Int)
	return cmp == 1
}

func (v UIntValue) GreaterEqual(other IntegerValue) BoolValue {
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

func (v UInt8Value) Copy() Value {
	return v
}

func (UInt8Value) GetOwner() string {
	// value is never owned
	return ""
}

func (UInt8Value) SetOwner(_ string) {
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

func (v UInt8Value) Negate() IntegerValue {
	panic(errors.NewUnreachableError())
}

func (v UInt8Value) Plus(other IntegerValue) IntegerValue {
	sum := v + other.(UInt8Value)
	// INT30-C
	if sum < v {
		panic(OverflowError{})
	}
	return sum
}

func (v UInt8Value) Minus(other IntegerValue) IntegerValue {
	diff := v - other.(UInt8Value)
	// INT30-C
	if diff > v {
		panic(UnderflowError{})
	}
	return diff
}

func (v UInt8Value) Mod(other IntegerValue) IntegerValue {
	o := other.(UInt8Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v UInt8Value) Mul(other IntegerValue) IntegerValue {
	o := other.(UInt8Value)
	if (v > 0) && (o > 0) && (v > (math.MaxUint8 / o)) {
		panic(&OverflowError{})
	}
	return v * o
}

func (v UInt8Value) Div(other IntegerValue) IntegerValue {
	o := other.(UInt8Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v / o
}

func (v UInt8Value) Less(other IntegerValue) BoolValue {
	return v < other.(UInt8Value)
}

func (v UInt8Value) LessEqual(other IntegerValue) BoolValue {
	return v <= other.(UInt8Value)
}

func (v UInt8Value) Greater(other IntegerValue) BoolValue {
	return v > other.(UInt8Value)
}

func (v UInt8Value) GreaterEqual(other IntegerValue) BoolValue {
	return v >= other.(UInt8Value)
}

func (v UInt8Value) Equal(other Value) BoolValue {
	otherUInt8, ok := other.(UInt8Value)
	if !ok {
		return false
	}
	return v == otherUInt8
}

func ConvertUInt8(value Value) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return UInt8Value(value.(IntegerValue).IntValue())
}

// UInt16Value

type UInt16Value uint16

func init() {
	gob.Register(UInt16Value(0))
}

func (UInt16Value) IsValue() {}

func (v UInt16Value) Copy() Value {
	return v
}
func (UInt16Value) GetOwner() string {
	// value is never owned
	return ""
}

func (UInt16Value) SetOwner(_ string) {
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
func (v UInt16Value) Negate() IntegerValue {
	panic(errors.NewUnreachableError())
}

func (v UInt16Value) Plus(other IntegerValue) IntegerValue {
	sum := v + other.(UInt16Value)
	// INT30-C
	if sum < v {
		panic(OverflowError{})
	}
	return sum
}

func (v UInt16Value) Minus(other IntegerValue) IntegerValue {
	diff := v - other.(UInt16Value)
	// INT30-C
	if diff > v {
		panic(UnderflowError{})
	}
	return diff
}

func (v UInt16Value) Mod(other IntegerValue) IntegerValue {
	o := other.(UInt16Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v UInt16Value) Mul(other IntegerValue) IntegerValue {
	o := other.(UInt16Value)
	if (v > 0) && (o > 0) && (v > (math.MaxUint16 / o)) {
		panic(&OverflowError{})
	}
	return v * o
}

func (v UInt16Value) Div(other IntegerValue) IntegerValue {
	o := other.(UInt16Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v / o
}

func (v UInt16Value) Less(other IntegerValue) BoolValue {
	return v < other.(UInt16Value)
}

func (v UInt16Value) LessEqual(other IntegerValue) BoolValue {
	return v <= other.(UInt16Value)
}

func (v UInt16Value) Greater(other IntegerValue) BoolValue {
	return v > other.(UInt16Value)
}

func (v UInt16Value) GreaterEqual(other IntegerValue) BoolValue {
	return v >= other.(UInt16Value)
}

func (v UInt16Value) Equal(other Value) BoolValue {
	otherUInt16, ok := other.(UInt16Value)
	if !ok {
		return false
	}
	return v == otherUInt16
}

func ConvertUInt16(value Value) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return UInt16Value(value.(IntegerValue).IntValue())
}

// UInt32Value

type UInt32Value uint32

func init() {
	gob.Register(UInt32Value(0))
}

func (UInt32Value) IsValue() {}

func (v UInt32Value) Copy() Value {
	return v
}

func (UInt32Value) GetOwner() string {
	// value is never owned
	return ""
}

func (UInt32Value) SetOwner(_ string) {
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

func (v UInt32Value) Negate() IntegerValue {
	panic(errors.NewUnreachableError())
}

func (v UInt32Value) Plus(other IntegerValue) IntegerValue {
	sum := v + other.(UInt32Value)
	// INT30-C
	if sum < v {
		panic(OverflowError{})
	}
	return sum
}

func (v UInt32Value) Minus(other IntegerValue) IntegerValue {
	diff := v - other.(UInt32Value)
	// INT30-C
	if diff > v {
		panic(UnderflowError{})
	}
	return diff
}

func (v UInt32Value) Mod(other IntegerValue) IntegerValue {
	o := other.(UInt32Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v UInt32Value) Mul(other IntegerValue) IntegerValue {
	o := other.(UInt32Value)
	if (v > 0) && (o > 0) && (v > (math.MaxUint32 / o)) {
		panic(&OverflowError{})
	}
	return v * o
}

func (v UInt32Value) Div(other IntegerValue) IntegerValue {
	o := other.(UInt32Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v / o
}

func (v UInt32Value) Less(other IntegerValue) BoolValue {
	return v < other.(UInt32Value)
}

func (v UInt32Value) LessEqual(other IntegerValue) BoolValue {
	return v <= other.(UInt32Value)
}

func (v UInt32Value) Greater(other IntegerValue) BoolValue {
	return v > other.(UInt32Value)
}

func (v UInt32Value) GreaterEqual(other IntegerValue) BoolValue {
	return v >= other.(UInt32Value)
}

func (v UInt32Value) Equal(other Value) BoolValue {
	otherUInt32, ok := other.(UInt32Value)
	if !ok {
		return false
	}
	return v == otherUInt32
}

func ConvertUInt32(value Value) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return UInt32Value(value.(IntegerValue).IntValue())
}

// UInt64Value

type UInt64Value uint64

func init() {
	gob.Register(UInt64Value(0))
}

func (UInt64Value) IsValue() {}

func (v UInt64Value) Copy() Value {
	return v
}

func (UInt64Value) GetOwner() string {
	// value is never owned
	return ""
}

func (UInt64Value) SetOwner(_ string) {
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

func (v UInt64Value) Negate() IntegerValue {
	panic(errors.NewUnreachableError())
}

func (v UInt64Value) Plus(other IntegerValue) IntegerValue {
	sum := v + other.(UInt64Value)
	// INT30-C
	if sum < v {
		panic(OverflowError{})
	}
	return sum
}

func (v UInt64Value) Minus(other IntegerValue) IntegerValue {
	diff := v - other.(UInt64Value)
	// INT30-C
	if diff > v {
		panic(UnderflowError{})
	}
	return diff
}

func (v UInt64Value) Mod(other IntegerValue) IntegerValue {
	o := other.(UInt64Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v UInt64Value) Mul(other IntegerValue) IntegerValue {
	o := other.(UInt64Value)
	if (v > 0) && (o > 0) && (v > (math.MaxUint64 / o)) {
		panic(&OverflowError{})
	}
	return v * o
}

func (v UInt64Value) Div(other IntegerValue) IntegerValue {
	o := other.(UInt64Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v / o
}

func (v UInt64Value) Less(other IntegerValue) BoolValue {
	return v < other.(UInt64Value)
}

func (v UInt64Value) LessEqual(other IntegerValue) BoolValue {
	return v <= other.(UInt64Value)
}

func (v UInt64Value) Greater(other IntegerValue) BoolValue {
	return v > other.(UInt64Value)
}

func (v UInt64Value) GreaterEqual(other IntegerValue) BoolValue {
	return v >= other.(UInt64Value)
}

func (v UInt64Value) Equal(other Value) BoolValue {
	otherUInt64, ok := other.(UInt64Value)
	if !ok {
		return false
	}
	return v == otherUInt64
}

func ConvertUInt64(value Value) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return UInt64Value(value.(IntegerValue).IntValue())
}

// UInt128Value

type UInt128Value struct {
	int *big.Int
}

func init() {
	gob.Register(UInt128Value{})
}

func (v UInt128Value) IsValue() {}

func (v UInt128Value) Copy() Value {
	return UInt128Value{big.NewInt(0).Set(v.int)}
}

func (UInt128Value) GetOwner() string {
	// value is never owned
	return ""
}

func (UInt128Value) SetOwner(_ string) {
	// NO-OP: value cannot be owned
}

func (v UInt128Value) IntValue() int {
	// TODO: handle overflow
	return int(v.int.Int64())
}

func (v UInt128Value) String() string {
	return v.int.String()
}

func (v UInt128Value) KeyString() string {
	return v.int.String()
}

func (v UInt128Value) Negate() IntegerValue {
	panic(errors.NewUnreachableError())
}

func (v UInt128Value) Plus(other IntegerValue) IntegerValue {
	sum := big.NewInt(0)
	sum.Add(v.int, other.(UInt128Value).int)
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
	if sum.Cmp(sema.UInt128TypeMax) > 0 {
		panic(OverflowError{})
	}
	return UInt128Value{sum}
}

func (v UInt128Value) Minus(other IntegerValue) IntegerValue {
	diff := big.NewInt(0)
	diff.Sub(v.int, other.(UInt128Value).int)
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
	if diff.Cmp(sema.UInt128TypeMin) < 0 {
		panic(UnderflowError{})
	}
	return UInt128Value{diff}
}

func (v UInt128Value) Mod(other IntegerValue) IntegerValue {
	o := other.(UInt128Value)
	res := big.NewInt(0)
	if o.int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Mod(v.int, o.int)
	return UInt128Value{res}
}

func (v UInt128Value) Mul(other IntegerValue) IntegerValue {
	o := other.(UInt128Value)
	res := big.NewInt(0)
	res.Mul(v.int, o.int)
	if res.Cmp(sema.UInt128TypeMax) > 0 {
		panic(OverflowError{})
	}
	return UInt128Value{res}
}

func (v UInt128Value) Div(other IntegerValue) IntegerValue {
	o := other.(UInt128Value)
	res := big.NewInt(0)
	if o.int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Div(v.int, o.int)
	return UInt128Value{res}
}

func (v UInt128Value) Less(other IntegerValue) BoolValue {
	cmp := v.int.Cmp(other.(UInt128Value).int)
	return cmp == -1
}

func (v UInt128Value) LessEqual(other IntegerValue) BoolValue {
	cmp := v.int.Cmp(other.(UInt128Value).int)
	return cmp <= 0
}

func (v UInt128Value) Greater(other IntegerValue) BoolValue {
	cmp := v.int.Cmp(other.(UInt128Value).int)
	return cmp == 1
}

func (v UInt128Value) GreaterEqual(other IntegerValue) BoolValue {
	cmp := v.int.Cmp(other.(UInt128Value).int)
	return cmp >= 0
}

func (v UInt128Value) Equal(other Value) BoolValue {
	otherInt, ok := other.(UInt128Value)
	if !ok {
		return false
	}
	cmp := v.int.Cmp(otherInt.int)
	return cmp == 0
}

func ConvertUInt128(value Value) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	intValue := value.(IntegerValue).IntValue()
	return UInt128Value{big.NewInt(0).SetInt64(int64(intValue))}
}

// UInt256Value

type UInt256Value struct {
	int *big.Int
}

func init() {
	gob.Register(UInt256Value{})
}

func (v UInt256Value) IsValue() {}

func (v UInt256Value) Copy() Value {
	return UInt256Value{big.NewInt(0).Set(v.int)}
}

func (UInt256Value) GetOwner() string {
	// value is never owned
	return ""
}

func (UInt256Value) SetOwner(_ string) {
	// NO-OP: value cannot be owned
}

func (v UInt256Value) IntValue() int {
	// TODO: handle overflow
	return int(v.int.Int64())
}

func (v UInt256Value) String() string {
	return v.int.String()
}

func (v UInt256Value) KeyString() string {
	return v.int.String()
}

func (v UInt256Value) Negate() IntegerValue {
	panic(errors.NewUnreachableError())
}

func (v UInt256Value) Plus(other IntegerValue) IntegerValue {
	sum := big.NewInt(0)
	sum.Add(v.int, other.(UInt256Value).int)
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
	if sum.Cmp(sema.UInt256TypeMax) > 0 {
		panic(OverflowError{})
	}
	return UInt256Value{sum}
}

func (v UInt256Value) Minus(other IntegerValue) IntegerValue {
	diff := big.NewInt(0)
	diff.Sub(v.int, other.(UInt256Value).int)
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
	if diff.Cmp(sema.UInt256TypeMin) < 0 {
		panic(UnderflowError{})
	}
	return UInt256Value{diff}
}

func (v UInt256Value) Mod(other IntegerValue) IntegerValue {
	o := other.(UInt256Value)
	res := big.NewInt(0)
	if o.int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Mod(v.int, o.int)
	return UInt256Value{res}
}

func (v UInt256Value) Mul(other IntegerValue) IntegerValue {
	o := other.(UInt256Value)
	res := big.NewInt(0)
	res.Mul(v.int, o.int)
	if res.Cmp(sema.UInt256TypeMax) > 0 {
		panic(OverflowError{})
	}
	return UInt256Value{res}
}

func (v UInt256Value) Div(other IntegerValue) IntegerValue {
	o := other.(UInt256Value)
	res := big.NewInt(0)
	if o.int.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Div(v.int, o.int)
	return UInt256Value{res}
}

func (v UInt256Value) Less(other IntegerValue) BoolValue {
	cmp := v.int.Cmp(other.(UInt256Value).int)
	return cmp == -1
}

func (v UInt256Value) LessEqual(other IntegerValue) BoolValue {
	cmp := v.int.Cmp(other.(UInt256Value).int)
	return cmp <= 0
}

func (v UInt256Value) Greater(other IntegerValue) BoolValue {
	cmp := v.int.Cmp(other.(UInt256Value).int)
	return cmp == 1
}

func (v UInt256Value) GreaterEqual(other IntegerValue) BoolValue {
	cmp := v.int.Cmp(other.(UInt256Value).int)
	return cmp >= 0
}

func (v UInt256Value) Equal(other Value) BoolValue {
	otherInt, ok := other.(UInt256Value)
	if !ok {
		return false
	}
	cmp := v.int.Cmp(otherInt.int)
	return cmp == 0
}

func ConvertUInt256(value Value) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	intValue := value.(IntegerValue).IntValue()
	return UInt256Value{big.NewInt(0).SetInt64(int64(intValue))}
}

// Word8Value

type Word8Value uint8

func init() {
	gob.Register(Word8Value(0))
}

func (Word8Value) IsValue() {}

func (v Word8Value) Copy() Value {
	return v
}

func (Word8Value) GetOwner() string {
	// value is never owned
	return ""
}

func (Word8Value) SetOwner(_ string) {
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

func (v Word8Value) Negate() IntegerValue {
	panic(errors.NewUnreachableError())
}

func (v Word8Value) Plus(other IntegerValue) IntegerValue {
	return v + other.(Word8Value)
}

func (v Word8Value) Minus(other IntegerValue) IntegerValue {
	return v - other.(Word8Value)
}

func (v Word8Value) Mod(other IntegerValue) IntegerValue {
	o := other.(Word8Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v Word8Value) Mul(other IntegerValue) IntegerValue {
	return v * other.(Word8Value)
}

func (v Word8Value) Div(other IntegerValue) IntegerValue {
	o := other.(Word8Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v / o
}

func (v Word8Value) Less(other IntegerValue) BoolValue {
	return v < other.(Word8Value)
}

func (v Word8Value) LessEqual(other IntegerValue) BoolValue {
	return v <= other.(Word8Value)
}

func (v Word8Value) Greater(other IntegerValue) BoolValue {
	return v > other.(Word8Value)
}

func (v Word8Value) GreaterEqual(other IntegerValue) BoolValue {
	return v >= other.(Word8Value)
}

func (v Word8Value) Equal(other Value) BoolValue {
	otherWord8, ok := other.(Word8Value)
	if !ok {
		return false
	}
	return v == otherWord8
}

func ConvertWord8(value Value) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return Word8Value(value.(IntegerValue).IntValue())
}

// Word16Value

type Word16Value uint16

func init() {
	gob.Register(Word16Value(0))
}

func (Word16Value) IsValue() {}

func (v Word16Value) Copy() Value {
	return v
}
func (Word16Value) GetOwner() string {
	// value is never owned
	return ""
}

func (Word16Value) SetOwner(_ string) {
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
func (v Word16Value) Negate() IntegerValue {
	panic(errors.NewUnreachableError())
}

func (v Word16Value) Plus(other IntegerValue) IntegerValue {
	return v + other.(Word16Value)
}

func (v Word16Value) Minus(other IntegerValue) IntegerValue {
	return v - other.(Word16Value)
}

func (v Word16Value) Mod(other IntegerValue) IntegerValue {
	o := other.(Word16Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v Word16Value) Mul(other IntegerValue) IntegerValue {
	return v * other.(Word16Value)
}

func (v Word16Value) Div(other IntegerValue) IntegerValue {
	o := other.(Word16Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v / o
}

func (v Word16Value) Less(other IntegerValue) BoolValue {
	return v < other.(Word16Value)
}

func (v Word16Value) LessEqual(other IntegerValue) BoolValue {
	return v <= other.(Word16Value)
}

func (v Word16Value) Greater(other IntegerValue) BoolValue {
	return v > other.(Word16Value)
}

func (v Word16Value) GreaterEqual(other IntegerValue) BoolValue {
	return v >= other.(Word16Value)
}

func (v Word16Value) Equal(other Value) BoolValue {
	otherWord16, ok := other.(Word16Value)
	if !ok {
		return false
	}
	return v == otherWord16
}

func ConvertWord16(value Value) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return Word16Value(value.(IntegerValue).IntValue())
}

// Word32Value

type Word32Value uint32

func init() {
	gob.Register(Word32Value(0))
}

func (Word32Value) IsValue() {}

func (v Word32Value) Copy() Value {
	return v
}

func (Word32Value) GetOwner() string {
	// value is never owned
	return ""
}

func (Word32Value) SetOwner(_ string) {
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

func (v Word32Value) Negate() IntegerValue {
	panic(errors.NewUnreachableError())
}

func (v Word32Value) Plus(other IntegerValue) IntegerValue {
	return v + other.(Word32Value)
}

func (v Word32Value) Minus(other IntegerValue) IntegerValue {
	return v - other.(Word32Value)
}

func (v Word32Value) Mod(other IntegerValue) IntegerValue {
	o := other.(Word32Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v Word32Value) Mul(other IntegerValue) IntegerValue {
	return v * other.(Word32Value)
}

func (v Word32Value) Div(other IntegerValue) IntegerValue {
	o := other.(Word32Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v / o
}

func (v Word32Value) Less(other IntegerValue) BoolValue {
	return v < other.(Word32Value)
}

func (v Word32Value) LessEqual(other IntegerValue) BoolValue {
	return v <= other.(Word32Value)
}

func (v Word32Value) Greater(other IntegerValue) BoolValue {
	return v > other.(Word32Value)
}

func (v Word32Value) GreaterEqual(other IntegerValue) BoolValue {
	return v >= other.(Word32Value)
}

func (v Word32Value) Equal(other Value) BoolValue {
	otherWord32, ok := other.(Word32Value)
	if !ok {
		return false
	}
	return v == otherWord32
}

func ConvertWord32(value Value) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return Word32Value(value.(IntegerValue).IntValue())
}

// Word64Value

type Word64Value uint64

func init() {
	gob.Register(Word64Value(0))
}

func (Word64Value) IsValue() {}

func (v Word64Value) Copy() Value {
	return v
}

func (Word64Value) GetOwner() string {
	// value is never owned
	return ""
}

func (Word64Value) SetOwner(_ string) {
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

func (v Word64Value) Negate() IntegerValue {
	panic(errors.NewUnreachableError())
}

func (v Word64Value) Plus(other IntegerValue) IntegerValue {
	return v + other.(Word64Value)
}

func (v Word64Value) Minus(other IntegerValue) IntegerValue {
	return v - other.(Word64Value)
}

func (v Word64Value) Mod(other IntegerValue) IntegerValue {
	o := other.(Word64Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v % o
}

func (v Word64Value) Mul(other IntegerValue) IntegerValue {
	return v * other.(Word64Value)
}

func (v Word64Value) Div(other IntegerValue) IntegerValue {
	o := other.(Word64Value)
	if o == 0 {
		panic(&DivisionByZeroError{})
	}
	return v / o
}

func (v Word64Value) Less(other IntegerValue) BoolValue {
	return v < other.(Word64Value)
}

func (v Word64Value) LessEqual(other IntegerValue) BoolValue {
	return v <= other.(Word64Value)
}

func (v Word64Value) Greater(other IntegerValue) BoolValue {
	return v > other.(Word64Value)
}

func (v Word64Value) GreaterEqual(other IntegerValue) BoolValue {
	return v >= other.(Word64Value)
}

func (v Word64Value) Equal(other Value) BoolValue {
	otherWord64, ok := other.(Word64Value)
	if !ok {
		return false
	}
	return v == otherWord64
}

func ConvertWord64(value Value) Value {
	// TODO: https://github.com/dapperlabs/flow-go/issues/2141
	return Word64Value(value.(IntegerValue).IntValue())
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
	Owner          string
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
		Owner: "",
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

func (v *CompositeValue) GetOwner() string {
	return v.Owner
}

func (v *CompositeValue) SetOwner(owner string) {
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
	Owner   string
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
		Owner: "",
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
		Owner: "",
	}
}

func (v *DictionaryValue) GetOwner() string {
	return v.Owner
}

func (v *DictionaryValue) SetOwner(owner string) {
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

func (NilValue) isOptionalValue() {}

func (v NilValue) Copy() Value {
	return v
}

func (NilValue) GetOwner() string {
	// value is never owned
	return ""
}

func (NilValue) SetOwner(_ string) {
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
	Owner string
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

func (*SomeValue) isOptionalValue() {}

func (v *SomeValue) Copy() Value {
	return &SomeValue{
		Value: v.Value.Copy(),
		// NOTE: new value has no owner
		Owner: "",
	}
}

func (v *SomeValue) GetOwner() string {
	return v.Owner
}

func (v *SomeValue) SetOwner(owner string) {
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

// AnyValue

type AnyValue struct {
	Value Value
	// TODO: don't store
	Type  sema.Type
	Owner string
}

func NewAnyValueOwningNonCopying(value Value, ty sema.Type) *AnyValue {
	return &AnyValue{
		Value: value,
		Type:  ty,
		Owner: value.GetOwner(),
	}
}

func init() {
	gob.Register(&AnyValue{})
}

func (*AnyValue) IsValue() {}

func (v *AnyValue) Copy() Value {
	return &AnyValue{
		Value: v.Value.Copy(),
		Type:  v.Type,
		// NOTE: new value has no owner
		Owner: "",
	}
}

func (v *AnyValue) GetOwner() string {
	return v.Owner
}

func (v *AnyValue) SetOwner(owner string) {
	if v.Owner == owner {
		return
	}

	v.Owner = owner

	v.Value.SetOwner(owner)
}

func (v *AnyValue) String() string {
	return fmt.Sprint(v.Value)
}

// StorageValue

type StorageValue struct {
	Identifier string
}

func (StorageValue) IsValue() {}

func (v StorageValue) Copy() Value {
	return StorageValue{
		Identifier: v.Identifier,
	}
}

func (v StorageValue) GetOwner() string {
	return v.Identifier
}

func (StorageValue) SetOwner(_ string) {
	// NO-OP: ownership cannot be changed
}

// PublishedValue

type PublishedValue struct {
	Identifier string
}

func (PublishedValue) IsValue() {}

func (v PublishedValue) Copy() Value {
	return PublishedValue{
		Identifier: v.Identifier,
	}
}

func (v PublishedValue) GetOwner() string {
	return v.Identifier
}

func (PublishedValue) SetOwner(_ string) {
	// NO-OP: ownership cannot be changed
}

// StorageReferenceValue

type StorageReferenceValue struct {
	TargetStorageIdentifier string
	TargetKey               string
	Owner                   string
}

func init() {
	gob.Register(&StorageReferenceValue{})
}

func (*StorageReferenceValue) IsValue() {}

func (v *StorageReferenceValue) Copy() Value {
	return &StorageReferenceValue{
		TargetStorageIdentifier: v.TargetStorageIdentifier,
		TargetKey:               v.TargetKey,
		// NOTE: new value has no owner
		Owner: "",
	}
}

func (v *StorageReferenceValue) GetOwner() string {
	return v.Owner
}

func (v *StorageReferenceValue) SetOwner(owner string) {
	v.Owner = owner
}

func (v *StorageReferenceValue) referencedValue(interpreter *Interpreter, locationRange LocationRange) Value {
	key := PrefixedStorageKey(v.TargetKey, AccessLevelPrivate)

	switch referenced := interpreter.readStored(v.TargetStorageIdentifier, key).(type) {
	case *SomeValue:
		return referenced.Value
	case NilValue:
		panic(&DereferenceError{
			LocationRange: locationRange,
		})
	default:
		panic(errors.NewUnreachableError())
	}
}

func (v *StorageReferenceValue) GetMember(interpreter *Interpreter, locationRange LocationRange, name string) Value {
	return v.referencedValue(interpreter, locationRange).(MemberAccessibleValue).
		GetMember(interpreter, locationRange, name)
}

func (v *StorageReferenceValue) SetMember(interpreter *Interpreter, locationRange LocationRange, name string, value Value) {
	v.referencedValue(interpreter, locationRange).(MemberAccessibleValue).
		SetMember(interpreter, locationRange, name, value)
}

func (v *StorageReferenceValue) Get(interpreter *Interpreter, locationRange LocationRange, key Value) Value {
	return v.referencedValue(interpreter, locationRange).(ValueIndexableValue).
		Get(interpreter, locationRange, key)
}

func (v *StorageReferenceValue) Set(interpreter *Interpreter, locationRange LocationRange, key Value, value Value) {
	v.referencedValue(interpreter, locationRange).(ValueIndexableValue).
		Set(interpreter, locationRange, key, value)
}

func (v *StorageReferenceValue) Equal(other Value) BoolValue {
	otherReference, ok := other.(*StorageReferenceValue)
	if !ok {
		return false
	}

	return v.TargetStorageIdentifier == otherReference.TargetStorageIdentifier &&
		v.TargetKey == otherReference.TargetKey
}

// EphemeralReferenceValue

type EphemeralReferenceValue struct {
	Value Value
}

func (*EphemeralReferenceValue) IsValue() {}

func (v *EphemeralReferenceValue) Copy() Value {
	return v
}

func (v *EphemeralReferenceValue) GetOwner() string {
	// value is never owned
	return ""
}

func (v *EphemeralReferenceValue) SetOwner(owner string) {
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

	return otherReference.Value == v.Value
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

func ConvertAddress(value Value) Value {
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
			uint64(value.(IntegerValue).IntValue()),
		)
	}
	return result
}

func (AddressValue) IsValue() {}

func (v AddressValue) Copy() Value {
	return v
}

func (v AddressValue) String() string {
	return fmt.Sprintf("%x", [common.AddressLength]byte(v))
}

func (AddressValue) GetOwner() string {
	// value is never owned
	return ""
}

func (AddressValue) SetOwner(_ string) {
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

func NewAccountValue(address AddressValue, setCode, addPublicKey, removePublicKey FunctionValue) *CompositeValue {
	storageIdentifier := address.Hex()

	return &CompositeValue{
		Kind:   common.CompositeKindStructure,
		TypeID: (&sema.AccountType{}).ID(),
		InjectedFields: map[string]Value{
			"address":         address,
			"storage":         StorageValue{Identifier: storageIdentifier},
			"published":       PublishedValue{Identifier: storageIdentifier},
			"setCode":         setCode,
			"addPublicKey":    addPublicKey,
			"removePublicKey": removePublicKey,
		},
	}
}

// PublicAccountValue

func NewPublicAccountValue(address AddressValue) *CompositeValue {
	storageIdentifier := address.Hex()

	return &CompositeValue{
		Kind:   common.CompositeKindStructure,
		TypeID: (&sema.PublicAccountType{}).ID(),
		InjectedFields: map[string]Value{
			"address":   address,
			"published": PublishedValue{Identifier: storageIdentifier},
		},
	}
}
