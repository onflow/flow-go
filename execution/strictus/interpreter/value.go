package interpreter

import "fmt"

type Value interface {
	isValue()
}

// VoidValue

type VoidValue struct{}

func (VoidValue) isValue() {}

// BoolValue

type BoolValue bool

func (BoolValue) isValue() {}

func (v BoolValue) Negate() BoolValue {
	return !v
}

// ArrayValue

type ArrayValue []interface{}

func (ArrayValue) isValue() {}

// IntValue

type IntValue interface {
	Value
	IntValue() int
	Negate() IntValue
	Plus(other IntValue) IntValue
	Minus(other IntValue) IntValue
	Mod(other IntValue) IntValue
	Mul(other IntValue) IntValue
	Div(other IntValue) IntValue
	Less(other IntValue) BoolValue
	LessEqual(other IntValue) BoolValue
	Greater(other IntValue) BoolValue
	GreaterEqual(other IntValue) BoolValue
}

// Int8Value

type Int8Value int8

func (Int8Value) isValue() {}

func (e Int8Value) IntValue() int {
	return int(e)
}

func (e Int8Value) Negate() IntValue {
	return -e
}

func (e Int8Value) Plus(other IntValue) IntValue {
	return e + other.(Int8Value)
}

func (e Int8Value) Minus(other IntValue) IntValue {
	return e - other.(Int8Value)
}

func (e Int8Value) Mod(other IntValue) IntValue {
	return e % other.(Int8Value)
}

func (e Int8Value) Mul(other IntValue) IntValue {
	return e * other.(Int8Value)
}

func (e Int8Value) Div(other IntValue) IntValue {
	return e / other.(Int8Value)
}

func (e Int8Value) Less(other IntValue) BoolValue {
	return e < other.(Int8Value)
}

func (e Int8Value) LessEqual(other IntValue) BoolValue {
	return e <= other.(Int8Value)
}

func (e Int8Value) Greater(other IntValue) BoolValue {
	return e > other.(Int8Value)
}

func (e Int8Value) GreaterEqual(other IntValue) BoolValue {
	return e >= other.(Int8Value)
}

// Int16Value

type Int16Value int16

func (Int16Value) isValue() {}

func (e Int16Value) IntValue() int {
	return int(e)
}

func (e Int32Value) Negate() IntValue {
	return -e
}

func (e Int16Value) Plus(other IntValue) IntValue {
	return e + other.(Int16Value)
}

func (e Int16Value) Minus(other IntValue) IntValue {
	return e - other.(Int16Value)
}

func (e Int16Value) Mod(other IntValue) IntValue {
	return e % other.(Int16Value)
}

func (e Int16Value) Mul(other IntValue) IntValue {
	return e * other.(Int16Value)
}

func (e Int16Value) Div(other IntValue) IntValue {
	return e / other.(Int16Value)
}

func (e Int16Value) Less(other IntValue) BoolValue {
	return e < other.(Int16Value)
}

func (e Int16Value) LessEqual(other IntValue) BoolValue {
	return e <= other.(Int16Value)
}

func (e Int16Value) Greater(other IntValue) BoolValue {
	return e > other.(Int16Value)
}

func (e Int16Value) GreaterEqual(other IntValue) BoolValue {
	return e >= other.(Int16Value)
}

// Int32Value

type Int32Value int32

func (Int32Value) isValue() {}

func (e Int32Value) IntValue() int {
	return int(e)
}

func (e Int16Value) Negate() IntValue {
	return -e
}

func (e Int32Value) Plus(other IntValue) IntValue {
	return e + other.(Int32Value)
}

func (e Int32Value) Minus(other IntValue) IntValue {
	return e - other.(Int32Value)
}

func (e Int32Value) Mod(other IntValue) IntValue {
	return e % other.(Int32Value)
}

func (e Int32Value) Mul(other IntValue) IntValue {
	return e * other.(Int32Value)
}

func (e Int32Value) Div(other IntValue) IntValue {
	return e / other.(Int32Value)
}

func (e Int32Value) Less(other IntValue) BoolValue {
	return e < other.(Int32Value)
}

func (e Int32Value) LessEqual(other IntValue) BoolValue {
	return e <= other.(Int32Value)
}

func (e Int32Value) Greater(other IntValue) BoolValue {
	return e > other.(Int32Value)
}

func (e Int32Value) GreaterEqual(other IntValue) BoolValue {
	return e >= other.(Int32Value)
}

// Int64Value

type Int64Value int64

func (Int64Value) isValue() {}

func (e Int64Value) IntValue() int {
	return int(e)
}

func (e Int64Value) Negate() IntValue {
	return -e
}

func (e Int64Value) Plus(other IntValue) IntValue {
	return e + other.(Int64Value)
}

func (e Int64Value) Minus(other IntValue) IntValue {
	return e - other.(Int64Value)
}

func (e Int64Value) Mod(other IntValue) IntValue {
	return e % other.(Int64Value)
}

func (e Int64Value) Mul(other IntValue) IntValue {
	return e * other.(Int64Value)
}

func (e Int64Value) Div(other IntValue) IntValue {
	return e / other.(Int64Value)
}

func (e Int64Value) Less(other IntValue) BoolValue {
	return e < other.(Int64Value)
}

func (e Int64Value) LessEqual(other IntValue) BoolValue {
	return e <= other.(Int64Value)
}

func (e Int64Value) Greater(other IntValue) BoolValue {
	return e > other.(Int64Value)
}

func (e Int64Value) GreaterEqual(other IntValue) BoolValue {
	return e >= other.(Int64Value)
}

// UInt8Value

type UInt8Value uint8

func (UInt8Value) isValue() {}

func (e UInt8Value) IntValue() int {
	return int(e)
}

func (e UInt8Value) Negate() IntValue {
	return -e
}

func (e UInt8Value) Plus(other IntValue) IntValue {
	return e + other.(UInt8Value)
}

func (e UInt8Value) Minus(other IntValue) IntValue {
	return e - other.(UInt8Value)
}

func (e UInt8Value) Mod(other IntValue) IntValue {
	return e % other.(UInt8Value)
}

func (e UInt8Value) Mul(other IntValue) IntValue {
	return e * other.(UInt8Value)
}

func (e UInt8Value) Div(other IntValue) IntValue {
	return e / other.(UInt8Value)
}

func (e UInt8Value) Less(other IntValue) BoolValue {
	return e < other.(UInt8Value)
}

func (e UInt8Value) LessEqual(other IntValue) BoolValue {
	return e <= other.(UInt8Value)
}

func (e UInt8Value) Greater(other IntValue) BoolValue {
	return e > other.(UInt8Value)
}

func (e UInt8Value) GreaterEqual(other IntValue) BoolValue {
	return e >= other.(UInt8Value)
}

// UInt16Value

type UInt16Value uint16

func (UInt16Value) isValue() {}

func (e UInt16Value) IntValue() int {
	return int(e)
}
func (e UInt16Value) Negate() IntValue {
	return -e
}

func (e UInt16Value) Plus(other IntValue) IntValue {
	return e + other.(UInt16Value)
}

func (e UInt16Value) Minus(other IntValue) IntValue {
	return e - other.(UInt16Value)
}

func (e UInt16Value) Mod(other IntValue) IntValue {
	return e % other.(UInt16Value)
}

func (e UInt16Value) Mul(other IntValue) IntValue {
	return e * other.(UInt16Value)
}

func (e UInt16Value) Div(other IntValue) IntValue {
	return e / other.(UInt16Value)
}

func (e UInt16Value) Less(other IntValue) BoolValue {
	return e < other.(UInt16Value)
}

func (e UInt16Value) LessEqual(other IntValue) BoolValue {
	return e <= other.(UInt16Value)
}

func (e UInt16Value) Greater(other IntValue) BoolValue {
	return e > other.(UInt16Value)
}

func (e UInt16Value) GreaterEqual(other IntValue) BoolValue {
	return e >= other.(UInt16Value)
}

// UInt32Value

type UInt32Value uint32

func (UInt32Value) isValue() {}

func (e UInt32Value) IntValue() int {
	return int(e)
}

func (e UInt32Value) Negate() IntValue {
	return -e
}

func (e UInt32Value) Plus(other IntValue) IntValue {
	return e + other.(UInt32Value)
}

func (e UInt32Value) Minus(other IntValue) IntValue {
	return e - other.(UInt32Value)
}

func (e UInt32Value) Mod(other IntValue) IntValue {
	return e % other.(UInt32Value)
}

func (e UInt32Value) Mul(other IntValue) IntValue {
	return e * other.(UInt32Value)
}

func (e UInt32Value) Div(other IntValue) IntValue {
	return e / other.(UInt32Value)
}

func (e UInt32Value) Less(other IntValue) BoolValue {
	return e < other.(UInt32Value)
}

func (e UInt32Value) LessEqual(other IntValue) BoolValue {
	return e <= other.(UInt32Value)
}

func (e UInt32Value) Greater(other IntValue) BoolValue {
	return e > other.(UInt32Value)
}

func (e UInt32Value) GreaterEqual(other IntValue) BoolValue {
	return e >= other.(UInt32Value)
}

// UInt64Value

type UInt64Value uint64

func (UInt64Value) isValue() {}

func (e UInt64Value) IntValue() int {
	return int(e)
}

func (e UInt64Value) Negate() IntValue {
	return -e
}

func (e UInt64Value) Plus(other IntValue) IntValue {
	return e + other.(UInt64Value)
}

func (e UInt64Value) Minus(other IntValue) IntValue {
	return e - other.(UInt64Value)
}

func (e UInt64Value) Mod(other IntValue) IntValue {
	return e % other.(UInt64Value)
}

func (e UInt64Value) Mul(other IntValue) IntValue {
	return e * other.(UInt64Value)
}

func (e UInt64Value) Div(other IntValue) IntValue {
	return e / other.(UInt64Value)
}

func (e UInt64Value) Less(other IntValue) BoolValue {
	return e < other.(UInt64Value)
}

func (e UInt64Value) LessEqual(other IntValue) BoolValue {
	return e <= other.(UInt64Value)
}

func (e UInt64Value) Greater(other IntValue) BoolValue {
	return e > other.(UInt64Value)
}

func (e UInt64Value) GreaterEqual(other IntValue) BoolValue {
	return e >= other.(UInt64Value)
}

// ToValue

// ToValue converts a Go value into an interpreter value
func ToValue(value interface{}) Value {
	// TODO: support more types
	switch value := value.(type) {
	case int8:
		return Int8Value(value)
	case int16:
		return Int16Value(value)
	case int32:
		return Int32Value(value)
	case int64:
		return Int64Value(value)
	case uint8:
		return UInt8Value(value)
	case uint16:
		return UInt16Value(value)
	case uint32:
		return UInt32Value(value)
	case uint64:
		return UInt64Value(value)
	case bool:
		return BoolValue(value)
	}

	panic(fmt.Sprintf("can't convert Go value to value: %#+v", value))
}

func ToValues(inputs []interface{}) []Value {
	var values []Value
	for _, argument := range inputs {
		values = append(
			values,
			ToValue(argument),
		)
	}
	return values
}
