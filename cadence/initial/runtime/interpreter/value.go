package interpreter

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/rivo/uniseg"
	"golang.org/x/text/unicode/norm"

	"example.com/cadence-initial/runtime/ast"
	"example.com/cadence-initial/runtime/common"
	"example.com/cadence-initial/runtime/errors"
	"example.com/cadence-initial/runtime/sema"
	"example.com/cadence-initial/runtime/trampoline"
)

// Value

type Value interface {
	IsValue()
	DynamicType(interpreter *Interpreter) DynamicType
	Copy() Value
	GetOwner() *common.Address
	SetOwner(address *common.Address)
	IsModified() bool
	SetModified(modified bool)
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
	Destroy(interpreter *Interpreter, locationRange LocationRange) trampoline.Trampoline
}

// HasKeyString

type HasKeyString interface {
	KeyString() string
}

// TypeValue

type TypeValue struct {
	Type sema.Type
}

func (TypeValue) IsValue() {}

func (TypeValue) DynamicType(_ *Interpreter) DynamicType {
	return MetaTypeDynamicType{}
}

func (v TypeValue) Copy() Value {
	return v
}

func (TypeValue) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (TypeValue) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (TypeValue) IsModified() bool {
	return false
}

func (TypeValue) SetModified(_ bool) {
	// NO-OP
}

func (v TypeValue) String() string {
	return fmt.Sprintf("Type<%s>", v.Type)
}

func (v TypeValue) Equal(other Value) BoolValue {
	otherMetaType, ok := other.(TypeValue)
	if !ok {
		return false
	}
	return BoolValue(v.Type.Equal(otherMetaType.Type))
}

func (v TypeValue) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {
	case "identifier":
		return NewStringValue(v.Type.QualifiedString())
	default:
		panic(errors.NewUnreachableError())
	}
}

func (v TypeValue) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// VoidValue

type VoidValue struct{}

func (VoidValue) IsValue() {}

func (VoidValue) DynamicType(_ *Interpreter) DynamicType {
	return VoidDynamicType{}
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

func (VoidValue) IsModified() bool {
	return false
}

func (VoidValue) SetModified(_ bool) {
	// NO-OP
}

func (VoidValue) String() string {
	return "()"
}

// BoolValue

type BoolValue bool

func (BoolValue) IsValue() {}

func (BoolValue) DynamicType(_ *Interpreter) DynamicType {
	return BoolDynamicType{}
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

func (BoolValue) IsModified() bool {
	return false
}

func (BoolValue) SetModified(_ bool) {
	// NO-OP
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
	Str      string
	modified bool
}

func NewStringValue(str string) *StringValue {
	return &StringValue{
		Str:      str,
		modified: true,
	}
}

func (*StringValue) IsValue() {}

func (*StringValue) DynamicType(_ *Interpreter) DynamicType {
	return StringDynamicType{}
}

func (v *StringValue) Copy() Value {
	return NewStringValue(v.Str)
}

func (*StringValue) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (*StringValue) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (v *StringValue) IsModified() bool {
	return v.modified
}

func (v *StringValue) SetModified(modified bool) {
	v.modified = modified
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
	fromInt := from.ToInt()
	toInt := to.ToInt()
	return NewStringValue(v.Str[fromInt:toInt])
}

func (v *StringValue) Get(_ *Interpreter, _ LocationRange, key Value) Value {
	i := key.(NumberValue).ToInt()

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
	index := key.(NumberValue).ToInt()
	char := value.(*StringValue)
	v.SetIndex(index, char)
}

func (v *StringValue) SetIndex(index int, char *StringValue) {
	v.modified = true

	str := v.Str

	// TODO: optimize grapheme clusters to prevent unnecessary iteration
	graphemes := uniseg.NewGraphemes(str)
	graphemes.Next()

	for j := 0; j < index; j++ {
		graphemes.Next()
	}

	start, end := graphemes.Positions()

	var sb strings.Builder

	sb.WriteString(str[:start])
	sb.WriteString(char.Str)
	sb.WriteString(str[end:])

	v.Str = sb.String()
}

func (v *StringValue) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {
	case "length":
		count := uniseg.GraphemeClusterCount(v.Str)
		return NewIntValueFromInt64(int64(count))

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
					values[i] = NewIntValueFromInt64(int64(b))
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
	Values   []Value
	Owner    *common.Address
	modified bool
}

func NewArrayValueUnownedNonCopying(values ...Value) *ArrayValue {
	// NOTE: new value has no owner

	for _, value := range values {
		value.SetOwner(nil)
	}

	if values == nil {
		values = make([]Value, 0)
	}

	return &ArrayValue{
		Values:   values,
		modified: true,
		Owner:    nil,
	}
}

func (*ArrayValue) IsValue() {}

func (v *ArrayValue) DynamicType(interpreter *Interpreter) DynamicType {
	elementTypes := make([]DynamicType, len(v.Values))

	for i, value := range v.Values {
		elementTypes[i] = value.DynamicType(interpreter)
	}

	return ArrayDynamicType{
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

func (v *ArrayValue) IsModified() bool {
	if v.modified {
		return true
	}

	for _, value := range v.Values {
		if value.IsModified() {
			return true
		}
	}

	return false
}

func (v *ArrayValue) SetModified(modified bool) {
	v.modified = modified
}

func (v *ArrayValue) Destroy(interpreter *Interpreter, locationRange LocationRange) trampoline.Trampoline {
	var result trampoline.Trampoline = trampoline.Done{}
	for _, value := range v.Values {
		capturedValue := value
		result = result.FlatMap(func(_ interface{}) trampoline.Trampoline {
			return capturedValue.(DestroyableValue).Destroy(interpreter, locationRange)
		})
	}
	return result
}

func (v *ArrayValue) Concat(other ConcatenatableValue) Value {
	otherArray := other.(*ArrayValue)
	concatenated := append(v.Copy().(*ArrayValue).Values, otherArray.Values...)
	return NewArrayValueUnownedNonCopying(concatenated...)
}

func (v *ArrayValue) Get(_ *Interpreter, _ LocationRange, key Value) Value {
	integerKey := key.(NumberValue).ToInt()
	return v.Values[integerKey]
}

func (v *ArrayValue) Set(_ *Interpreter, _ LocationRange, key Value, value Value) {
	index := key.(NumberValue).ToInt()
	v.SetIndex(index, value)
}

func (v *ArrayValue) SetIndex(index int, value Value) {
	v.modified = true
	value.SetOwner(v.Owner)
	v.Values[index] = value
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
	v.modified = true

	element.SetOwner(v.Owner)
	v.Values = append(v.Values, element)
}

func (v *ArrayValue) Insert(i int, element Value) {
	v.modified = true

	element.SetOwner(v.Owner)
	v.Values = append(v.Values[:i], append([]Value{element}, v.Values[i:]...)...)
}

// TODO: unset owner?
func (v *ArrayValue) Remove(i int) Value {
	v.modified = true

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
	v.modified = true

	var firstElement Value
	firstElement, v.Values = v.Values[0], v.Values[1:]
	return firstElement
}

// TODO: unset owner?
func (v *ArrayValue) RemoveLast() Value {
	v.modified = true

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
		return NewIntValueFromInt64(int64(v.Count()))

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
				i := invocation.Arguments[0].(NumberValue).ToInt()
				element := invocation.Arguments[1]
				v.Insert(i, element)
				return trampoline.Done{Result: VoidValue{}}
			},
		)

	case "remove":
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				i := invocation.Arguments[0].(NumberValue).ToInt()
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
	ToInt() int
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

type IntegerValue interface {
	NumberValue
	BitwiseOr(other IntegerValue) IntegerValue
	BitwiseXor(other IntegerValue) IntegerValue
	BitwiseAnd(other IntegerValue) IntegerValue
	BitwiseLeftShift(other IntegerValue) IntegerValue
	BitwiseRightShift(other IntegerValue) IntegerValue
}

// BigNumberValue.
// Implemented by values with an integer value outside the range of int64

type BigNumberValue interface {
	NumberValue
	ToBigInt() *big.Int
}

// Int

type IntValue struct {
	BigInt *big.Int
}

func NewIntValueFromInt64(value int64) IntValue {
	return NewIntValueFromBigInt(big.NewInt(value))
}

func NewIntValueFromBigInt(value *big.Int) IntValue {
	return IntValue{BigInt: value}
}

func ConvertInt(value Value, _ *Interpreter) Value {
	switch value := value.(type) {
	case BigNumberValue:
		return NewIntValueFromBigInt(value.ToBigInt())

	case NumberValue:
		// NOTE: safe, UInt64Value is handled by BigNumberValue above
		return NewIntValueFromInt64(int64(value.ToInt()))

	default:
		panic(errors.NewUnreachableError())
	}
}

func (v IntValue) IsValue() {}

func (IntValue) DynamicType(_ *Interpreter) DynamicType {
	return NumberDynamicType{&sema.IntType{}}
}

func (v IntValue) Copy() Value {
	return IntValue{new(big.Int).Set(v.BigInt)}
}

func (IntValue) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (IntValue) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (IntValue) IsModified() bool {
	return false
}

func (IntValue) SetModified(_ bool) {
	// NO-OP
}

func (v IntValue) ToInt() int {
	// TODO: handle overflow
	return int(v.BigInt.Int64())
}

func (v IntValue) ToBigInt() *big.Int {
	return new(big.Int).Set(v.BigInt)
}

func (v IntValue) String() string {
	return v.BigInt.String()
}

func (v IntValue) KeyString() string {
	return v.BigInt.String()
}

func (v IntValue) Negate() NumberValue {
	return NewIntValueFromBigInt(new(big.Int).Neg(v.BigInt))
}

func (v IntValue) Plus(other NumberValue) NumberValue {
	o := other.(IntValue)
	res := new(big.Int)
	res.Add(v.BigInt, o.BigInt)
	return IntValue{res}
}

func (v IntValue) Minus(other NumberValue) NumberValue {
	o := other.(IntValue)
	res := new(big.Int)
	res.Sub(v.BigInt, o.BigInt)
	return IntValue{res}
}

func (v IntValue) Mod(other NumberValue) NumberValue {
	o := other.(IntValue)
	res := new(big.Int)
	// INT33-C
	if o.BigInt.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Rem(v.BigInt, o.BigInt)
	return IntValue{res}
}

func (v IntValue) Mul(other NumberValue) NumberValue {
	o := other.(IntValue)
	res := new(big.Int)
	res.Mul(v.BigInt, o.BigInt)
	return IntValue{res}
}

func (v IntValue) Div(other NumberValue) NumberValue {
	o := other.(IntValue)
	res := new(big.Int)
	// INT33-C
	if o.BigInt.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Div(v.BigInt, o.BigInt)
	return IntValue{res}
}

func (v IntValue) Less(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(IntValue).BigInt)
	return cmp == -1
}

func (v IntValue) LessEqual(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(IntValue).BigInt)
	return cmp <= 0
}

func (v IntValue) Greater(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(IntValue).BigInt)
	return cmp == 1
}

func (v IntValue) GreaterEqual(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(IntValue).BigInt)
	return cmp >= 0
}

func (v IntValue) Equal(other Value) BoolValue {
	otherInt, ok := other.(IntValue)
	if !ok {
		return false
	}
	cmp := v.BigInt.Cmp(otherInt.BigInt)
	return cmp == 0
}

func (v IntValue) BitwiseOr(other IntegerValue) IntegerValue {
	o := other.(IntValue)
	res := new(big.Int)
	res.Or(v.BigInt, o.BigInt)
	return IntValue{res}
}

func (v IntValue) BitwiseXor(other IntegerValue) IntegerValue {
	o := other.(IntValue)
	res := new(big.Int)
	res.Xor(v.BigInt, o.BigInt)
	return IntValue{res}
}

func (v IntValue) BitwiseAnd(other IntegerValue) IntegerValue {
	o := other.(IntValue)
	res := new(big.Int)
	res.And(v.BigInt, o.BigInt)
	return IntValue{res}
}

func (v IntValue) BitwiseLeftShift(other IntegerValue) IntegerValue {
	o := other.(IntValue)
	res := new(big.Int)
	if o.BigInt.Sign() < 0 {
		panic(UnderflowError{})
	}
	if !o.BigInt.IsUint64() {
		panic(OverflowError{})
	}
	res.Lsh(v.BigInt, uint(o.BigInt.Uint64()))
	return IntValue{res}
}

func (v IntValue) BitwiseRightShift(other IntegerValue) IntegerValue {
	o := other.(IntValue)
	res := new(big.Int)
	if o.BigInt.Sign() < 0 {
		panic(UnderflowError{})
	}
	if !o.BigInt.IsUint64() {
		panic(OverflowError{})
	}
	res.Rsh(v.BigInt, uint(o.BigInt.Uint64()))
	return IntValue{res}
}

func (v IntValue) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {

	case sema.ToStringFunctionName:
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := NewStringValue(v.String())
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (IntValue) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// Int8Value

type Int8Value int8

func (Int8Value) IsValue() {}

func (Int8Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberDynamicType{&sema.Int8Type{}}
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

func (Int8Value) IsModified() bool {
	return false
}

func (Int8Value) SetModified(_ bool) {
	// NO-OP
}

func (v Int8Value) String() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int8Value) KeyString() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int8Value) ToInt() int {
	return int(v)
}

func (v Int8Value) Negate() NumberValue {
	// INT32-C
	if v == math.MinInt8 {
		panic(OverflowError{})
	}
	return -v
}

func (v Int8Value) Plus(other NumberValue) NumberValue {
	o := other.(Int8Value)
	// INT32-C
	if (o > 0) && (v > (math.MaxInt8 - o)) {
		panic(OverflowError{})
	} else if (o < 0) && (v < (math.MinInt8 - o)) {
		panic(UnderflowError{})
	}
	return v + o
}

func (v Int8Value) Minus(other NumberValue) NumberValue {
	o := other.(Int8Value)
	// INT32-C
	if (o > 0) && (v < (math.MinInt8 + o)) {
		panic(OverflowError{})
	} else if (o < 0) && (v > (math.MaxInt8 + o)) {
		panic(UnderflowError{})
	}
	return v - o
}

func (v Int8Value) Mod(other NumberValue) NumberValue {
	o := other.(Int8Value)
	// INT33-C
	if o == 0 {
		panic(DivisionByZeroError{})
	}
	return v % o
}

func (v Int8Value) Mul(other NumberValue) NumberValue {
	o := other.(Int8Value)
	// INT32-C
	if v > 0 {
		if o > 0 {
			if v > (math.MaxInt8 / o) {
				panic(OverflowError{})
			}
		} else {
			if o < (math.MinInt8 / v) {
				panic(OverflowError{})
			}
		}
	} else {
		if o > 0 {
			if v < (math.MinInt8 / o) {
				panic(OverflowError{})
			}
		} else {
			if (v != 0) && (o < (math.MaxInt8 / v)) {
				panic(OverflowError{})
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
	var res int8

	switch value := value.(type) {
	case BigNumberValue:
		v := value.ToBigInt()
		if v.Cmp(sema.Int8TypeMaxInt) > 0 {
			panic(OverflowError{})
		} else if v.Cmp(sema.Int8TypeMinInt) < 0 {
			panic(UnderflowError{})
		}
		res = int8(v.Int64())

	case NumberValue:
		v := value.ToInt()
		if v > math.MaxInt8 {
			panic(OverflowError{})
		} else if v < math.MinInt8 {
			panic(UnderflowError{})
		}
		res = int8(v)

	default:
		panic(errors.NewUnreachableError())
	}

	return Int8Value(res)
}

func (v Int8Value) BitwiseOr(other IntegerValue) IntegerValue {
	o := other.(Int8Value)
	return v | o
}

func (v Int8Value) BitwiseXor(other IntegerValue) IntegerValue {
	o := other.(Int8Value)
	return v ^ o
}

func (v Int8Value) BitwiseAnd(other IntegerValue) IntegerValue {
	o := other.(Int8Value)
	return v & o
}

func (v Int8Value) BitwiseLeftShift(other IntegerValue) IntegerValue {
	o := other.(Int8Value)
	return v << o
}

func (v Int8Value) BitwiseRightShift(other IntegerValue) IntegerValue {
	o := other.(Int8Value)
	return v >> o
}

func (v Int8Value) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {

	case sema.ToStringFunctionName:
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := NewStringValue(v.String())
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (Int8Value) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// Int16Value

type Int16Value int16

func (Int16Value) IsValue() {}

func (Int16Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberDynamicType{&sema.Int16Type{}}
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

func (Int16Value) IsModified() bool {
	return false
}

func (Int16Value) SetModified(_ bool) {
	// NO-OP
}

func (v Int16Value) String() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int16Value) KeyString() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int16Value) ToInt() int {
	return int(v)
}

func (v Int16Value) Negate() NumberValue {
	// INT32-C
	if v == math.MinInt16 {
		panic(OverflowError{})
	}
	return -v
}

func (v Int16Value) Plus(other NumberValue) NumberValue {
	o := other.(Int16Value)
	// INT32-C
	if (o > 0) && (v > (math.MaxInt16 - o)) {
		panic(OverflowError{})
	} else if (o < 0) && (v < (math.MinInt16 - o)) {
		panic(UnderflowError{})
	}
	return v + o
}

func (v Int16Value) Minus(other NumberValue) NumberValue {
	o := other.(Int16Value)
	// INT32-C
	if (o > 0) && (v < (math.MinInt16 + o)) {
		panic(OverflowError{})
	} else if (o < 0) && (v > (math.MaxInt16 + o)) {
		panic(UnderflowError{})
	}
	return v - o
}

func (v Int16Value) Mod(other NumberValue) NumberValue {
	o := other.(Int16Value)
	// INT33-C
	if o == 0 {
		panic(DivisionByZeroError{})
	}
	return v % o
}

func (v Int16Value) Mul(other NumberValue) NumberValue {
	o := other.(Int16Value)
	// INT32-C
	if v > 0 {
		if o > 0 {
			if v > (math.MaxInt16 / o) {
				panic(OverflowError{})
			}
		} else {
			if o < (math.MinInt16 / v) {
				panic(OverflowError{})
			}
		}
	} else {
		if o > 0 {
			if v < (math.MinInt16 / o) {
				panic(OverflowError{})
			}
		} else {
			if (v != 0) && (o < (math.MaxInt16 / v)) {
				panic(OverflowError{})
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
	var res int16

	switch value := value.(type) {
	case BigNumberValue:
		v := value.ToBigInt()
		if v.Cmp(sema.Int16TypeMaxInt) > 0 {
			panic(OverflowError{})
		} else if v.Cmp(sema.Int16TypeMinInt) < 0 {
			panic(UnderflowError{})
		}
		res = int16(v.Int64())

	case NumberValue:
		v := value.ToInt()
		if v > math.MaxInt16 {
			panic(OverflowError{})
		} else if v < math.MinInt16 {
			panic(UnderflowError{})
		}
		res = int16(v)

	default:
		panic(errors.NewUnreachableError())
	}

	return Int16Value(res)
}

func (v Int16Value) BitwiseOr(other IntegerValue) IntegerValue {
	o := other.(Int16Value)
	return v | o
}

func (v Int16Value) BitwiseXor(other IntegerValue) IntegerValue {
	o := other.(Int16Value)
	return v ^ o
}

func (v Int16Value) BitwiseAnd(other IntegerValue) IntegerValue {
	o := other.(Int16Value)
	return v & o
}

func (v Int16Value) BitwiseLeftShift(other IntegerValue) IntegerValue {
	o := other.(Int16Value)
	return v << o
}

func (v Int16Value) BitwiseRightShift(other IntegerValue) IntegerValue {
	o := other.(Int16Value)
	return v >> o
}

func (v Int16Value) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {

	case sema.ToStringFunctionName:
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := NewStringValue(v.String())
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (Int16Value) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// Int32Value

type Int32Value int32

func (Int32Value) IsValue() {}

func (Int32Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberDynamicType{&sema.Int32Type{}}
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

func (Int32Value) IsModified() bool {
	return false
}

func (Int32Value) SetModified(_ bool) {
	// NO-OP
}

func (v Int32Value) String() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int32Value) KeyString() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int32Value) ToInt() int {
	return int(v)
}

func (v Int32Value) Negate() NumberValue {
	// INT32-C
	if v == math.MinInt32 {
		panic(OverflowError{})
	}
	return -v
}

func (v Int32Value) Plus(other NumberValue) NumberValue {
	o := other.(Int32Value)
	// INT32-C
	if (o > 0) && (v > (math.MaxInt32 - o)) {
		panic(OverflowError{})
	} else if (o < 0) && (v < (math.MinInt32 - o)) {
		panic(UnderflowError{})
	}
	return v + o
}

func (v Int32Value) Minus(other NumberValue) NumberValue {
	o := other.(Int32Value)
	// INT32-C
	if (o > 0) && (v < (math.MinInt32 + o)) {
		panic(OverflowError{})
	} else if (o < 0) && (v > (math.MaxInt32 + o)) {
		panic(UnderflowError{})
	}
	return v - o
}

func (v Int32Value) Mod(other NumberValue) NumberValue {
	o := other.(Int32Value)
	// INT33-C
	if o == 0 {
		panic(DivisionByZeroError{})
	}
	return v % o
}

func (v Int32Value) Mul(other NumberValue) NumberValue {
	o := other.(Int32Value)
	// INT32-C
	if v > 0 {
		if o > 0 {
			if v > (math.MaxInt32 / o) {
				panic(OverflowError{})
			}
		} else {
			if o < (math.MinInt32 / v) {
				panic(OverflowError{})
			}
		}
	} else {
		if o > 0 {
			if v < (math.MinInt32 / o) {
				panic(OverflowError{})
			}
		} else {
			if (v != 0) && (o < (math.MaxInt32 / v)) {
				panic(OverflowError{})
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
	var res int32

	switch value := value.(type) {
	case BigNumberValue:
		v := value.ToBigInt()
		if v.Cmp(sema.Int32TypeMaxInt) > 0 {
			panic(OverflowError{})
		} else if v.Cmp(sema.Int32TypeMinInt) < 0 {
			panic(UnderflowError{})
		}
		res = int32(v.Int64())

	case NumberValue:
		v := value.ToInt()
		if v > math.MaxInt32 {
			panic(OverflowError{})
		} else if v < math.MinInt32 {
			panic(UnderflowError{})
		}
		res = int32(v)

	default:
		panic(errors.NewUnreachableError())
	}

	return Int32Value(res)
}

func (v Int32Value) BitwiseOr(other IntegerValue) IntegerValue {
	o := other.(Int32Value)
	return v | o
}

func (v Int32Value) BitwiseXor(other IntegerValue) IntegerValue {
	o := other.(Int32Value)
	return v ^ o
}

func (v Int32Value) BitwiseAnd(other IntegerValue) IntegerValue {
	o := other.(Int32Value)
	return v & o
}

func (v Int32Value) BitwiseLeftShift(other IntegerValue) IntegerValue {
	o := other.(Int32Value)
	return v << o
}

func (v Int32Value) BitwiseRightShift(other IntegerValue) IntegerValue {
	o := other.(Int32Value)
	return v >> o
}

func (v Int32Value) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {

	case sema.ToStringFunctionName:
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := NewStringValue(v.String())
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (Int32Value) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// Int64Value

type Int64Value int64

func (Int64Value) IsValue() {}

func (Int64Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberDynamicType{&sema.Int64Type{}}
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

func (Int64Value) IsModified() bool {
	return false
}

func (Int64Value) SetModified(_ bool) {
	// NO-OP
}

func (v Int64Value) String() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int64Value) KeyString() string {
	return strconv.FormatInt(int64(v), 10)
}

func (v Int64Value) ToInt() int {
	return int(v)
}

func (v Int64Value) Negate() NumberValue {
	// INT32-C
	if v == math.MinInt64 {
		panic(OverflowError{})
	}
	return -v
}

func safeAddInt64(a, b int64) int64 {
	// INT32-C
	if (b > 0) && (a > (math.MaxInt64 - b)) {
		panic(OverflowError{})
	} else if (b < 0) && (a < (math.MinInt64 - b)) {
		panic(UnderflowError{})
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
		panic(OverflowError{})
	} else if (o < 0) && (v > (math.MaxInt64 + o)) {
		panic(UnderflowError{})
	}
	return v - o
}

func (v Int64Value) Mod(other NumberValue) NumberValue {
	o := other.(Int64Value)
	// INT33-C
	if o == 0 {
		panic(DivisionByZeroError{})
	}
	return v % o
}

func (v Int64Value) Mul(other NumberValue) NumberValue {
	o := other.(Int64Value)
	// INT32-C
	if v > 0 {
		if o > 0 {
			if v > (math.MaxInt64 / o) {
				panic(OverflowError{})
			}
		} else {
			if o < (math.MinInt64 / v) {
				panic(OverflowError{})
			}
		}
	} else {
		if o > 0 {
			if v < (math.MinInt64 / o) {
				panic(OverflowError{})
			}
		} else {
			if (v != 0) && (o < (math.MaxInt64 / v)) {
				panic(OverflowError{})
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
	var res int64

	switch value := value.(type) {
	case BigNumberValue:
		v := value.ToBigInt()
		if v.Cmp(sema.Int64TypeMaxInt) > 0 {
			panic(OverflowError{})
		} else if v.Cmp(sema.Int64TypeMinInt) < 0 {
			panic(UnderflowError{})
		}
		res = v.Int64()

	case NumberValue:
		v := value.ToInt()
		res = int64(v)

	default:
		panic(errors.NewUnreachableError())
	}

	return Int64Value(res)
}

func (v Int64Value) BitwiseOr(other IntegerValue) IntegerValue {
	o := other.(Int64Value)
	return v | o
}

func (v Int64Value) BitwiseXor(other IntegerValue) IntegerValue {
	o := other.(Int64Value)
	return v ^ o
}

func (v Int64Value) BitwiseAnd(other IntegerValue) IntegerValue {
	o := other.(Int64Value)
	return v & o
}

func (v Int64Value) BitwiseLeftShift(other IntegerValue) IntegerValue {
	o := other.(Int64Value)
	return v << o
}

func (v Int64Value) BitwiseRightShift(other IntegerValue) IntegerValue {
	o := other.(Int64Value)
	return v >> o
}

func (v Int64Value) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {

	case sema.ToStringFunctionName:
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := NewStringValue(v.String())
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (Int64Value) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// Int128Value

type Int128Value struct {
	BigInt *big.Int
}

func NewInt128ValueFromInt64(value int64) Int128Value {
	return NewInt128ValueFromBigInt(big.NewInt(value))
}

func NewInt128ValueFromBigInt(value *big.Int) Int128Value {
	return Int128Value{BigInt: value}
}
func (v Int128Value) IsValue() {}

func (Int128Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberDynamicType{&sema.Int128Type{}}
}

func (v Int128Value) Copy() Value {
	return Int128Value{BigInt: new(big.Int).Set(v.BigInt)}
}

func (Int128Value) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (Int128Value) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (Int128Value) IsModified() bool {
	return false
}

func (Int128Value) SetModified(_ bool) {
	// NO-OP
}

func (v Int128Value) ToInt() int {
	// TODO: handle overflow
	return int(v.BigInt.Int64())
}

func (v Int128Value) ToBigInt() *big.Int {
	return new(big.Int).Set(v.BigInt)
}

func (v Int128Value) String() string {
	return v.BigInt.String()
}

func (v Int128Value) KeyString() string {
	return v.BigInt.String()
}

func (v Int128Value) Negate() NumberValue {
	// INT32-C
	//   if v == Int128TypeMinIntBig {
	//       ...
	//   }
	if v.BigInt.Cmp(sema.Int128TypeMinIntBig) == 0 {
		panic(OverflowError{})
	}
	return Int128Value{new(big.Int).Neg(v.BigInt)}
}

func (v Int128Value) Plus(other NumberValue) NumberValue {
	o := other.(Int128Value)
	// Given that this value is backed by an arbitrary size integer,
	// we can just add and check the range of the result.
	//
	// If Go gains a native int128 type and we switch this value
	// to be based on it, then we need to follow INT32-C:
	//
	//   if (o > 0) && (v > (Int128TypeMaxIntBig - o)) {
	//       ...
	//   } else if (o < 0) && (v < (Int128TypeMinIntBig - o)) {
	//       ...
	//   }
	//
	res := new(big.Int)
	res.Add(v.BigInt, o.BigInt)
	if res.Cmp(sema.Int128TypeMinIntBig) < 0 {
		panic(UnderflowError{})
	} else if res.Cmp(sema.Int128TypeMaxIntBig) > 0 {
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
	//   if (o > 0) && (v < (Int128TypeMinIntBig + o)) {
	// 	     ...
	//   } else if (o < 0) && (v > (Int128TypeMaxIntBig + o)) {
	//       ...
	//   }
	//
	res := new(big.Int)
	res.Sub(v.BigInt, o.BigInt)
	if res.Cmp(sema.Int128TypeMinIntBig) < 0 {
		panic(UnderflowError{})
	} else if res.Cmp(sema.Int128TypeMaxIntBig) > 0 {
		panic(OverflowError{})
	}
	return Int128Value{res}
}

func (v Int128Value) Mod(other NumberValue) NumberValue {
	o := other.(Int128Value)
	res := new(big.Int)
	// INT33-C
	if o.BigInt.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Rem(v.BigInt, o.BigInt)
	return Int128Value{res}
}

func (v Int128Value) Mul(other NumberValue) NumberValue {
	o := other.(Int128Value)
	res := new(big.Int)
	res.Mul(v.BigInt, o.BigInt)
	if res.Cmp(sema.Int128TypeMinIntBig) < 0 {
		panic(UnderflowError{})
	} else if res.Cmp(sema.Int128TypeMaxIntBig) > 0 {
		panic(OverflowError{})
	}
	return Int128Value{res}
}

func (v Int128Value) Div(other NumberValue) NumberValue {
	o := other.(Int128Value)
	res := new(big.Int)
	// INT33-C:
	//   if o == 0 {
	//       ...
	//   } else if (v == Int128TypeMinIntBig) && (o == -1) {
	//       ...
	//   }
	if o.BigInt.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.SetInt64(-1)
	if (v.BigInt.Cmp(sema.Int128TypeMinIntBig) == 0) && (o.BigInt.Cmp(res) == 0) {
		panic(OverflowError{})
	}
	res.Div(v.BigInt, o.BigInt)
	return Int128Value{res}
}

func (v Int128Value) Less(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(Int128Value).BigInt)
	return cmp == -1
}

func (v Int128Value) LessEqual(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(Int128Value).BigInt)
	return cmp <= 0
}

func (v Int128Value) Greater(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(Int128Value).BigInt)
	return cmp == 1
}

func (v Int128Value) GreaterEqual(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(Int128Value).BigInt)
	return cmp >= 0
}

func (v Int128Value) Equal(other Value) BoolValue {
	otherInt, ok := other.(Int128Value)
	if !ok {
		return false
	}
	cmp := v.BigInt.Cmp(otherInt.BigInt)
	return cmp == 0
}

func ConvertInt128(value Value, _ *Interpreter) Value {
	var v *big.Int

	switch value := value.(type) {
	case BigNumberValue:
		v = value.ToBigInt()

	case NumberValue:
		v = big.NewInt(int64(value.ToInt()))

	default:
		panic(errors.NewUnreachableError())
	}

	if v.Cmp(sema.Int128TypeMaxIntBig) > 0 {
		panic(OverflowError{})
	} else if v.Cmp(sema.Int128TypeMinIntBig) < 0 {
		panic(UnderflowError{})
	}

	return NewInt128ValueFromBigInt(v)
}

func (v Int128Value) BitwiseOr(other IntegerValue) IntegerValue {
	o := other.(Int128Value)
	res := new(big.Int)
	res.Or(v.BigInt, o.BigInt)
	return Int128Value{res}
}

func (v Int128Value) BitwiseXor(other IntegerValue) IntegerValue {
	o := other.(Int128Value)
	res := new(big.Int)
	res.Xor(v.BigInt, o.BigInt)
	return Int128Value{res}
}

func (v Int128Value) BitwiseAnd(other IntegerValue) IntegerValue {
	o := other.(Int128Value)
	res := new(big.Int)
	res.And(v.BigInt, o.BigInt)
	return Int128Value{res}
}

func (v Int128Value) BitwiseLeftShift(other IntegerValue) IntegerValue {
	o := other.(Int128Value)
	res := new(big.Int)
	if o.BigInt.Sign() < 0 {
		panic(UnderflowError{})
	}
	if !o.BigInt.IsUint64() {
		panic(OverflowError{})
	}
	res.Lsh(v.BigInt, uint(o.BigInt.Uint64()))
	return Int128Value{res}
}

func (v Int128Value) BitwiseRightShift(other IntegerValue) IntegerValue {
	o := other.(Int128Value)
	res := new(big.Int)
	if o.BigInt.Sign() < 0 {
		panic(UnderflowError{})
	}
	if !o.BigInt.IsUint64() {
		panic(OverflowError{})
	}
	res.Rsh(v.BigInt, uint(o.BigInt.Uint64()))
	return Int128Value{res}
}

func (v Int128Value) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {

	case sema.ToStringFunctionName:
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := NewStringValue(v.String())
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (Int128Value) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// Int256Value

type Int256Value struct {
	BigInt *big.Int
}

func NewInt256ValueFromInt64(value int64) Int256Value {
	return NewInt256ValueFromBigInt(big.NewInt(value))
}

func NewInt256ValueFromBigInt(value *big.Int) Int256Value {
	return Int256Value{BigInt: value}
}

func (v Int256Value) IsValue() {}

func (Int256Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberDynamicType{&sema.Int256Type{}}
}

func (v Int256Value) Copy() Value {
	return Int256Value{new(big.Int).Set(v.BigInt)}
}

func (Int256Value) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (Int256Value) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (Int256Value) IsModified() bool {
	return false
}

func (Int256Value) SetModified(_ bool) {
	// NO-OP
}

func (v Int256Value) ToInt() int {
	// TODO: handle overflow
	return int(v.BigInt.Int64())
}

func (v Int256Value) ToBigInt() *big.Int {
	return new(big.Int).Set(v.BigInt)
}

func (v Int256Value) String() string {
	return v.BigInt.String()
}

func (v Int256Value) KeyString() string {
	return v.BigInt.String()
}

func (v Int256Value) Negate() NumberValue {
	// INT32-C
	//   if v == Int256TypeMinIntBig {
	//       ...
	//   }
	if v.BigInt.Cmp(sema.Int256TypeMinIntBig) == 0 {
		panic(OverflowError{})
	}
	return Int256Value{BigInt: new(big.Int).Neg(v.BigInt)}
}

func (v Int256Value) Plus(other NumberValue) NumberValue {
	o := other.(Int256Value)
	// Given that this value is backed by an arbitrary size integer,
	// we can just add and check the range of the result.
	//
	// If Go gains a native int256 type and we switch this value
	// to be based on it, then we need to follow INT32-C:
	//
	//   if (o > 0) && (v > (Int256TypeMaxIntBig - o)) {
	//       ...
	//   } else if (o < 0) && (v < (Int256TypeMinIntBig - o)) {
	//       ...
	//   }
	//
	res := new(big.Int)
	res.Add(v.BigInt, o.BigInt)
	if res.Cmp(sema.Int256TypeMinIntBig) < 0 {
		panic(UnderflowError{})
	} else if res.Cmp(sema.Int256TypeMaxIntBig) > 0 {
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
	//   if (o > 0) && (v < (Int256TypeMinIntBig + o)) {
	// 	     ...
	//   } else if (o < 0) && (v > (Int256TypeMaxIntBig + o)) {
	//       ...
	//   }
	//
	res := new(big.Int)
	res.Sub(v.BigInt, o.BigInt)
	if res.Cmp(sema.Int256TypeMinIntBig) < 0 {
		panic(UnderflowError{})
	} else if res.Cmp(sema.Int256TypeMaxIntBig) > 0 {
		panic(OverflowError{})
	}
	return Int256Value{res}
}

func (v Int256Value) Mod(other NumberValue) NumberValue {
	o := other.(Int256Value)
	res := new(big.Int)
	// INT33-C
	if o.BigInt.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Rem(v.BigInt, o.BigInt)
	return Int256Value{res}
}

func (v Int256Value) Mul(other NumberValue) NumberValue {
	o := other.(Int256Value)
	res := new(big.Int)
	res.Mul(v.BigInt, o.BigInt)
	if res.Cmp(sema.Int256TypeMinIntBig) < 0 {
		panic(UnderflowError{})
	} else if res.Cmp(sema.Int256TypeMaxIntBig) > 0 {
		panic(OverflowError{})
	}
	return Int256Value{res}
}

func (v Int256Value) Div(other NumberValue) NumberValue {
	o := other.(Int256Value)
	res := new(big.Int)
	// INT33-C:
	//   if o == 0 {
	//       ...
	//   } else if (v == Int256TypeMinIntBig) && (o == -1) {
	//       ...
	//   }
	if o.BigInt.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.SetInt64(-1)
	if (v.BigInt.Cmp(sema.Int256TypeMinIntBig) == 0) && (o.BigInt.Cmp(res) == 0) {
		panic(OverflowError{})
	}
	res.Div(v.BigInt, o.BigInt)
	return Int256Value{res}
}

func (v Int256Value) Less(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(Int256Value).BigInt)
	return cmp == -1
}

func (v Int256Value) LessEqual(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(Int256Value).BigInt)
	return cmp <= 0
}

func (v Int256Value) Greater(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(Int256Value).BigInt)
	return cmp == 1
}

func (v Int256Value) GreaterEqual(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(Int256Value).BigInt)
	return cmp >= 0
}

func (v Int256Value) Equal(other Value) BoolValue {
	otherInt, ok := other.(Int256Value)
	if !ok {
		return false
	}
	cmp := v.BigInt.Cmp(otherInt.BigInt)
	return cmp == 0
}

func ConvertInt256(value Value, _ *Interpreter) Value {
	var v *big.Int

	switch value := value.(type) {
	case BigNumberValue:
		v = value.ToBigInt()

	case NumberValue:
		v = big.NewInt(int64(value.ToInt()))

	default:
		panic(errors.NewUnreachableError())
	}

	if v.Cmp(sema.Int256TypeMaxIntBig) > 0 {
		panic(OverflowError{})
	} else if v.Cmp(sema.Int256TypeMinIntBig) < 0 {
		panic(UnderflowError{})
	}

	return NewInt256ValueFromBigInt(v)
}

func (v Int256Value) BitwiseOr(other IntegerValue) IntegerValue {
	o := other.(Int256Value)
	res := new(big.Int)
	res.Or(v.BigInt, o.BigInt)
	return Int256Value{res}
}

func (v Int256Value) BitwiseXor(other IntegerValue) IntegerValue {
	o := other.(Int256Value)
	res := new(big.Int)
	res.Xor(v.BigInt, o.BigInt)
	return Int256Value{res}
}

func (v Int256Value) BitwiseAnd(other IntegerValue) IntegerValue {
	o := other.(Int256Value)
	res := new(big.Int)
	res.And(v.BigInt, o.BigInt)
	return Int256Value{res}
}

func (v Int256Value) BitwiseLeftShift(other IntegerValue) IntegerValue {
	o := other.(Int256Value)
	res := new(big.Int)
	if o.BigInt.Sign() < 0 {
		panic(UnderflowError{})
	}
	if !o.BigInt.IsUint64() {
		panic(OverflowError{})
	}
	res.Lsh(v.BigInt, uint(o.BigInt.Uint64()))
	return Int256Value{res}
}

func (v Int256Value) BitwiseRightShift(other IntegerValue) IntegerValue {
	o := other.(Int256Value)
	res := new(big.Int)
	if o.BigInt.Sign() < 0 {
		panic(UnderflowError{})
	}
	if !o.BigInt.IsUint64() {
		panic(OverflowError{})
	}
	res.Rsh(v.BigInt, uint(o.BigInt.Uint64()))
	return Int256Value{res}
}

func (v Int256Value) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {

	case sema.ToStringFunctionName:
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := NewStringValue(v.String())
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (Int256Value) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// UIntValue

type UIntValue struct {
	BigInt *big.Int
}

func NewUIntValueFromUint64(value uint64) UIntValue {
	return NewUIntValueFromBigInt(new(big.Int).SetUint64(value))
}

func NewUIntValueFromBigInt(value *big.Int) UIntValue {
	return UIntValue{BigInt: value}
}

func ConvertUInt(value Value, _ *Interpreter) Value {
	switch value := value.(type) {
	case BigNumberValue:
		v := value.ToBigInt()
		if v.Sign() < 0 {
			panic(UnderflowError{})
		}
		return NewUIntValueFromBigInt(value.ToBigInt())

	case NumberValue:
		v := value.ToInt()
		if v < 0 {
			panic(UnderflowError{})
		}
		return NewUIntValueFromUint64(uint64(v))

	default:
		panic(errors.NewUnreachableError())
	}
}

func (v UIntValue) IsValue() {}

func (UIntValue) DynamicType(_ *Interpreter) DynamicType {
	return NumberDynamicType{&sema.UIntType{}}
}

func (v UIntValue) Copy() Value {
	return UIntValue{new(big.Int).Set(v.BigInt)}
}

func (UIntValue) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (UIntValue) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (UIntValue) IsModified() bool {
	return false
}

func (UIntValue) SetModified(_ bool) {
	// NO-OP
}

func (v UIntValue) ToInt() int {
	// TODO: handle overflow
	return int(v.BigInt.Int64())
}

func (v UIntValue) ToBigInt() *big.Int {
	return new(big.Int).Set(v.BigInt)
}

func (v UIntValue) String() string {
	return v.BigInt.String()
}

func (v UIntValue) KeyString() string {
	return v.BigInt.String()
}

func (v UIntValue) Negate() NumberValue {
	panic(errors.NewUnreachableError())
}

func (v UIntValue) Plus(other NumberValue) NumberValue {
	o := other.(UIntValue)
	res := new(big.Int)
	res.Add(v.BigInt, o.BigInt)
	return UIntValue{res}
}

func (v UIntValue) Minus(other NumberValue) NumberValue {
	o := other.(UIntValue)
	res := new(big.Int)
	res.Sub(v.BigInt, o.BigInt)
	if res.Sign() < 0 {
		panic(UnderflowError{})
	}
	return UIntValue{res}
}

func (v UIntValue) Mod(other NumberValue) NumberValue {
	o := other.(UIntValue)
	res := new(big.Int)
	// INT33-C
	if o.BigInt.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Rem(v.BigInt, o.BigInt)
	return UIntValue{res}
}

func (v UIntValue) Mul(other NumberValue) NumberValue {
	o := other.(UIntValue)
	res := new(big.Int)
	res.Mul(v.BigInt, o.BigInt)
	return UIntValue{res}
}

func (v UIntValue) Div(other NumberValue) NumberValue {
	o := other.(UIntValue)
	res := new(big.Int)
	// INT33-C
	if o.BigInt.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Div(v.BigInt, o.BigInt)
	return UIntValue{res}
}

func (v UIntValue) Less(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(UIntValue).BigInt)
	return cmp == -1
}

func (v UIntValue) LessEqual(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(UIntValue).BigInt)
	return cmp <= 0
}

func (v UIntValue) Greater(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(UIntValue).BigInt)
	return cmp == 1
}

func (v UIntValue) GreaterEqual(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(UIntValue).BigInt)
	return cmp >= 0
}

func (v UIntValue) Equal(other Value) BoolValue {
	otherUInt, ok := other.(UIntValue)
	if !ok {
		return false
	}
	cmp := v.BigInt.Cmp(otherUInt.BigInt)
	return cmp == 0
}

func (v UIntValue) BitwiseOr(other IntegerValue) IntegerValue {
	o := other.(UIntValue)
	res := new(big.Int)
	res.Or(v.BigInt, o.BigInt)
	return UIntValue{res}
}

func (v UIntValue) BitwiseXor(other IntegerValue) IntegerValue {
	o := other.(UIntValue)
	res := new(big.Int)
	res.Xor(v.BigInt, o.BigInt)
	return UIntValue{res}
}

func (v UIntValue) BitwiseAnd(other IntegerValue) IntegerValue {
	o := other.(UIntValue)
	res := new(big.Int)
	res.And(v.BigInt, o.BigInt)
	return UIntValue{res}
}

func (v UIntValue) BitwiseLeftShift(other IntegerValue) IntegerValue {
	o := other.(UIntValue)
	res := new(big.Int)
	if o.BigInt.Sign() < 0 {
		panic(UnderflowError{})
	}
	if !o.BigInt.IsUint64() {
		panic(OverflowError{})
	}
	res.Lsh(v.BigInt, uint(o.BigInt.Uint64()))
	return UIntValue{res}
}

func (v UIntValue) BitwiseRightShift(other IntegerValue) IntegerValue {
	o := other.(UIntValue)
	res := new(big.Int)
	if o.BigInt.Sign() < 0 {
		panic(UnderflowError{})
	}
	if !o.BigInt.IsUint64() {
		panic(OverflowError{})
	}
	res.Rsh(v.BigInt, uint(o.BigInt.Uint64()))
	return UIntValue{res}
}

func (v UIntValue) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {

	case sema.ToStringFunctionName:
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := NewStringValue(v.String())
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (UIntValue) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// UInt8Value

type UInt8Value uint8

func (UInt8Value) IsValue() {}

func (UInt8Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberDynamicType{&sema.UInt8Type{}}
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

func (UInt8Value) IsModified() bool {
	return false
}

func (UInt8Value) SetModified(_ bool) {
	// NO-OP
}

func (v UInt8Value) String() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt8Value) KeyString() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt8Value) ToInt() int {
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
		panic(DivisionByZeroError{})
	}
	return v % o
}

func (v UInt8Value) Mul(other NumberValue) NumberValue {
	o := other.(UInt8Value)
	if (v > 0) && (o > 0) && (v > (math.MaxUint8 / o)) {
		panic(OverflowError{})
	}
	return v * o
}

func (v UInt8Value) Div(other NumberValue) NumberValue {
	o := other.(UInt8Value)
	if o == 0 {
		panic(DivisionByZeroError{})
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
	var res uint8

	switch value := value.(type) {
	case BigNumberValue:
		v := value.ToBigInt()
		if v.Cmp(sema.UInt8TypeMaxInt) > 0 {
			panic(OverflowError{})
		} else if v.Sign() < 0 {
			panic(UnderflowError{})
		}
		res = uint8(v.Int64())

	case NumberValue:
		v := value.ToInt()
		if v > math.MaxUint8 {
			panic(OverflowError{})
		} else if v < 0 {
			panic(UnderflowError{})
		}
		res = uint8(v)

	default:
		panic(errors.NewUnreachableError())
	}

	return UInt8Value(res)
}

func (v UInt8Value) BitwiseOr(other IntegerValue) IntegerValue {
	o := other.(UInt8Value)
	return v | o
}

func (v UInt8Value) BitwiseXor(other IntegerValue) IntegerValue {
	o := other.(UInt8Value)
	return v ^ o
}

func (v UInt8Value) BitwiseAnd(other IntegerValue) IntegerValue {
	o := other.(UInt8Value)
	return v & o
}

func (v UInt8Value) BitwiseLeftShift(other IntegerValue) IntegerValue {
	o := other.(UInt8Value)
	return v << o
}

func (v UInt8Value) BitwiseRightShift(other IntegerValue) IntegerValue {
	o := other.(UInt8Value)
	return v >> o
}

func (v UInt8Value) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {

	case sema.ToStringFunctionName:
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := NewStringValue(v.String())
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (UInt8Value) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// UInt16Value

type UInt16Value uint16

func (UInt16Value) IsValue() {}

func (UInt16Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberDynamicType{&sema.UInt16Type{}}
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

func (UInt16Value) IsModified() bool {
	return false
}

func (UInt16Value) SetModified(_ bool) {
	// NO-OP
}

func (v UInt16Value) String() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt16Value) KeyString() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt16Value) ToInt() int {
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
		panic(DivisionByZeroError{})
	}
	return v % o
}

func (v UInt16Value) Mul(other NumberValue) NumberValue {
	o := other.(UInt16Value)
	if (v > 0) && (o > 0) && (v > (math.MaxUint16 / o)) {
		panic(OverflowError{})
	}
	return v * o
}

func (v UInt16Value) Div(other NumberValue) NumberValue {
	o := other.(UInt16Value)
	if o == 0 {
		panic(DivisionByZeroError{})
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
	var res uint16

	switch value := value.(type) {
	case BigNumberValue:
		v := value.ToBigInt()
		if v.Cmp(sema.UInt16TypeMaxInt) > 0 {
			panic(OverflowError{})
		} else if v.Sign() < 0 {
			panic(UnderflowError{})
		}
		res = uint16(v.Int64())

	case NumberValue:
		v := value.ToInt()
		if v > math.MaxUint16 {
			panic(OverflowError{})
		} else if v < 0 {
			panic(UnderflowError{})
		}
		res = uint16(v)

	default:
		panic(errors.NewUnreachableError())
	}

	return UInt16Value(res)
}

func (v UInt16Value) BitwiseOr(other IntegerValue) IntegerValue {
	o := other.(UInt16Value)
	return v | o
}

func (v UInt16Value) BitwiseXor(other IntegerValue) IntegerValue {
	o := other.(UInt16Value)
	return v ^ o
}

func (v UInt16Value) BitwiseAnd(other IntegerValue) IntegerValue {
	o := other.(UInt16Value)
	return v & o
}

func (v UInt16Value) BitwiseLeftShift(other IntegerValue) IntegerValue {
	o := other.(UInt16Value)
	return v << o
}

func (v UInt16Value) BitwiseRightShift(other IntegerValue) IntegerValue {
	o := other.(UInt16Value)
	return v >> o
}

func (v UInt16Value) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {

	case sema.ToStringFunctionName:
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := NewStringValue(v.String())
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (UInt16Value) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// UInt32Value

type UInt32Value uint32

func (UInt32Value) IsValue() {}

func (UInt32Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberDynamicType{&sema.UInt32Type{}}
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

func (UInt32Value) IsModified() bool {
	return false
}

func (UInt32Value) SetModified(_ bool) {
	// NO-OP
}

func (v UInt32Value) String() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt32Value) KeyString() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt32Value) ToInt() int {
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
		panic(DivisionByZeroError{})
	}
	return v % o
}

func (v UInt32Value) Mul(other NumberValue) NumberValue {
	o := other.(UInt32Value)
	if (v > 0) && (o > 0) && (v > (math.MaxUint32 / o)) {
		panic(OverflowError{})
	}
	return v * o
}

func (v UInt32Value) Div(other NumberValue) NumberValue {
	o := other.(UInt32Value)
	if o == 0 {
		panic(DivisionByZeroError{})
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
	var res uint32

	switch value := value.(type) {
	case BigNumberValue:
		v := value.ToBigInt()
		if v.Cmp(sema.UInt32TypeMaxInt) > 0 {
			panic(OverflowError{})
		} else if v.Sign() < 0 {
			panic(UnderflowError{})
		}
		res = uint32(v.Int64())

	case NumberValue:
		v := value.ToInt()
		if v > math.MaxUint32 {
			panic(OverflowError{})
		} else if v < 0 {
			panic(UnderflowError{})
		}
		res = uint32(v)

	default:
		panic(errors.NewUnreachableError())
	}

	return UInt32Value(res)
}

func (v UInt32Value) BitwiseOr(other IntegerValue) IntegerValue {
	o := other.(UInt32Value)
	return v | o
}

func (v UInt32Value) BitwiseXor(other IntegerValue) IntegerValue {
	o := other.(UInt32Value)
	return v ^ o
}

func (v UInt32Value) BitwiseAnd(other IntegerValue) IntegerValue {
	o := other.(UInt32Value)
	return v & o
}

func (v UInt32Value) BitwiseLeftShift(other IntegerValue) IntegerValue {
	o := other.(UInt32Value)
	return v << o
}

func (v UInt32Value) BitwiseRightShift(other IntegerValue) IntegerValue {
	o := other.(UInt32Value)
	return v >> o
}

func (v UInt32Value) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {

	case sema.ToStringFunctionName:
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := NewStringValue(v.String())
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (UInt32Value) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// UInt64Value

type UInt64Value uint64

func (UInt64Value) IsValue() {}

func (UInt64Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberDynamicType{&sema.UInt64Type{}}
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

func (UInt64Value) IsModified() bool {
	return false
}

func (UInt64Value) SetModified(_ bool) {
	// NO-OP
}

func (v UInt64Value) String() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt64Value) KeyString() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v UInt64Value) ToInt() int {
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
		panic(DivisionByZeroError{})
	}
	return v % o
}

func (v UInt64Value) Mul(other NumberValue) NumberValue {
	o := other.(UInt64Value)
	if (v > 0) && (o > 0) && (v > (math.MaxUint64 / o)) {
		panic(OverflowError{})
	}
	return v * o
}

func (v UInt64Value) Div(other NumberValue) NumberValue {
	o := other.(UInt64Value)
	if o == 0 {
		panic(DivisionByZeroError{})
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
	var res uint64

	switch value := value.(type) {
	case BigNumberValue:
		v := value.ToBigInt()
		if v.Cmp(sema.UInt64TypeMaxInt) > 0 {
			panic(OverflowError{})
		} else if v.Sign() < 0 {
			panic(UnderflowError{})
		}
		res = uint64(v.Int64())

	case NumberValue:
		v := value.ToInt()
		if v < 0 {
			panic(UnderflowError{})
		}
		res = uint64(v)

	default:
		panic(errors.NewUnreachableError())
	}

	return UInt64Value(res)
}

func (v UInt64Value) BitwiseOr(other IntegerValue) IntegerValue {
	o := other.(UInt64Value)
	return v | o
}

func (v UInt64Value) BitwiseXor(other IntegerValue) IntegerValue {
	o := other.(UInt64Value)
	return v ^ o
}

func (v UInt64Value) BitwiseAnd(other IntegerValue) IntegerValue {
	o := other.(UInt64Value)
	return v & o
}

func (v UInt64Value) BitwiseLeftShift(other IntegerValue) IntegerValue {
	o := other.(UInt64Value)
	return v << o
}

func (v UInt64Value) BitwiseRightShift(other IntegerValue) IntegerValue {
	o := other.(UInt64Value)
	return v >> o
}

func (v UInt64Value) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {

	case sema.ToStringFunctionName:
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := NewStringValue(v.String())
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (UInt64Value) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// UInt128Value

type UInt128Value struct {
	BigInt *big.Int
}

func NewUInt128ValueFromUint64(value uint64) UInt128Value {
	return NewUInt128ValueFromBigInt(new(big.Int).SetUint64(value))
}

func NewUInt128ValueFromBigInt(value *big.Int) UInt128Value {
	return UInt128Value{BigInt: value}
}

func (v UInt128Value) IsValue() {}

func (UInt128Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberDynamicType{&sema.UInt128Type{}}
}

func (v UInt128Value) Copy() Value {
	return UInt128Value{new(big.Int).Set(v.BigInt)}
}

func (UInt128Value) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (UInt128Value) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (UInt128Value) IsModified() bool {
	return false
}

func (UInt128Value) SetModified(_ bool) {
	// NO-OP
}

func (v UInt128Value) ToInt() int {
	// TODO: handle overflow
	return int(v.BigInt.Int64())
}

func (v UInt128Value) ToBigInt() *big.Int {
	return new(big.Int).Set(v.BigInt)
}

func (v UInt128Value) String() string {
	return v.BigInt.String()
}

func (v UInt128Value) KeyString() string {
	return v.BigInt.String()
}

func (v UInt128Value) Negate() NumberValue {
	panic(errors.NewUnreachableError())
}

func (v UInt128Value) Plus(other NumberValue) NumberValue {
	sum := new(big.Int)
	sum.Add(v.BigInt, other.(UInt128Value).BigInt)
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
	if sum.Cmp(sema.UInt128TypeMaxIntBig) > 0 {
		panic(OverflowError{})
	}
	return UInt128Value{sum}
}

func (v UInt128Value) Minus(other NumberValue) NumberValue {
	diff := new(big.Int)
	diff.Sub(v.BigInt, other.(UInt128Value).BigInt)
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
	if diff.Cmp(sema.UInt128TypeMinIntBig) < 0 {
		panic(UnderflowError{})
	}
	return UInt128Value{diff}
}

func (v UInt128Value) Mod(other NumberValue) NumberValue {
	o := other.(UInt128Value)
	res := new(big.Int)
	if o.BigInt.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Rem(v.BigInt, o.BigInt)
	return UInt128Value{res}
}

func (v UInt128Value) Mul(other NumberValue) NumberValue {
	o := other.(UInt128Value)
	res := new(big.Int)
	res.Mul(v.BigInt, o.BigInt)
	if res.Cmp(sema.UInt128TypeMaxIntBig) > 0 {
		panic(OverflowError{})
	}
	return UInt128Value{res}
}

func (v UInt128Value) Div(other NumberValue) NumberValue {
	o := other.(UInt128Value)
	res := new(big.Int)
	if o.BigInt.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Div(v.BigInt, o.BigInt)
	return UInt128Value{res}
}

func (v UInt128Value) Less(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(UInt128Value).BigInt)
	return cmp == -1
}

func (v UInt128Value) LessEqual(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(UInt128Value).BigInt)
	return cmp <= 0
}

func (v UInt128Value) Greater(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(UInt128Value).BigInt)
	return cmp == 1
}

func (v UInt128Value) GreaterEqual(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(UInt128Value).BigInt)
	return cmp >= 0
}

func (v UInt128Value) Equal(other Value) BoolValue {
	otherInt, ok := other.(UInt128Value)
	if !ok {
		return false
	}
	cmp := v.BigInt.Cmp(otherInt.BigInt)
	return cmp == 0
}

func ConvertUInt128(value Value, _ *Interpreter) Value {
	var v *big.Int

	switch value := value.(type) {
	case BigNumberValue:
		v = value.ToBigInt()

	case NumberValue:
		v = big.NewInt(int64(value.ToInt()))

	default:
		panic(errors.NewUnreachableError())
	}

	if v.Cmp(sema.UInt128TypeMaxIntBig) > 0 {
		panic(OverflowError{})
	} else if v.Sign() < 0 {
		panic(UnderflowError{})
	}

	return NewUInt128ValueFromBigInt(v)
}

func (v UInt128Value) BitwiseOr(other IntegerValue) IntegerValue {
	o := other.(UInt128Value)
	res := new(big.Int)
	res.Or(v.BigInt, o.BigInt)
	return UInt128Value{res}
}

func (v UInt128Value) BitwiseXor(other IntegerValue) IntegerValue {
	o := other.(UInt128Value)
	res := new(big.Int)
	res.Xor(v.BigInt, o.BigInt)
	return UInt128Value{res}
}

func (v UInt128Value) BitwiseAnd(other IntegerValue) IntegerValue {
	o := other.(UInt128Value)
	res := new(big.Int)
	res.And(v.BigInt, o.BigInt)
	return UInt128Value{res}
}

func (v UInt128Value) BitwiseLeftShift(other IntegerValue) IntegerValue {
	o := other.(UInt128Value)
	res := new(big.Int)
	if o.BigInt.Sign() < 0 {
		panic(UnderflowError{})
	}
	if !o.BigInt.IsUint64() {
		panic(OverflowError{})
	}
	res.Lsh(v.BigInt, uint(o.BigInt.Uint64()))
	return UInt128Value{res}
}

func (v UInt128Value) BitwiseRightShift(other IntegerValue) IntegerValue {
	o := other.(UInt128Value)
	res := new(big.Int)
	if o.BigInt.Sign() < 0 {
		panic(UnderflowError{})
	}
	if !o.BigInt.IsUint64() {
		panic(OverflowError{})
	}
	res.Rsh(v.BigInt, uint(o.BigInt.Uint64()))
	return UInt128Value{res}
}

func (v UInt128Value) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {

	case sema.ToStringFunctionName:
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := NewStringValue(v.String())
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (UInt128Value) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// UInt256Value

type UInt256Value struct {
	BigInt *big.Int
}

func NewUInt256ValueFromUint64(value uint64) UInt256Value {
	return NewUInt256ValueFromBigInt(new(big.Int).SetUint64(value))
}

func NewUInt256ValueFromBigInt(value *big.Int) UInt256Value {
	return UInt256Value{BigInt: value}
}

func (v UInt256Value) IsValue() {}

func (UInt256Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberDynamicType{&sema.UInt256Type{}}
}

func (v UInt256Value) Copy() Value {
	return UInt256Value{new(big.Int).Set(v.BigInt)}
}

func (UInt256Value) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (UInt256Value) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (UInt256Value) IsModified() bool {
	return false
}

func (UInt256Value) SetModified(_ bool) {
	// NO-OP
}

func (v UInt256Value) ToInt() int {
	// TODO: handle overflow
	return int(v.BigInt.Int64())
}

func (v UInt256Value) ToBigInt() *big.Int {
	return new(big.Int).Set(v.BigInt)
}

func (v UInt256Value) String() string {
	return v.BigInt.String()
}

func (v UInt256Value) KeyString() string {
	return v.BigInt.String()
}

func (v UInt256Value) Negate() NumberValue {
	panic(errors.NewUnreachableError())
}

func (v UInt256Value) Plus(other NumberValue) NumberValue {
	sum := new(big.Int)
	sum.Add(v.BigInt, other.(UInt256Value).BigInt)
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
	if sum.Cmp(sema.UInt256TypeMaxIntBig) > 0 {
		panic(OverflowError{})
	}
	return UInt256Value{sum}
}

func (v UInt256Value) Minus(other NumberValue) NumberValue {
	diff := new(big.Int)
	diff.Sub(v.BigInt, other.(UInt256Value).BigInt)
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
	if diff.Cmp(sema.UInt256TypeMinIntBig) < 0 {
		panic(UnderflowError{})
	}
	return UInt256Value{diff}
}

func (v UInt256Value) Mod(other NumberValue) NumberValue {
	o := other.(UInt256Value)
	res := new(big.Int)
	if o.BigInt.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Rem(v.BigInt, o.BigInt)
	return UInt256Value{res}
}

func (v UInt256Value) Mul(other NumberValue) NumberValue {
	o := other.(UInt256Value)
	res := new(big.Int)
	res.Mul(v.BigInt, o.BigInt)
	if res.Cmp(sema.UInt256TypeMaxIntBig) > 0 {
		panic(OverflowError{})
	}
	return UInt256Value{res}
}

func (v UInt256Value) Div(other NumberValue) NumberValue {
	o := other.(UInt256Value)
	res := new(big.Int)
	if o.BigInt.Cmp(res) == 0 {
		panic(DivisionByZeroError{})
	}
	res.Div(v.BigInt, o.BigInt)
	return UInt256Value{res}
}

func (v UInt256Value) Less(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(UInt256Value).BigInt)
	return cmp == -1
}

func (v UInt256Value) LessEqual(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(UInt256Value).BigInt)
	return cmp <= 0
}

func (v UInt256Value) Greater(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(UInt256Value).BigInt)
	return cmp == 1
}

func (v UInt256Value) GreaterEqual(other NumberValue) BoolValue {
	cmp := v.BigInt.Cmp(other.(UInt256Value).BigInt)
	return cmp >= 0
}

func (v UInt256Value) Equal(other Value) BoolValue {
	otherInt, ok := other.(UInt256Value)
	if !ok {
		return false
	}
	cmp := v.BigInt.Cmp(otherInt.BigInt)
	return cmp == 0
}

func ConvertUInt256(value Value, _ *Interpreter) Value {
	var v *big.Int

	switch value := value.(type) {
	case BigNumberValue:
		v = value.ToBigInt()

	case NumberValue:
		v = big.NewInt(int64(value.ToInt()))

	default:
		panic(errors.NewUnreachableError())
	}

	if v.Cmp(sema.UInt256TypeMaxIntBig) > 0 {
		panic(OverflowError{})
	} else if v.Sign() < 0 {
		panic(UnderflowError{})
	}

	return NewUInt256ValueFromBigInt(v)
}

func (v UInt256Value) BitwiseOr(other IntegerValue) IntegerValue {
	o := other.(UInt256Value)
	res := new(big.Int)
	res.Or(v.BigInt, o.BigInt)
	return UInt256Value{res}
}

func (v UInt256Value) BitwiseXor(other IntegerValue) IntegerValue {
	o := other.(UInt256Value)
	res := new(big.Int)
	res.Xor(v.BigInt, o.BigInt)
	return UInt256Value{res}
}

func (v UInt256Value) BitwiseAnd(other IntegerValue) IntegerValue {
	o := other.(UInt256Value)
	res := new(big.Int)
	res.And(v.BigInt, o.BigInt)
	return UInt256Value{res}
}

func (v UInt256Value) BitwiseLeftShift(other IntegerValue) IntegerValue {
	o := other.(UInt256Value)
	res := new(big.Int)
	if o.BigInt.Sign() < 0 {
		panic(UnderflowError{})
	}
	if !o.BigInt.IsUint64() {
		panic(OverflowError{})
	}
	res.Lsh(v.BigInt, uint(o.BigInt.Uint64()))
	return UInt256Value{res}
}

func (v UInt256Value) BitwiseRightShift(other IntegerValue) IntegerValue {
	o := other.(UInt256Value)
	res := new(big.Int)
	if o.BigInt.Sign() < 0 {
		panic(UnderflowError{})
	}
	if !o.BigInt.IsUint64() {
		panic(OverflowError{})
	}
	res.Rsh(v.BigInt, uint(o.BigInt.Uint64()))
	return UInt256Value{res}
}

func (v UInt256Value) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {

	case sema.ToStringFunctionName:
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := NewStringValue(v.String())
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (UInt256Value) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// Word8Value

type Word8Value uint8

func (Word8Value) IsValue() {}

func (Word8Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberDynamicType{&sema.Word8Type{}}
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

func (Word8Value) IsModified() bool {
	return false
}

func (Word8Value) SetModified(_ bool) {
	// NO-OP
}

func (v Word8Value) String() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v Word8Value) KeyString() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v Word8Value) ToInt() int {
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
		panic(DivisionByZeroError{})
	}
	return v % o
}

func (v Word8Value) Mul(other NumberValue) NumberValue {
	return v * other.(Word8Value)
}

func (v Word8Value) Div(other NumberValue) NumberValue {
	o := other.(Word8Value)
	if o == 0 {
		panic(DivisionByZeroError{})
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

func ConvertWord8(value Value, interpreter *Interpreter) Value {
	return Word8Value(ConvertUInt8(value, interpreter).(UInt8Value))
}

func (v Word8Value) BitwiseOr(other IntegerValue) IntegerValue {
	o := other.(Word8Value)
	return v | o
}

func (v Word8Value) BitwiseXor(other IntegerValue) IntegerValue {
	o := other.(Word8Value)
	return v ^ o
}

func (v Word8Value) BitwiseAnd(other IntegerValue) IntegerValue {
	o := other.(Word8Value)
	return v & o
}

func (v Word8Value) BitwiseLeftShift(other IntegerValue) IntegerValue {
	o := other.(Word8Value)
	return v << o
}

func (v Word8Value) BitwiseRightShift(other IntegerValue) IntegerValue {
	o := other.(Word8Value)
	return v >> o
}

func (v Word8Value) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {

	case sema.ToStringFunctionName:
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := NewStringValue(v.String())
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (Word8Value) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// Word16Value

type Word16Value uint16

func (Word16Value) IsValue() {}

func (Word16Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberDynamicType{&sema.Word16Type{}}
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

func (Word16Value) IsModified() bool {
	return false
}

func (Word16Value) SetModified(_ bool) {
	// NO-OP
}

func (v Word16Value) String() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v Word16Value) KeyString() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v Word16Value) ToInt() int {
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
		panic(DivisionByZeroError{})
	}
	return v % o
}

func (v Word16Value) Mul(other NumberValue) NumberValue {
	return v * other.(Word16Value)
}

func (v Word16Value) Div(other NumberValue) NumberValue {
	o := other.(Word16Value)
	if o == 0 {
		panic(DivisionByZeroError{})
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

func ConvertWord16(value Value, interpreter *Interpreter) Value {
	return Word16Value(ConvertUInt16(value, interpreter).(UInt16Value))
}

func (v Word16Value) BitwiseOr(other IntegerValue) IntegerValue {
	o := other.(Word16Value)
	return v | o
}

func (v Word16Value) BitwiseXor(other IntegerValue) IntegerValue {
	o := other.(Word16Value)
	return v ^ o
}

func (v Word16Value) BitwiseAnd(other IntegerValue) IntegerValue {
	o := other.(Word16Value)
	return v & o
}

func (v Word16Value) BitwiseLeftShift(other IntegerValue) IntegerValue {
	o := other.(Word16Value)
	return v << o
}

func (v Word16Value) BitwiseRightShift(other IntegerValue) IntegerValue {
	o := other.(Word16Value)
	return v >> o
}

func (v Word16Value) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {

	case sema.ToStringFunctionName:
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := NewStringValue(v.String())
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (Word16Value) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// Word32Value

type Word32Value uint32

func (Word32Value) IsValue() {}

func (Word32Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberDynamicType{&sema.Word32Type{}}
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

func (Word32Value) IsModified() bool {
	return false
}

func (Word32Value) SetModified(_ bool) {
	// NO-OP
}

func (v Word32Value) String() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v Word32Value) KeyString() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v Word32Value) ToInt() int {
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
		panic(DivisionByZeroError{})
	}
	return v % o
}

func (v Word32Value) Mul(other NumberValue) NumberValue {
	return v * other.(Word32Value)
}

func (v Word32Value) Div(other NumberValue) NumberValue {
	o := other.(Word32Value)
	if o == 0 {
		panic(DivisionByZeroError{})
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

func ConvertWord32(value Value, interpreter *Interpreter) Value {
	return Word32Value(ConvertUInt32(value, interpreter).(UInt32Value))
}

func (v Word32Value) BitwiseOr(other IntegerValue) IntegerValue {
	o := other.(Word32Value)
	return v | o
}

func (v Word32Value) BitwiseXor(other IntegerValue) IntegerValue {
	o := other.(Word32Value)
	return v ^ o
}

func (v Word32Value) BitwiseAnd(other IntegerValue) IntegerValue {
	o := other.(Word32Value)
	return v & o
}

func (v Word32Value) BitwiseLeftShift(other IntegerValue) IntegerValue {
	o := other.(Word32Value)
	return v << o
}

func (v Word32Value) BitwiseRightShift(other IntegerValue) IntegerValue {
	o := other.(Word32Value)
	return v >> o
}

func (v Word32Value) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {

	case sema.ToStringFunctionName:
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := NewStringValue(v.String())
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (Word32Value) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// Word64Value

type Word64Value uint64

func (Word64Value) IsValue() {}

func (Word64Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberDynamicType{&sema.Word64Type{}}
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

func (Word64Value) IsModified() bool {
	return false
}

func (Word64Value) SetModified(_ bool) {
	// NO-OP
}

func (v Word64Value) String() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v Word64Value) KeyString() string {
	return strconv.FormatUint(uint64(v), 10)
}

func (v Word64Value) ToInt() int {
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
		panic(DivisionByZeroError{})
	}
	return v % o
}

func (v Word64Value) Mul(other NumberValue) NumberValue {
	return v * other.(Word64Value)
}

func (v Word64Value) Div(other NumberValue) NumberValue {
	o := other.(Word64Value)
	if o == 0 {
		panic(DivisionByZeroError{})
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

func ConvertWord64(value Value, interpreter *Interpreter) Value {
	return Word64Value(ConvertUInt64(value, interpreter).(UInt64Value))
}

func (v Word64Value) BitwiseOr(other IntegerValue) IntegerValue {
	o := other.(Word64Value)
	return v | o
}

func (v Word64Value) BitwiseXor(other IntegerValue) IntegerValue {
	o := other.(Word64Value)
	return v ^ o
}

func (v Word64Value) BitwiseAnd(other IntegerValue) IntegerValue {
	o := other.(Word64Value)
	return v & o
}

func (v Word64Value) BitwiseLeftShift(other IntegerValue) IntegerValue {
	o := other.(Word64Value)
	return v << o
}

func (v Word64Value) BitwiseRightShift(other IntegerValue) IntegerValue {
	o := other.(Word64Value)
	return v >> o
}

func (v Word64Value) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {

	case sema.ToStringFunctionName:
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := NewStringValue(v.String())
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (Word64Value) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// Fix64Value

const Fix64MaxValue = math.MaxInt64
const Fix64MaxDivisorValue = sema.Fix64Factor * sema.Fix64Factor
const Fix64MaxIntDividend = Fix64MaxValue / sema.Fix64Factor

type Fix64Value int64

func NewFix64ValueWithInteger(integer int64) Fix64Value {

	if integer < sema.Fix64TypeMinInt {
		panic(UnderflowError{})
	}

	if integer > sema.Fix64TypeMaxInt {
		panic(OverflowError{})
	}

	return Fix64Value(integer * sema.Fix64Factor)
}

func (Fix64Value) IsValue() {}

func (Fix64Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberDynamicType{&sema.Fix64Type{}}
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

func (Fix64Value) IsModified() bool {
	return false
}

func (Fix64Value) SetModified(_ bool) {
	// NO-OP
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

func (v Fix64Value) ToInt() int {
	return int(v / sema.Fix64Factor)
}

func (v Fix64Value) Negate() NumberValue {
	// INT32-C
	if v == math.MinInt64 {
		panic(OverflowError{})
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
		panic(OverflowError{})
	} else if (o < 0) && (v > (math.MaxInt64 + o)) {
		panic(UnderflowError{})
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
		panic(OverflowError{})
	}

	x1y1Fixed := x1y1 * sema.Fix64Factor
	if x1y1 != 0 && x1y1Fixed/x1y1 != sema.Fix64Factor {
		panic(OverflowError{})
	}
	x1y1 = x1y1Fixed

	x2y1 := x2 * y1
	if x2 != 0 && x2y1/x2 != y1 {
		panic(OverflowError{})
	}

	x1y2 := x1 * y2
	if x1 != 0 && x1y2/x1 != y2 {
		panic(OverflowError{})
	}

	x2 = x2 / Fix64MulPrecision
	y2 = y2 / Fix64MulPrecision
	x2y2 := x2 * y2
	if x2 != 0 && x2y2/x2 != y2 {
		panic(OverflowError{})
	}

	result := x1y1
	result = safeAddInt64(result, x2y1)
	result = safeAddInt64(result, x1y2)
	result = safeAddInt64(result, x2y2)
	return Fix64Value(result)
}

func (v Fix64Value) Reciprocal() Fix64Value {
	if v == 0 {
		panic(DivisionByZeroError{})
	}
	return Fix64MaxDivisorValue / v
}

func (v Fix64Value) Div(other NumberValue) NumberValue {
	o := other.(Fix64Value)
	if o > Fix64MaxDivisorValue {
		panic(OverflowError{})
	}
	return v.Mul(o.Reciprocal())
}

func (v Fix64Value) Mod(other NumberValue) NumberValue {
	o := other.(Fix64Value)
	// v - int(v/o) * o
	quotient := v.Div(o).(Fix64Value)
	truncatedQuotient := (int64(quotient) / sema.Fix64Factor) * sema.Fix64Factor
	return v.Minus(Fix64Value(truncatedQuotient).Mul(o))
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

func ConvertFix64(value Value, interpreter *Interpreter) Value {
	switch value := value.(type) {
	case Fix64Value:
		return value

	case UFix64Value:
		if value > Fix64MaxValue {
			panic(OverflowError{})
		}
		return Fix64Value(value)

	case BigNumberValue:
		v := value.ToBigInt()

		// First, check if the value is at least in the int64 range.
		// The integer range for Fix64 is smaller, but this test at least
		// allows us to call `v.Int64()` safely.

		if !v.IsInt64() {
			panic(OverflowError{})
		}

		// Now check that the integer value fits the range of Fix64
		return NewFix64ValueWithInteger(v.Int64())

	case NumberValue:
		v := value.ToInt()
		// Check that the integer value fits the range of Fix64
		return NewFix64ValueWithInteger(int64(v))

	default:
		panic(fmt.Sprintf(
			"can't convert %s to Fix64: %s",
			value.DynamicType(interpreter),
			value,
		))
	}
}

func (v Fix64Value) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {

	case sema.ToStringFunctionName:
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := NewStringValue(v.String())
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (Fix64Value) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// UFix64Value

type UFix64Value uint64

const UFix64MaxValue = math.MaxUint64
const UFix64MaxDivisorValue = sema.Fix64Factor * sema.Fix64Factor
const UFix64MaxIntDividend = UFix64MaxValue / sema.Fix64Factor

func NewUFix64ValueWithInteger(integer uint64) UFix64Value {
	if integer > sema.UFix64TypeMaxInt {
		panic(OverflowError{})
	}

	return UFix64Value(integer * sema.Fix64Factor)
}

func (UFix64Value) IsValue() {}

func (UFix64Value) DynamicType(_ *Interpreter) DynamicType {
	return NumberDynamicType{&sema.UFix64Type{}}
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

func (UFix64Value) IsModified() bool {
	return false
}

func (UFix64Value) SetModified(_ bool) {
	// NO-OP
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

func (v UFix64Value) ToInt() int {
	return int(v / sema.Fix64Factor)
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
		panic(OverflowError{})
	}

	x1y1Fixed := x1y1 * factor
	if x1y1 != 0 && x1y1Fixed/x1y1 != factor {
		panic(OverflowError{})
	}
	x1y1 = x1y1Fixed

	x2y1 := x2 * y1
	if x2 != 0 && x2y1/x2 != y1 {
		panic(OverflowError{})
	}

	x1y2 := x1 * y2
	if x1 != 0 && x1y2/x1 != y2 {
		panic(OverflowError{})
	}

	x2 = x2 / UFix64MulPrecision
	y2 = y2 / UFix64MulPrecision
	x2y2 := x2 * y2
	if x2 != 0 && x2y2/x2 != y2 {
		panic(OverflowError{})
	}

	result := x1y1
	result = safeAddUint64(result, x2y1)
	result = safeAddUint64(result, x1y2)
	result = safeAddUint64(result, x2y2)
	return UFix64Value(result)
}

func (v UFix64Value) Reciprocal() UFix64Value {
	if v == 0 {
		panic(DivisionByZeroError{})
	}
	return UFix64MaxDivisorValue / v
}

func (v UFix64Value) Div(other NumberValue) NumberValue {
	o := other.(UFix64Value)
	if o > UFix64MaxDivisorValue {
		panic(OverflowError{})
	}
	return v.Mul(o.Reciprocal())
}

func (v UFix64Value) Mod(other NumberValue) NumberValue {
	o := other.(UFix64Value)
	// v - int(v/o) * o
	quotient := v.Div(o).(UFix64Value)
	truncatedQuotient := (uint64(quotient) / sema.Fix64Factor) * sema.Fix64Factor
	return v.Minus(UFix64Value(truncatedQuotient).Mul(o))
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
	switch value := value.(type) {
	case UFix64Value:
		return value

	case Fix64Value:
		if value < 0 {
			panic(UnderflowError{})
		}
		return UFix64Value(value)

	case BigNumberValue:
		v := value.ToBigInt()

		if v.Sign() < 0 {
			panic(UnderflowError{})
		}

		// First, check if the value is at least in the uint64 range.
		// The integer range for UFix64 is smaller, but this test at least
		// allows us to call `v.UInt64()` safely.

		if !v.IsUint64() {
			panic(OverflowError{})
		}

		// Now check that the integer value fits the range of UFix64
		return NewUFix64ValueWithInteger(v.Uint64())

	case NumberValue:
		v := value.ToInt()
		if v < 0 {
			panic(UnderflowError{})
		}
		// Check that the integer value fits the range of UFix64
		return NewUFix64ValueWithInteger(uint64(v))

	default:
		panic(fmt.Sprintf(
			"can't convert %s to UFix64: %s",
			value.DynamicType(interpreter),
			value,
		))
	}
}

func (v UFix64Value) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {

	case sema.ToStringFunctionName:
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := NewStringValue(v.String())
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (UFix64Value) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
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
	destroyed      bool
	modified       bool
}

func NewCompositeValue(
	location ast.Location,
	typeID sema.TypeID,
	kind common.CompositeKind,
	fields map[string]Value,
	owner *common.Address,
) *CompositeValue {
	if fields == nil {
		fields = map[string]Value{}
	}
	return &CompositeValue{
		Location: location,
		TypeID:   typeID,
		Kind:     kind,
		Fields:   fields,
		Owner:    owner,
		modified: true,
	}
}

func (v *CompositeValue) Destroy(interpreter *Interpreter, locationRange LocationRange) trampoline.Trampoline {

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
			LocationRange: locationRange,
			Interpreter:   interpreter,
		}

		tramp = destructor.Invoke(invocation)
	}

	return tramp.Then(func(_ interface{}) {
		v.destroyed = true
		v.modified = true
	})
}

func (*CompositeValue) IsValue() {}

func (v *CompositeValue) DynamicType(interpreter *Interpreter) DynamicType {
	staticType := interpreter.getCompositeType(v.Location, v.TypeID)
	return CompositeDynamicType{
		StaticType: staticType,
	}
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
		destroyed:      v.destroyed,
		// NOTE: new value has no owner
		Owner:    nil,
		modified: true,
	}
}

func (v *CompositeValue) checkStatus(locationRange LocationRange) {
	if v.destroyed {
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

func (v *CompositeValue) IsModified() bool {
	if v.modified {
		return true
	}

	for _, value := range v.Fields {
		if value.IsModified() {
			return true
		}
	}

	for _, value := range v.InjectedFields {
		if value.IsModified() {
			return true
		}
	}

	for _, value := range v.NestedValues {
		if value.IsModified() {
			return true
		}
	}

	return false
}

func (v *CompositeValue) SetModified(modified bool) {
	v.modified = modified
}

func (v *CompositeValue) GetMember(interpreter *Interpreter, locationRange LocationRange, name string) Value {
	v.checkStatus(locationRange)

	if v.Kind == common.CompositeKindResource &&
		name == "owner" {

		return v.OwnerValue()
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

	v.InitializeFunctions(interpreter)

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

func (v *CompositeValue) InitializeFunctions(interpreter *Interpreter) {
	if v.Functions != nil {
		return
	}
	v.Functions = interpreter.typeCodes.compositeCodes[v.TypeID].compositeFunctions
}

func (v *CompositeValue) OwnerValue() OptionalValue {
	if v.Owner == nil {
		return NilValue{}
	}

	address := AddressValue(*v.Owner)

	return NewSomeValueOwningNonCopying(
		PublicAccountValue{Address: address},
	)
}

func (v *CompositeValue) SetMember(_ *Interpreter, locationRange LocationRange, name string, value Value) {
	v.checkStatus(locationRange)

	v.modified = true

	value.SetOwner(v.Owner)

	v.Fields[name] = value
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
	Keys     *ArrayValue
	Entries  map[string]Value
	Owner    *common.Address
	modified bool
	// Deferral of values:
	//
	// Values in the dictionary might be deferred, i.e. are they encoded
	// separately and stored in separate storage keys.
	//
	// DeferredOwner is the account in which the deferred values are stored.
	// The account might differ from the owner: If the dictionary is moved
	// from one account to another, its owner changes, but its deferred values
	// stay stored in the deferred owner's account until the end of the transaction.
	DeferredOwner *common.Address
	// DeferredKeys are the keys which are deferred and have not been loaded from storage yet.
	DeferredKeys map[string]string
	// prevDeferredKeys are the keys which are deferred and have been loaded from storage,
	// i.e. they are keys that were previously in DeferredKeys.
	prevDeferredKeys map[string]string
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
		Owner:            nil,
		modified:         true,
		DeferredOwner:    nil,
		DeferredKeys:     nil,
		prevDeferredKeys: nil,
	}

	for i := 0; i < keysAndValuesCount; i += 2 {
		_ = result.Insert(nil, LocationRange{}, keysAndValues[i], keysAndValues[i+1])
	}

	return result
}

func (*DictionaryValue) IsValue() {}

func (v *DictionaryValue) DynamicType(interpreter *Interpreter) DynamicType {
	entryTypes := make([]struct{ KeyType, ValueType DynamicType }, len(v.Keys.Values))

	for i, key := range v.Keys.Values {
		// NOTE: Force unwrap, otherwise dynamic type check is for optional type.
		// This is safe because we are iterating over the keys.
		value := v.Get(interpreter, LocationRange{}, key).(*SomeValue).Value
		entryTypes[i] =
			struct{ KeyType, ValueType DynamicType }{
				KeyType:   key.DynamicType(interpreter),
				ValueType: value.DynamicType(interpreter),
			}
	}

	return DictionaryDynamicType{
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
		Keys:             newKeys,
		Entries:          newEntries,
		DeferredOwner:    v.DeferredOwner,
		DeferredKeys:     v.DeferredKeys,
		prevDeferredKeys: v.prevDeferredKeys,
		// NOTE: new value has no owner
		Owner:    nil,
		modified: true,
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

func (v *DictionaryValue) IsModified() bool {
	if v.modified {
		return true
	}

	if v.Keys.IsModified() {
		return true
	}

	for _, value := range v.Entries {
		if value.IsModified() {
			return true
		}
	}

	return false
}

func (v *DictionaryValue) SetModified(modified bool) {
	v.modified = modified
}

func (v *DictionaryValue) Destroy(interpreter *Interpreter, locationRange LocationRange) trampoline.Trampoline {
	var result trampoline.Trampoline = trampoline.Done{}

	maybeDestroy := func(value interface{}) {
		destroyableValue, ok := value.(DestroyableValue)
		if !ok {
			return
		}

		result = result.
			FlatMap(func(_ interface{}) trampoline.Trampoline {
				return destroyableValue.Destroy(interpreter, locationRange)
			})
	}

	for _, keyValue := range v.Keys.Values {
		// Don't use `Entries` here: the value might be deferred and needs to be loaded
		value := v.Get(interpreter, locationRange, keyValue)
		maybeDestroy(keyValue)
		maybeDestroy(value)
	}

	for _, storageKey := range v.DeferredKeys {
		interpreter.writeStored(*v.DeferredOwner, storageKey, NilValue{})
	}

	for _, storageKey := range v.prevDeferredKeys {
		interpreter.writeStored(*v.DeferredOwner, storageKey, NilValue{})
	}

	return result
}

func (v *DictionaryValue) Get(inter *Interpreter, _ LocationRange, keyValue Value) Value {
	key := dictionaryKey(keyValue)
	value, ok := v.Entries[key]
	if ok {
		return NewSomeValueOwningNonCopying(value)
	}

	// Is the key a deferred value? If so, load it from storage
	// and keep it as an entry in memory

	if v.DeferredKeys != nil {
		storageKey, ok := v.DeferredKeys[key]
		if ok {
			delete(v.DeferredKeys, key)
			if v.prevDeferredKeys == nil {
				v.prevDeferredKeys = map[string]string{}
			}
			v.prevDeferredKeys[key] = storageKey

			storedValue := inter.readStored(*v.DeferredOwner, storageKey, true)
			v.Entries[key] = storedValue.(*SomeValue).Value

			// NOTE: *not* writing nil to the storage key,
			// as this would result in a loss of the value:
			// the read value is not modified,
			// so it won't be written back

			return storedValue
		}
	}

	return NilValue{}

}

func dictionaryKey(keyValue Value) string {
	hasKeyString, ok := keyValue.(HasKeyString)
	if !ok {
		panic(errors.NewUnreachableError())
	}
	return hasKeyString.KeyString()
}

func (v *DictionaryValue) Set(inter *Interpreter, locationRange LocationRange, keyValue Value, value Value) {
	v.modified = true

	switch typedValue := value.(type) {
	case *SomeValue:
		_ = v.Insert(inter, locationRange, keyValue, typedValue.Value)

	case NilValue:
		_ = v.Remove(inter, locationRange, keyValue)
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
		return NewIntValueFromInt64(int64(v.Count()))

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

				existingValue := v.Remove(
					invocation.Interpreter,
					invocation.LocationRange,
					keyValue,
				)

				return trampoline.Done{Result: existingValue}
			},
		)

	case "insert":
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				keyValue := invocation.Arguments[0]
				newValue := invocation.Arguments[1]

				existingValue := v.Insert(
					invocation.Interpreter,
					invocation.LocationRange,
					keyValue,
					newValue,
				)

				return trampoline.Done{Result: existingValue}
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
func (v *DictionaryValue) Remove(inter *Interpreter, locationRange LocationRange, keyValue Value) OptionalValue {
	v.modified = true

	// Don't use `Entries` here: the value might be deferred and needs to be loaded
	value := v.Get(inter, locationRange, keyValue)

	key := dictionaryKey(keyValue)

	// If a resource that was previously deferred is removed from the dictionary,
	// we delete its old key in storage, and then rely on resource semantics
	// to make sure it is stored or destroyed later

	if v.prevDeferredKeys != nil {
		if storageKey, ok := v.prevDeferredKeys[key]; ok {
			inter.writeStored(*v.DeferredOwner, storageKey, NilValue{})
		}
	}

	switch value := value.(type) {
	case *SomeValue:

		delete(v.Entries, key)

		// TODO: optimize linear scan
		for i, keyValue := range v.Keys.Values {
			if dictionaryKey(keyValue) == key {
				v.Keys.Remove(i)
				return value
			}
		}

		// Should never occur, the key should have been found
		panic(errors.NewUnreachableError())

	case NilValue:
		return value

	default:
		panic(errors.NewUnreachableError())
	}
}

func (v *DictionaryValue) Insert(inter *Interpreter, locationRange LocationRange, keyValue, value Value) OptionalValue {
	v.modified = true

	// Don't use `Entries` here: the value might be deferred and needs to be loaded
	existingValue := v.Get(inter, locationRange, keyValue)

	key := dictionaryKey(keyValue)

	value.SetOwner(v.Owner)

	// Mark the inserted value itself modified.
	// It might have been stored as a deferred value and loaded,
	// so must be written (potentially as a deferred value again),
	// and would otherwise be ignored by the writeback optimization.

	value.SetModified(true)

	v.Entries[key] = value

	switch existingValue := existingValue.(type) {
	case *SomeValue:
		return existingValue

	case NilValue:
		v.Keys.Append(keyValue)
		return existingValue

	default:
		panic(errors.NewUnreachableError())
	}
}

type DictionaryEntryValues struct {
	Key   Value
	Value Value
}

// OptionalValue

type OptionalValue interface {
	Value
	isOptionalValue()
}

// NilValue

type NilValue struct{}

func (NilValue) IsValue() {}

func (NilValue) DynamicType(_ *Interpreter) DynamicType {
	return NilDynamicType{}
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

func (NilValue) IsModified() bool {
	return false
}

func (NilValue) SetModified(_ bool) {
	// NO-OP
}

func (v NilValue) Destroy(_ *Interpreter, _ LocationRange) trampoline.Trampoline {
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

func NewSomeValueOwningNonCopying(value Value) *SomeValue {
	return &SomeValue{
		Value: value,
		Owner: value.GetOwner(),
	}
}

func (*SomeValue) IsValue() {}

func (v *SomeValue) DynamicType(interpreter *Interpreter) DynamicType {
	innerType := v.Value.DynamicType(interpreter)
	return SomeDynamicType{InnerType: innerType}
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

func (v *SomeValue) IsModified() bool {
	return v.Value.IsModified()
}

func (v *SomeValue) SetModified(modified bool) {
	v.Value.SetModified(modified)
}

func (v *SomeValue) Destroy(interpreter *Interpreter, locationRange LocationRange) trampoline.Trampoline {
	return v.Value.(DestroyableValue).Destroy(interpreter, locationRange)
}

func (v *SomeValue) String() string {
	return fmt.Sprint(v.Value)
}

// StorageReferenceValue

type StorageReferenceValue struct {
	Authorized           bool
	TargetStorageAddress common.Address
	TargetKey            string
}

func (*StorageReferenceValue) IsValue() {}

func (v *StorageReferenceValue) DynamicType(interpreter *Interpreter) DynamicType {
	referencedValue := v.referencedValue(interpreter)
	if referencedValue == nil {
		panic(&DereferenceError{})
	}

	innerType := (*referencedValue).DynamicType(interpreter)

	return StorageReferenceDynamicType{
		authorized: v.Authorized,
		innerType:  innerType,
	}
}

func (v *StorageReferenceValue) Copy() Value {
	return &StorageReferenceValue{
		Authorized:           v.Authorized,
		TargetStorageAddress: v.TargetStorageAddress,
		TargetKey:            v.TargetKey,
	}
}

func (v *StorageReferenceValue) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (v *StorageReferenceValue) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (*StorageReferenceValue) IsModified() bool {
	return false
}

func (*StorageReferenceValue) SetModified(_ bool) {
	// NO-OP
}

func (v *StorageReferenceValue) referencedValue(interpreter *Interpreter) *Value {
	switch referenced := interpreter.readStored(v.TargetStorageAddress, v.TargetKey, false).(type) {
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
	referencedValue := v.referencedValue()
	if referencedValue == nil {
		panic(&DereferenceError{})
	}

	innerType := (*referencedValue).DynamicType(interpreter)

	return EphemeralReferenceDynamicType{
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

func (*EphemeralReferenceValue) IsModified() bool {
	return false
}

func (*EphemeralReferenceValue) SetModified(_ bool) {
	// NO-OP
}

func (v *EphemeralReferenceValue) referencedValue() *Value {
	// Just like for storage references, references to optionals are unwrapped,
	// i.e. a reference to `nil` aborts when dereferenced.

	switch referenced := v.Value.(type) {
	case *SomeValue:
		return &referenced.Value
	case NilValue:
		return nil
	default:
		return &v.Value
	}
}

func (v *EphemeralReferenceValue) GetMember(interpreter *Interpreter, locationRange LocationRange, name string) Value {
	referencedValue := v.referencedValue()
	if referencedValue == nil {
		panic(&DereferenceError{
			LocationRange: locationRange,
		})
	}

	return (*referencedValue).(MemberAccessibleValue).
		GetMember(interpreter, locationRange, name)
}

func (v *EphemeralReferenceValue) SetMember(interpreter *Interpreter, locationRange LocationRange, name string, value Value) {
	referencedValue := v.referencedValue()
	if referencedValue == nil {
		panic(&DereferenceError{
			LocationRange: locationRange,
		})
	}

	(*referencedValue).(MemberAccessibleValue).
		SetMember(interpreter, locationRange, name, value)
}

func (v *EphemeralReferenceValue) Get(interpreter *Interpreter, locationRange LocationRange, key Value) Value {
	referencedValue := v.referencedValue()
	if referencedValue == nil {
		panic(&DereferenceError{
			LocationRange: locationRange,
		})
	}

	return (*referencedValue).(ValueIndexableValue).
		Get(interpreter, locationRange, key)
}

func (v *EphemeralReferenceValue) Set(interpreter *Interpreter, locationRange LocationRange, key Value, value Value) {
	referencedValue := v.referencedValue()
	if referencedValue == nil {
		panic(&DereferenceError{
			LocationRange: locationRange,
		})
	}

	(*referencedValue).(ValueIndexableValue).
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
		bigEndianBytes := intValue.BigInt.Bytes()
		copy(
			result[common.AddressLength-len(bigEndianBytes):common.AddressLength],
			bigEndianBytes,
		)
	} else {
		binary.BigEndian.PutUint64(
			result[common.AddressLength-8:common.AddressLength],
			uint64(value.(NumberValue).ToInt()),
		)
	}
	return result
}

func (AddressValue) IsValue() {}

func (AddressValue) DynamicType(_ *Interpreter) DynamicType {
	return AddressDynamicType{}
}

func (v AddressValue) Copy() Value {
	return v
}

func (v AddressValue) KeyString() string {
	return v.String()
}

func (v AddressValue) String() string {
	hexString := fmt.Sprintf("%x", [common.AddressLength]byte(v))
	return fmt.Sprintf("0x%s", strings.TrimLeft(hexString, "0"))
}

func (AddressValue) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (AddressValue) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (AddressValue) IsModified() bool {
	return false
}

func (AddressValue) SetModified(_ bool) {
	// NO-OP
}

func (v AddressValue) Equal(other Value) BoolValue {
	otherAddress, ok := other.(AddressValue)
	if !ok {
		return false
	}
	return v == otherAddress
}

func (v AddressValue) Hex() string {
	return v.ToAddress().Hex()
}

func (v AddressValue) ToAddress() common.Address {
	return common.Address(v)
}

func (v AddressValue) GetMember(_ *Interpreter, _ LocationRange, name string) Value {
	switch name {

	case sema.ToStringFunctionName:
		return NewHostFunctionValue(
			func(invocation Invocation) trampoline.Trampoline {
				result := NewStringValue(v.String())
				return trampoline.Done{Result: result}
			},
		)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (AddressValue) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// AccountValue

type AccountValue interface {
	isAccountValue()
	AddressValue() AddressValue
}

// AuthAccountValue

type AuthAccountValue struct {
	Address                              AddressValue
	setCodeFunction                      FunctionValue
	unsafeNotInitializingSetCodeFunction FunctionValue
	addPublicKeyFunction                 FunctionValue
	removePublicKeyFunction              FunctionValue
}

func NewAuthAccountValue(
	address AddressValue,
	setCodeFunction FunctionValue,
	unsafeNotInitializingSetCodeFunction FunctionValue,
	addPublicKeyFunction FunctionValue,
	removePublicKeyFunction FunctionValue,
) AuthAccountValue {
	return AuthAccountValue{
		Address:                              address,
		setCodeFunction:                      setCodeFunction,
		unsafeNotInitializingSetCodeFunction: unsafeNotInitializingSetCodeFunction,
		addPublicKeyFunction:                 addPublicKeyFunction,
		removePublicKeyFunction:              removePublicKeyFunction,
	}
}

func (AuthAccountValue) IsValue() {}

func (AuthAccountValue) isAccountValue() {}

func (v AuthAccountValue) AddressValue() AddressValue {
	return v.Address
}

func (AuthAccountValue) DynamicType(_ *Interpreter) DynamicType {
	return AuthAccountDynamicType{}
}

func (v AuthAccountValue) Copy() Value {
	return v
}

func (AuthAccountValue) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (AuthAccountValue) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (AuthAccountValue) IsModified() bool {
	return false
}

func (AuthAccountValue) SetModified(_ bool) {
	// NO-OP
}

func (v AuthAccountValue) Destroy(_ *Interpreter, _ LocationRange) trampoline.Trampoline {
	return trampoline.Done{}
}

func (v AuthAccountValue) String() string {
	return fmt.Sprintf("AuthAccount(%s)", v.Address)
}

func accountGetCapabilityFunction(
	addressValue AddressValue,
	authorized bool,
) HostFunctionValue {
	return NewHostFunctionValue(func(invocation Invocation) trampoline.Trampoline {

		path := invocation.Arguments[0].(PathValue)

		if authorized {

			// If the account is an authorized account (`AuthAccount`),
			// ensure the path has a `private` or `public` domain.

			if !checkPathDomain(
				path,
				common.PathDomainPrivate,
				common.PathDomainPublic,
			) {
				return trampoline.Done{Result: NilValue{}}
			}
		} else {

			// If the account is a public account (`PublicAccount`),
			// ensure the path has a `public` domain.

			if !checkPathDomain(
				path,
				common.PathDomainPublic,
			) {
				return trampoline.Done{Result: NilValue{}}
			}
		}

		capability := CapabilityValue{
			Address: addressValue,
			Path:    path,
		}

		result := NewSomeValueOwningNonCopying(capability)

		return trampoline.Done{Result: result}
	})
}

func (v AuthAccountValue) GetMember(inter *Interpreter, _ LocationRange, name string) Value {
	switch name {
	case "address":
		return v.Address

	case "setCode":
		return v.setCodeFunction

	case "unsafeNotInitializingSetCode":
		return v.unsafeNotInitializingSetCodeFunction

	case "addPublicKey":
		return v.addPublicKeyFunction

	case "removePublicKey":
		return v.removePublicKeyFunction

	case "load":
		return inter.authAccountLoadFunction(v.Address)

	case "copy":
		return inter.authAccountCopyFunction(v.Address)

	case "save":
		return inter.authAccountSaveFunction(v.Address)

	case "borrow":
		return inter.authAccountBorrowFunction(v.Address)

	case "link":
		return inter.authAccountLinkFunction(v.Address)

	case "unlink":
		return inter.authAccountUnlinkFunction(v.Address)

	case "getLinkTarget":
		return inter.authAccountGetLinkTargetFunction(v.Address)

	case "getCapability":
		return accountGetCapabilityFunction(v.Address, true)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (AuthAccountValue) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// PublicAccountValue

type PublicAccountValue struct {
	Address    AddressValue
	Identifier string
}

func NewPublicAccountValue(address AddressValue) PublicAccountValue {
	return PublicAccountValue{
		Address: address,
	}
}

func (PublicAccountValue) IsValue() {}

func (PublicAccountValue) isAccountValue() {}

func (v PublicAccountValue) AddressValue() AddressValue {
	return v.Address
}

func (PublicAccountValue) DynamicType(_ *Interpreter) DynamicType {
	return AuthAccountDynamicType{}
}

func (v PublicAccountValue) Copy() Value {
	return v
}

func (PublicAccountValue) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (PublicAccountValue) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (PublicAccountValue) IsModified() bool {
	return false
}

func (PublicAccountValue) SetModified(_ bool) {
	// NO-OP
}

func (v PublicAccountValue) Destroy(_ *Interpreter, _ LocationRange) trampoline.Trampoline {
	return trampoline.Done{}
}

func (v PublicAccountValue) String() string {
	return fmt.Sprintf("PublicAccount(%s)", v.Address)
}

func (v PublicAccountValue) GetMember(inter *Interpreter, _ LocationRange, name string) Value {
	switch name {
	case "address":
		return v.Address

	case "getCapability":
		return accountGetCapabilityFunction(v.Address, false)

	case "getLinkTarget":
		return inter.authAccountGetLinkTargetFunction(v.Address)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (PublicAccountValue) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// PathValue

type PathValue struct {
	Domain     common.PathDomain
	Identifier string
}

func (PathValue) IsValue() {}

func (PathValue) DynamicType(_ *Interpreter) DynamicType {
	return PathDynamicType{}
}

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

func (PathValue) IsModified() bool {
	return false
}

func (PathValue) SetModified(_ bool) {
	// NO-OP
}

func (v PathValue) Destroy(_ *Interpreter, _ LocationRange) trampoline.Trampoline {
	return trampoline.Done{}
}

func (v PathValue) String() string {
	return fmt.Sprintf(
		"/%s/%s",
		v.Domain.Identifier(),
		v.Identifier,
	)
}

// CapabilityValue

type CapabilityValue struct {
	Address AddressValue
	Path    PathValue
}

func (CapabilityValue) IsValue() {}

func (CapabilityValue) DynamicType(_ *Interpreter) DynamicType {
	return CapabilityDynamicType{}
}

func (v CapabilityValue) Copy() Value {
	return v
}

func (CapabilityValue) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (CapabilityValue) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (CapabilityValue) IsModified() bool {
	return false
}

func (CapabilityValue) SetModified(_ bool) {
	// NO-OP
}

func (v CapabilityValue) Destroy(_ *Interpreter, _ LocationRange) trampoline.Trampoline {
	return trampoline.Done{}
}

func (v CapabilityValue) String() string {
	return fmt.Sprintf(
		"/%s%s",
		v.Address,
		v.Path,
	)
}

func (v CapabilityValue) GetMember(inter *Interpreter, _ LocationRange, name string) Value {
	switch name {
	case "borrow":
		return inter.capabilityBorrowFunction(v.Address, v.Path)

	case "check":
		return inter.capabilityCheckFunction(v.Address, v.Path)

	default:
		panic(errors.NewUnreachableError())
	}
}

func (CapabilityValue) SetMember(_ *Interpreter, _ LocationRange, _ string, _ Value) {
	panic(errors.NewUnreachableError())
}

// LinkValue

type LinkValue struct {
	TargetPath PathValue
	Type       StaticType
}

func (LinkValue) IsValue() {}

func (LinkValue) DynamicType(_ *Interpreter) DynamicType {
	return CapabilityDynamicType{}
}

func (v LinkValue) Copy() Value {
	return v
}

func (LinkValue) GetOwner() *common.Address {
	// value is never owned
	return nil
}

func (LinkValue) SetOwner(_ *common.Address) {
	// NO-OP: value cannot be owned
}

func (LinkValue) IsModified() bool {
	return false
}

func (LinkValue) SetModified(_ bool) {
	// NO-OP
}

func (v LinkValue) Destroy(_ *Interpreter, _ LocationRange) trampoline.Trampoline {
	return trampoline.Done{}
}

func (v LinkValue) String() string {
	return fmt.Sprintf(
		"Link(type: %s, targetPath: %s)",
		v.Type,
		v.TargetPath,
	)
}
