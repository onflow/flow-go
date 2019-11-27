package interpreter

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/rivo/uniseg"
	"golang.org/x/text/unicode/norm"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/trampoline"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
)

type Value interface {
	isValue()
	Copy() Value
}

type ExportableValue interface {
	Export() values.Value
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
	Equal(other Value) BoolValue
}

// DestroyableValue

type DestroyableValue interface {
	Destroy(*Interpreter, LocationPosition) trampoline.Trampoline
}

// VoidValue

type VoidValue struct{}

func init() {
	gob.Register(VoidValue{})
}

func (VoidValue) isValue() {}

func (v VoidValue) Copy() Value {
	return v
}

func (v VoidValue) Export() values.Value {
	return values.Void{}
}

func (v VoidValue) String() string {
	return "()"
}

// BoolValue

type BoolValue bool

func init() {
	gob.Register(BoolValue(true))
}

func (BoolValue) isValue() {}

func (v BoolValue) Copy() Value {
	return v
}

func (v BoolValue) Export() values.Value {
	return values.Bool(v)
}

func (v BoolValue) Negate() BoolValue {
	return !v
}

func (v BoolValue) Equal(other Value) BoolValue {
	return bool(v) == bool(other.(BoolValue))
}

func (v BoolValue) String() string {
	return strconv.FormatBool(bool(v))
}

func (v BoolValue) KeyString() string {
	return v.String()
}

// StringValue

type StringValue struct {
	Str *string
}

func init() {
	gob.Register(StringValue{})
}

func NewStringValue(str string) StringValue {
	return StringValue{&str}
}

func (StringValue) isValue() {}

func (v StringValue) Copy() Value {
	return v
}

func (v StringValue) Export() values.Value {
	return values.String(v.StrValue())
}

func (v StringValue) String() string {
	// TODO: quote like in string literal
	return strconv.Quote(v.StrValue())
}

func (v StringValue) StrValue() string {
	return *v.Str
}

func (v StringValue) KeyString() string {
	return v.StrValue()
}

func (v StringValue) Equal(other Value) BoolValue {
	otherString := other.(StringValue)
	return norm.NFC.String(v.StrValue()) == norm.NFC.String(otherString.StrValue())
}

func (v StringValue) Concat(other ConcatenatableValue) Value {
	otherString := other.(StringValue)

	var sb strings.Builder

	sb.WriteString(v.StrValue())
	sb.WriteString(otherString.StrValue())

	return NewStringValue(sb.String())
}

func (v StringValue) Slice(from IntValue, to IntValue) Value {
	fromInt := from.IntValue()
	toInt := to.IntValue()
	str := v.StrValue()
	return NewStringValue(str[fromInt:toInt])
}

func (v StringValue) Get(_ *Interpreter, _ LocationRange, key Value) Value {
	i := key.(IntegerValue).IntValue()

	// TODO: optimize grapheme clusters to prevent unnecessary iteration
	graphemes := uniseg.NewGraphemes(v.StrValue())
	graphemes.Next()

	for j := 0; j < i; j++ {
		graphemes.Next()
	}

	char := graphemes.Str()

	return NewStringValue(char)
}

func (v StringValue) Set(_ *Interpreter, _ LocationRange, key Value, value Value) {
	i := key.(IntegerValue).IntValue()
	char := value.(StringValue).StrValue()

	str := v.StrValue()

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

	*v.Str = sb.String()
}

func (v StringValue) GetMember(interpreter *Interpreter, _ LocationRange, name string) Value {
	switch name {
	case "length":
		count := uniseg.GraphemeClusterCount(v.StrValue())
		return NewIntValue(int64(count))
	case "concat":
		return NewHostFunctionValue(
			func(arguments []Value, location LocationPosition) trampoline.Trampoline {
				otherValue := arguments[0].(ConcatenatableValue)
				result := v.Concat(otherValue)
				return trampoline.Done{Result: result}
			},
		)
	case "slice":
		return NewHostFunctionValue(
			func(arguments []Value, location LocationPosition) trampoline.Trampoline {
				from := arguments[0].(IntValue)
				to := arguments[1].(IntValue)
				result := v.Slice(from, to)
				return trampoline.Done{Result: result}
			},
		)
	default:
		panic(errors.NewUnreachableError())
	}
}

func (v StringValue) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// ArrayValue

type ArrayValue struct {
	Values *[]Value
}

func init() {
	gob.Register(ArrayValue{})
}

func NewArrayValue(values ...Value) ArrayValue {
	return ArrayValue{
		Values: &values,
	}
}

func (ArrayValue) isValue() {}

func (v ArrayValue) Copy() Value {
	// TODO: optimize, use copy-on-write
	copies := make([]Value, len(*v.Values))
	for i, value := range *v.Values {
		copies[i] = value.Copy()
	}
	return NewArrayValue(copies...)
}

func (v ArrayValue) Destroy(interpreter *Interpreter, location LocationPosition) trampoline.Trampoline {
	var result trampoline.Trampoline = trampoline.Done{}
	for _, value := range *v.Values {
		result = result.FlatMap(func(_ interface{}) trampoline.Trampoline {
			return value.(DestroyableValue).Destroy(interpreter, location)
		})
	}
	return result
}

func (v ArrayValue) Export() values.Value {
	// TODO: how to export constant-sized array?
	arrayVal := make(values.VariableSizedArray, len(*v.Values))

	for i, value := range *v.Values {
		arrayVal[i] = value.(ExportableValue).Export()
	}

	return arrayVal
}

func (v ArrayValue) GobEncode() ([]byte, error) {
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
		v.Values = new([]Value)
	}
	return nil
}

func (v ArrayValue) Concat(other ConcatenatableValue) Value {
	otherArray := other.(ArrayValue)
	values := append(*v.Values, *otherArray.Values...)
	return NewArrayValue(values...)
}

func (v ArrayValue) Get(_ *Interpreter, _ LocationRange, key Value) Value {
	return (*v.Values)[key.(IntegerValue).IntValue()]
}

func (v ArrayValue) Set(_ *Interpreter, _ LocationRange, key Value, value Value) {
	(*v.Values)[key.(IntegerValue).IntValue()] = value
}

func (v ArrayValue) String() string {
	var builder strings.Builder
	builder.WriteString("[")
	for i, value := range *v.Values {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(fmt.Sprint(value))
	}
	builder.WriteString("]")
	return builder.String()
}

func (v ArrayValue) Append(x Value) {
	*v.Values = append(*v.Values, x)
}

func (v ArrayValue) Insert(i int, x Value) {
	values := *v.Values
	*v.Values = append(values[:i], append([]Value{x}, values[i:]...)...)
}

func (v ArrayValue) Remove(i int) Value {
	values := *v.Values
	result := values[i]
	lastIndex := len(values) - 1

	copy(values[i:], values[i+1:])

	// avoid memory leaks by explicitly setting value to nil
	values[lastIndex] = nil

	*v.Values = values[:lastIndex]

	return result
}

func (v ArrayValue) RemoveFirst() Value {
	values := *v.Values
	var x Value

	x, *v.Values = values[0], values[1:]

	return x
}

func (v ArrayValue) RemoveLast() Value {
	values := *v.Values
	var x Value

	lastIndex := len(values) - 1
	x, *v.Values = values[lastIndex], values[:lastIndex]

	return x
}

func (v ArrayValue) Contains(x Value) BoolValue {
	y := x.(EquatableValue)

	for _, z := range *v.Values {
		if y.Equal(z) {
			return BoolValue(true)
		}
	}

	return BoolValue(false)
}

func (v ArrayValue) GetMember(interpreter *Interpreter, _ LocationRange, name string) Value {
	switch name {
	case "length":
		return NewIntValue(int64(v.Count()))

	case "append":
		return NewHostFunctionValue(
			func(arguments []Value, location LocationPosition) trampoline.Trampoline {
				v.Append(arguments[0])
				return trampoline.Done{Result: VoidValue{}}
			},
		)

	case "concat":
		return NewHostFunctionValue(
			func(arguments []Value, location LocationPosition) trampoline.Trampoline {
				x := arguments[0].(ConcatenatableValue)
				result := v.Concat(x)
				return trampoline.Done{Result: result}
			},
		)

	case "insert":
		return NewHostFunctionValue(
			func(arguments []Value, location LocationPosition) trampoline.Trampoline {
				i := arguments[0].(IntegerValue).IntValue()
				x := arguments[1]
				v.Insert(i, x)
				return trampoline.Done{Result: VoidValue{}}
			},
		)

	case "remove":
		return NewHostFunctionValue(
			func(arguments []Value, location LocationPosition) trampoline.Trampoline {
				i := arguments[0].(IntegerValue).IntValue()
				result := v.Remove(i)
				return trampoline.Done{Result: result}
			},
		)

	case "removeFirst":
		return NewHostFunctionValue(
			func(arguments []Value, location LocationPosition) trampoline.Trampoline {
				result := v.RemoveFirst()
				return trampoline.Done{Result: result}
			},
		)

	case "removeLast":
		return NewHostFunctionValue(
			func(arguments []Value, location LocationPosition) trampoline.Trampoline {
				result := v.RemoveLast()
				return trampoline.Done{Result: result}
			},
		)

	case "contains":
		return NewHostFunctionValue(
			func(arguments []Value, location LocationPosition) trampoline.Trampoline {
				result := v.Contains(arguments[0])
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (v ArrayValue) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

func (v ArrayValue) Count() int {
	return len(*v.Values)
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
	if intValue, ok := value.(IntValue); ok {
		return intValue.Copy()
	}
	return NewIntValue(int64(value.(IntegerValue).IntValue()))
}

func (v IntValue) isValue() {}

func (v IntValue) Copy() Value {
	return IntValue{big.NewInt(0).Set(v.Int)}
}

func (v IntValue) Export() values.Value {
	return values.NewIntFromBig(big.NewInt(0).Set(v.Int))
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
	newValue := big.NewInt(0).Add(v.Int, other.(IntValue).Int)
	return IntValue{newValue}
}

func (v IntValue) Minus(other IntegerValue) IntegerValue {
	newValue := big.NewInt(0).Sub(v.Int, other.(IntValue).Int)
	return IntValue{newValue}
}

func (v IntValue) Mod(other IntegerValue) IntegerValue {
	newValue := big.NewInt(0).Mod(v.Int, other.(IntValue).Int)
	return IntValue{newValue}
}

func (v IntValue) Mul(other IntegerValue) IntegerValue {
	newValue := big.NewInt(0).Mul(v.Int, other.(IntValue).Int)
	return IntValue{newValue}
}

func (v IntValue) Div(other IntegerValue) IntegerValue {
	newValue := big.NewInt(0).Div(v.Int, other.(IntValue).Int)
	return IntValue{newValue}
}

func (v IntValue) Less(other IntegerValue) BoolValue {
	cmp := v.Int.Cmp(other.(IntValue).Int)
	return BoolValue(cmp == -1)
}

func (v IntValue) LessEqual(other IntegerValue) BoolValue {
	cmp := v.Int.Cmp(other.(IntValue).Int)
	return BoolValue(cmp <= 0)
}

func (v IntValue) Greater(other IntegerValue) BoolValue {
	cmp := v.Int.Cmp(other.(IntValue).Int)
	return BoolValue(cmp == 1)
}

func (v IntValue) GreaterEqual(other IntegerValue) BoolValue {
	cmp := v.Int.Cmp(other.(IntValue).Int)
	return BoolValue(cmp >= 0)
}

func (v IntValue) Equal(other Value) BoolValue {
	cmp := v.Int.Cmp(other.(IntValue).Int)
	return BoolValue(cmp == 0)
}

// Int8Value

type Int8Value int8

func init() {
	gob.Register(Int8Value(0))
}

func (Int8Value) isValue() {}

func (v Int8Value) Copy() Value {
	return v
}

func (v Int8Value) String() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int8Value) KeyString() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int8Value) Export() values.Value {
	return values.Int8(v)
}

func (v Int8Value) IntValue() int {
	return int(v)
}

func (v Int8Value) Negate() IntegerValue {
	return -v
}

func (v Int8Value) Plus(other IntegerValue) IntegerValue {
	return v + other.(Int8Value)
}

func (v Int8Value) Minus(other IntegerValue) IntegerValue {
	return v - other.(Int8Value)
}

func (v Int8Value) Mod(other IntegerValue) IntegerValue {
	return v % other.(Int8Value)
}

func (v Int8Value) Mul(other IntegerValue) IntegerValue {
	return v * other.(Int8Value)
}

func (v Int8Value) Div(other IntegerValue) IntegerValue {
	return v / other.(Int8Value)
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
	return v == other.(Int8Value)
}

func ConvertInt8(value Value) Value {
	return Int8Value(value.(IntegerValue).IntValue())
}

// Int16Value

type Int16Value int16

func init() {
	gob.Register(Int16Value(0))
}

func (Int16Value) isValue() {}

func (v Int16Value) Copy() Value {
	return v
}

func (v Int16Value) String() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int16Value) KeyString() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int16Value) Export() values.Int16 {
	return values.Int16(v)
}

func (v Int16Value) IntValue() int {
	return int(v)
}

func (v Int16Value) Negate() IntegerValue {
	return -v
}

func (v Int16Value) Plus(other IntegerValue) IntegerValue {
	return v + other.(Int16Value)
}

func (v Int16Value) Minus(other IntegerValue) IntegerValue {
	return v - other.(Int16Value)
}

func (v Int16Value) Mod(other IntegerValue) IntegerValue {
	return v % other.(Int16Value)
}

func (v Int16Value) Mul(other IntegerValue) IntegerValue {
	return v * other.(Int16Value)
}

func (v Int16Value) Div(other IntegerValue) IntegerValue {
	return v / other.(Int16Value)
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
	return v == other.(Int16Value)
}

func ConvertInt16(value Value) Value {
	return Int16Value(value.(IntegerValue).IntValue())
}

// Int32Value

type Int32Value int32

func init() {
	gob.Register(Int32Value(0))
}

func (Int32Value) isValue() {}

func (v Int32Value) Copy() Value {
	return v
}

func (v Int32Value) String() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int32Value) KeyString() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int32Value) Export() values.Value {
	return values.Int32(v)
}

func (v Int32Value) IntValue() int {
	return int(v)
}

func (v Int32Value) Negate() IntegerValue {
	return -v
}

func (v Int32Value) Plus(other IntegerValue) IntegerValue {
	return v + other.(Int32Value)
}

func (v Int32Value) Minus(other IntegerValue) IntegerValue {
	return v - other.(Int32Value)
}

func (v Int32Value) Mod(other IntegerValue) IntegerValue {
	return v % other.(Int32Value)
}

func (v Int32Value) Mul(other IntegerValue) IntegerValue {
	return v * other.(Int32Value)
}

func (v Int32Value) Div(other IntegerValue) IntegerValue {
	return v / other.(Int32Value)
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
	return v == other.(Int32Value)
}

func ConvertInt32(value Value) Value {
	return Int32Value(value.(IntegerValue).IntValue())
}

// Int64Value

type Int64Value int64

func init() {
	gob.Register(Int64Value(0))
}

func (Int64Value) isValue() {}

func (v Int64Value) Copy() Value {
	return v
}

func (v Int64Value) String() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int64Value) KeyString() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int64Value) Export() values.Value {
	return values.Int64(v)
}

func (v Int64Value) IntValue() int {
	return int(v)
}

func (v Int64Value) Negate() IntegerValue {
	return -v
}

func (v Int64Value) Plus(other IntegerValue) IntegerValue {
	return v + other.(Int64Value)
}

func (v Int64Value) Minus(other IntegerValue) IntegerValue {
	return v - other.(Int64Value)
}

func (v Int64Value) Mod(other IntegerValue) IntegerValue {
	return v % other.(Int64Value)
}

func (v Int64Value) Mul(other IntegerValue) IntegerValue {
	return v * other.(Int64Value)
}

func (v Int64Value) Div(other IntegerValue) IntegerValue {
	return v / other.(Int64Value)
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
	return v == other.(Int64Value)
}

func ConvertInt64(value Value) Value {
	return Int64Value(value.(IntegerValue).IntValue())
}

// UInt8Value

type UInt8Value uint8

func init() {
	gob.Register(UInt8Value(0))
}

func (UInt8Value) isValue() {}

func (v UInt8Value) Copy() Value {
	return v
}

func (v UInt8Value) String() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt8Value) KeyString() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt8Value) Export() values.Value {
	return values.Uint8(v)
}

func (v UInt8Value) IntValue() int {
	return int(v)
}

func (v UInt8Value) Negate() IntegerValue {
	return -v
}

func (v UInt8Value) Plus(other IntegerValue) IntegerValue {
	return v + other.(UInt8Value)
}

func (v UInt8Value) Minus(other IntegerValue) IntegerValue {
	return v - other.(UInt8Value)
}

func (v UInt8Value) Mod(other IntegerValue) IntegerValue {
	return v % other.(UInt8Value)
}

func (v UInt8Value) Mul(other IntegerValue) IntegerValue {
	return v * other.(UInt8Value)
}

func (v UInt8Value) Div(other IntegerValue) IntegerValue {
	return v / other.(UInt8Value)
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
	return v == other.(UInt8Value)
}

func ConvertUInt8(value Value) Value {
	return UInt8Value(value.(IntegerValue).IntValue())
}

// UInt16Value

type UInt16Value uint16

func init() {
	gob.Register(UInt16Value(0))
}

func (UInt16Value) isValue() {}

func (v UInt16Value) Copy() Value {
	return v
}

func (v UInt16Value) String() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt16Value) KeyString() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt16Value) Export() values.Value {
	return values.Uint16(v)
}

func (v UInt16Value) IntValue() int {
	return int(v)
}
func (v UInt16Value) Negate() IntegerValue {
	return -v
}

func (v UInt16Value) Plus(other IntegerValue) IntegerValue {
	return v + other.(UInt16Value)
}

func (v UInt16Value) Minus(other IntegerValue) IntegerValue {
	return v - other.(UInt16Value)
}

func (v UInt16Value) Mod(other IntegerValue) IntegerValue {
	return v % other.(UInt16Value)
}

func (v UInt16Value) Mul(other IntegerValue) IntegerValue {
	return v * other.(UInt16Value)
}

func (v UInt16Value) Div(other IntegerValue) IntegerValue {
	return v / other.(UInt16Value)
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
	return v == other.(UInt16Value)
}

func ConvertUInt16(value Value) Value {
	return UInt16Value(value.(IntegerValue).IntValue())
}

// UInt32Value

type UInt32Value uint32

func init() {
	gob.Register(UInt32Value(0))
}

func (UInt32Value) isValue() {}

func (v UInt32Value) Copy() Value {
	return v
}

func (v UInt32Value) String() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt32Value) KeyString() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt32Value) Export() values.Value {
	return values.Uint32(v)
}

func (v UInt32Value) IntValue() int {
	return int(v)
}

func (v UInt32Value) Negate() IntegerValue {
	return -v
}

func (v UInt32Value) Plus(other IntegerValue) IntegerValue {
	return v + other.(UInt32Value)
}

func (v UInt32Value) Minus(other IntegerValue) IntegerValue {
	return v - other.(UInt32Value)
}

func (v UInt32Value) Mod(other IntegerValue) IntegerValue {
	return v % other.(UInt32Value)
}

func (v UInt32Value) Mul(other IntegerValue) IntegerValue {
	return v * other.(UInt32Value)
}

func (v UInt32Value) Div(other IntegerValue) IntegerValue {
	return v / other.(UInt32Value)
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
	return v == other.(UInt32Value)
}

func ConvertUInt32(value Value) Value {
	return UInt32Value(value.(IntegerValue).IntValue())
}

// UInt64Value

type UInt64Value uint64

func init() {
	gob.Register(UInt64Value(0))
}

func (UInt64Value) isValue() {}

func (v UInt64Value) Copy() Value {
	return v
}

func (v UInt64Value) String() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt64Value) KeyString() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt64Value) Export() values.Value {
	return values.Uint64(v)
}

func (v UInt64Value) IntValue() int {
	return int(v)
}

func (v UInt64Value) Negate() IntegerValue {
	return -v
}

func (v UInt64Value) Plus(other IntegerValue) IntegerValue {
	return v + other.(UInt64Value)
}

func (v UInt64Value) Minus(other IntegerValue) IntegerValue {
	return v - other.(UInt64Value)
}

func (v UInt64Value) Mod(other IntegerValue) IntegerValue {
	return v % other.(UInt64Value)
}

func (v UInt64Value) Mul(other IntegerValue) IntegerValue {
	return v * other.(UInt64Value)
}

func (v UInt64Value) Div(other IntegerValue) IntegerValue {
	return v / other.(UInt64Value)
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
	return v == other.(UInt64Value)
}

func ConvertUInt64(value Value) Value {
	return UInt64Value(value.(IntegerValue).IntValue())
}

// CompositeValue

type CompositeValue struct {
	Location   ast.Location
	Identifier string
	Fields     *map[string]Value
	Functions  *map[string]FunctionValue
	Destructor *InterpretedFunctionValue
}

func init() {
	gob.Register(CompositeValue{})
}

func (v CompositeValue) Destroy(interpreter *Interpreter, location LocationPosition) trampoline.Trampoline {
	// if composite was deserialized, dynamically link in the destructor
	if v.Destructor == nil {
		v.Destructor = interpreter.DestructorFunctions[v.Identifier]
	}

	destructor := v.Destructor
	if destructor == nil {
		return trampoline.Done{Result: VoidValue{}}
	}

	return interpreter.bindSelf(*destructor, v).
		invoke(nil, location)
}

func (CompositeValue) isValue() {}

func (v CompositeValue) Copy() Value {
	newFields := make(map[string]Value, len(*v.Fields))
	for field, value := range *v.Fields {
		newFields[field] = value.Copy()
	}

	// NOTE: not copying functions or destructor – they are linked in

	return CompositeValue{
		Location:   v.Location,
		Identifier: v.Identifier,
		Fields:     &newFields,
		Functions:  v.Functions,
		Destructor: v.Destructor,
	}
}

func (v CompositeValue) Export() values.Value {
	fieldMap := *v.Fields

	fields := make([]values.Value, 0, len(fieldMap))

	for _, value := range fieldMap {
		fields = append(fields, value.(ExportableValue).Export())
	}

	return values.Composite{fields}
}

func (v CompositeValue) GetMember(interpreter *Interpreter, _ LocationRange, name string) Value {
	value, ok := (*v.Fields)[name]
	if ok {
		return value
	}

	// get correct interpreter
	if v.Location != nil {
		subInterpreter, ok := interpreter.SubInterpreters[v.Location.ID()]
		if ok {
			interpreter = subInterpreter
		}
	}

	// if composite was deserialized, dynamically link in the functions
	if v.Functions == nil {
		functions := interpreter.CompositeFunctions[v.Identifier]
		v.Functions = &functions
	}

	function, ok := (*v.Functions)[name]
	if ok {
		if interpretedFunction, ok := function.(InterpretedFunctionValue); ok {
			function = interpreter.bindSelf(interpretedFunction, v)
		}
		return function
	}

	return nil
}

func (v CompositeValue) SetMember(interpreter *Interpreter, locationRange LocationRange, name string, value Value) {
	(*v.Fields)[name] = value
}

func (v CompositeValue) GobEncode() ([]byte, error) {
	w := new(bytes.Buffer)
	encoder := gob.NewEncoder(w)
	// NOTE: important: decode as pointer, so gob sees
	// the interface, not the concrete type
	err := encoder.Encode(&v.Location)
	if err != nil {
		return nil, err
	}
	err = encoder.Encode(v.Identifier)
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
	err = decoder.Decode(&v.Identifier)
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

func (v CompositeValue) String() string {
	var builder strings.Builder
	builder.WriteString(v.Identifier)
	builder.WriteString("(")
	i := 0
	for name, value := range *v.Fields {
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

func (v CompositeValue) GetField(name string) Value {
	return (*v.Fields)[name]
}

// DictionaryValue

type DictionaryValue struct {
	Keys    ArrayValue
	Entries map[string]Value
}

func NewDictionaryValue(keysAndValues ...Value) DictionaryValue {
	keysAndValuesCount := len(keysAndValues)
	if keysAndValuesCount%2 != 0 {
		panic("uneven number of keys and values")
	}

	result := DictionaryValue{
		Keys:    NewArrayValue(),
		Entries: make(map[string]Value, keysAndValuesCount/2),
	}

	for i := 0; i < keysAndValuesCount; i += 2 {
		result.Insert(keysAndValues[i], keysAndValues[i+1])
	}

	return result
}

func init() {
	gob.Register(DictionaryValue{})
}

func (DictionaryValue) isValue() {}

func (v DictionaryValue) Copy() Value {
	newKeys := v.Keys.Copy().(ArrayValue)

	newEntries := make(map[string]Value, len(v.Entries))
	for name, value := range v.Entries {
		newEntries[name] = value.Copy()
	}

	return DictionaryValue{
		Keys:    newKeys,
		Entries: newEntries,
	}
}

func (v DictionaryValue) Destroy(interpreter *Interpreter, location LocationPosition) trampoline.Trampoline {
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

	for _, keyValue := range *v.Keys.Values {
		maybeDestroy(keyValue)
	}

	for _, value := range v.Entries {
		maybeDestroy(value)
	}

	return result
}

func (v DictionaryValue) Export() values.Value {
	d := make(values.Dictionary, v.Count())

	for i, keyValue := range *v.Keys.Values {
		key := dictionaryKey(keyValue)
		value := v.Entries[key]

		exportedKey := keyValue.(ExportableValue).Export()
		exportedValue := value.(ExportableValue).Export()

		d[i] = values.KeyValuePair{
			Key:   exportedKey,
			Value: exportedValue,
		}
	}

	return d
}

func (v DictionaryValue) Get(_ *Interpreter, _ LocationRange, keyValue Value) Value {
	value, ok := v.Entries[dictionaryKey(keyValue)]
	if !ok {
		return NilValue{}
	}
	return SomeValue{Value: value}
}

func dictionaryKey(keyValue Value) string {
	hasKeyString, ok := keyValue.(HasKeyString)
	if !ok {
		panic(errors.NewUnreachableError())
	}
	return hasKeyString.KeyString()
}

type HasKeyString interface {
	KeyString() string
}

func (v DictionaryValue) Set(_ *Interpreter, _ LocationRange, keyValue Value, value Value) {
	switch typedValue := value.(type) {
	case SomeValue:
		v.Insert(keyValue, typedValue.Value)

	case NilValue:
		v.Remove(keyValue)
		return

	default:
		panic(errors.NewUnreachableError())
	}
}

func (v DictionaryValue) String() string {
	var builder strings.Builder
	builder.WriteString("{")
	i := 0
	for _, keyValue := range *v.Keys.Values {
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

func (v DictionaryValue) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {
	case "length":
		return NewIntValue(int64(v.Count()))

	case "keys":
		return v.Keys.Copy()

	case "values":
		values := make([]Value, v.Count())
		i := 0
		for _, keyValue := range *v.Keys.Values {
			key := dictionaryKey(keyValue)
			values[i] = v.Entries[key]
			i++
		}
		return NewArrayValue(values...)

	case "remove":
		return NewHostFunctionValue(
			func(arguments []Value, location LocationPosition) trampoline.Trampoline {
				keyValue := arguments[0]

				existingValue := v.Remove(keyValue)

				var returnValue Value
				if existingValue == nil {
					returnValue = NilValue{}
				} else {
					returnValue = SomeValue{Value: existingValue}
				}

				return trampoline.Done{
					Result: returnValue,
				}
			},
		)

	case "insert":
		return NewHostFunctionValue(
			func(arguments []Value, location LocationPosition) trampoline.Trampoline {
				keyValue := arguments[0]
				newValue := arguments[1]

				existingValue := v.Insert(keyValue, newValue)

				var returnValue Value
				if existingValue == nil {
					returnValue = NilValue{}
				} else {
					returnValue = SomeValue{Value: existingValue}
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

func (v DictionaryValue) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	// Dictionaries have no settable members (fields / functions)
	panic(errors.NewUnreachableError())
}

func (v DictionaryValue) Count() int {
	return v.Keys.Count()
}

func (v DictionaryValue) Remove(keyValue Value) (existingValue Value) {
	key := dictionaryKey(keyValue)
	existingValue, exists := v.Entries[key]

	if !exists {
		return nil
	}

	delete(v.Entries, key)

	// TODO: optimize linear scan
	for i, keyValue := range *v.Keys.Values {
		if dictionaryKey(keyValue) == key {
			v.Keys.Remove(i)
			return existingValue
		}
	}

	panic(errors.NewUnreachableError())
}

func (v DictionaryValue) Insert(keyValue Value, value Value) (existingValue Value) {
	key := dictionaryKey(keyValue)
	existingValue, existed := v.Entries[key]

	if !existed {
		v.Keys.Append(keyValue)
	}
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

// EventValue

type EventValue struct {
	Identifier string
	Fields     []EventField
	Location   ast.Location
}

func (EventValue) isValue() {}

func (v EventValue) Export() values.Value {
	fields := make([]values.Value, len(v.Fields))

	for i, field := range v.Fields {
		fields[i] = field.Value.(ExportableValue).Export()
	}

	return values.Event{
		Identifier: v.Identifier,
		Fields:     fields,
	}
}

func (v EventValue) Copy() Value {
	fields := make([]EventField, len(v.Fields))
	for i, field := range v.Fields {
		fields[i] = EventField{
			Identifier: field.Identifier,
			Value:      field.Value.Copy(),
		}
	}

	return EventValue{
		Identifier: v.Identifier,
		Fields:     fields,
	}
}

func (v EventValue) String() string {
	var fields strings.Builder
	for i, field := range v.Fields {
		if i > 0 {
			fields.WriteString(", ")
		}
		fields.WriteString(field.String())
	}

	return fmt.Sprintf("%s(%s)", v.Identifier, fields.String())
}

// EventField

type EventField struct {
	Identifier string
	Value      Value
}

func (f EventField) String() string {
	return fmt.Sprintf("%s: %s", f.Identifier, f.Value)
}

// ToValue

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
	var values []Value
	for _, argument := range inputs {
		value, ok := argument.(Value)
		if !ok {
			var err error
			value, err = ToValue(argument)
			if err != nil {
				return nil, err
			}
		}
		values = append(
			values,
			value,
		)
	}
	return values, nil
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

func (NilValue) isValue() {}

func (NilValue) isOptionalValue() {}

func (v NilValue) Copy() Value {
	return v
}

func (v NilValue) Destroy(_ *Interpreter, _ LocationPosition) trampoline.Trampoline {
	return trampoline.Done{}
}

func (NilValue) String() string {
	return "nil"
}

func (v NilValue) Export() values.Value {
	return values.Nil{}
}

// SomeValue

type SomeValue struct {
	Value Value
}

func init() {
	gob.Register(SomeValue{})
}

func (SomeValue) isValue() {}

func (SomeValue) isOptionalValue() {}

func (v SomeValue) Copy() Value {
	return SomeValue{
		Value: v.Value.Copy(),
	}
}

func (v SomeValue) Destroy(interpreter *Interpreter, location LocationPosition) trampoline.Trampoline {
	return v.Value.(DestroyableValue).Destroy(interpreter, location)
}

func (v SomeValue) String() string {
	return fmt.Sprint(v.Value)
}

// AnyValue

type AnyValue struct {
	Value Value
	// TODO: don't store
	Type sema.Type
}

func init() {
	gob.Register(AnyValue{})
}

func (AnyValue) isValue() {}

func (v AnyValue) Copy() Value {
	return AnyValue{
		Value: v.Value.Copy(),
		Type:  v.Type,
	}
}

func (v AnyValue) String() string {
	return fmt.Sprint(v.Value)
}

// StorageValue

type StorageValue struct {
	Identifier string
}

func (StorageValue) isValue() {}

func (v StorageValue) Copy() Value {
	return StorageValue{
		Identifier: v.Identifier,
	}
}

// ReferenceValue

type ReferenceValue struct {
	StorageIdentifier string
	Key               string
}

func init() {
	gob.Register(ReferenceValue{})
}

func (ReferenceValue) isValue() {}

func (v ReferenceValue) Copy() Value {
	return v
}

func (v ReferenceValue) referencedValue(interpreter *Interpreter, locationRange LocationRange) Value {
	switch referenced :=
		interpreter.readStored(v.StorageIdentifier, v.Key).(type) {
	case SomeValue:
		return referenced.Value
	case NilValue:
		panic(&DereferenceError{
			LocationRange: locationRange,
		})
	default:
		panic(errors.NewUnreachableError())
	}
}

func (v ReferenceValue) GetMember(interpreter *Interpreter, locationRange LocationRange, name string) Value {
	return v.referencedValue(interpreter, locationRange).(MemberAccessibleValue).
		GetMember(interpreter, locationRange, name)
}

func (v ReferenceValue) SetMember(interpreter *Interpreter, locationRange LocationRange, name string, value Value) {
	v.referencedValue(interpreter, locationRange).(MemberAccessibleValue).
		SetMember(interpreter, locationRange, name, value)
}

func (v ReferenceValue) Get(interpreter *Interpreter, locationRange LocationRange, key Value) Value {
	return v.referencedValue(interpreter, locationRange).(ValueIndexableValue).
		Get(interpreter, locationRange, key)
}

func (v ReferenceValue) Set(interpreter *Interpreter, locationRange LocationRange, key Value, value Value) {
	v.referencedValue(interpreter, locationRange).(ValueIndexableValue).
		Set(interpreter, locationRange, key, value)
}

// AddressValue

const AddressLength = 20

type AddressValue [AddressLength]byte

func init() {
	gob.Register(AddressValue{})
}

func ConvertAddress(value Value) Value {
	result := AddressValue{}
	if intValue, ok := value.(IntValue); ok {
		bigEndianBytes := intValue.Int.Bytes()
		copy(
			result[AddressLength-len(bigEndianBytes):AddressLength],
			bigEndianBytes,
		)
	} else {
		binary.BigEndian.PutUint64(
			result[AddressLength-8:AddressLength],
			uint64(value.(IntegerValue).IntValue()),
		)
	}
	return result
}

func (AddressValue) isValue() {}

func (v AddressValue) Export() values.Value {
	return values.Address(v)
}

func (v AddressValue) Copy() Value {
	return v
}

func (v AddressValue) String() string {
	return fmt.Sprintf("%x", [AddressLength]byte(v))
}

// AccountValue

type AccountValue = CompositeValue

func NewAccountValue(address AddressValue) AccountValue {
	addressHex := fmt.Sprintf("%x", address)

	return AccountValue{
		Identifier: (&sema.AccountType{}).ID(),
		Fields: &map[string]Value{
			"address": address,
			"storage": StorageValue{Identifier: addressHex},
		},
	}
}
