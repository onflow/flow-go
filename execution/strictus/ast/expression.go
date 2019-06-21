package ast

import "fmt"

type Expression interface {
	Element
	isExpression()
}

// BoolExpression

type BoolExpression bool

func (BoolExpression) isExpression() {}

func (e BoolExpression) Accept(v Visitor) Repr {
	return v.VisitBoolExpression(e)
}

// IntExpression

type IntExpression interface {
	Expression
	isIntExpression()
	IntValue() int
	Plus(other IntExpression) IntExpression
	Minus(other IntExpression) IntExpression
	Mod(other IntExpression) IntExpression
	Mul(other IntExpression) IntExpression
	Div(other IntExpression) IntExpression
	Less(other IntExpression) BoolExpression
	LessEqual(other IntExpression) BoolExpression
	Greater(other IntExpression) BoolExpression
	GreaterEqual(other IntExpression) BoolExpression
}

// Int8Expression

type Int8Expression int8

func (Int8Expression) isExpression() {}

func (e Int8Expression) Accept(v Visitor) Repr {
	return v.VisitInt8Expression(e)
}

func (Int8Expression) isIntExpression() {}

func (e Int8Expression) IntValue() int {
	return int(e)
}

func (e Int8Expression) Plus(other IntExpression) IntExpression {
	return e + other.(Int8Expression)
}

func (e Int8Expression) Minus(other IntExpression) IntExpression {
	return e - other.(Int8Expression)
}

func (e Int8Expression) Mod(other IntExpression) IntExpression {
	return e % other.(Int8Expression)
}

func (e Int8Expression) Mul(other IntExpression) IntExpression {
	return e * other.(Int8Expression)
}

func (e Int8Expression) Div(other IntExpression) IntExpression {
	return e / other.(Int8Expression)
}

func (e Int8Expression) Less(other IntExpression) BoolExpression {
	return e < other.(Int8Expression)
}

func (e Int8Expression) LessEqual(other IntExpression) BoolExpression {
	return e <= other.(Int8Expression)
}

func (e Int8Expression) Greater(other IntExpression) BoolExpression {
	return e > other.(Int8Expression)
}

func (e Int8Expression) GreaterEqual(other IntExpression) BoolExpression {
	return e >= other.(Int8Expression)
}

// Int16Expression

type Int16Expression int16

func (Int16Expression) isExpression() {}

func (e Int16Expression) Accept(v Visitor) Repr {
	return v.VisitInt16Expression(e)
}

func (Int16Expression) isIntExpression() {}

func (e Int16Expression) IntValue() int {
	return int(e)
}

func (e Int16Expression) Plus(other IntExpression) IntExpression {
	return e + other.(Int16Expression)
}

func (e Int16Expression) Minus(other IntExpression) IntExpression {
	return e - other.(Int16Expression)
}

func (e Int16Expression) Mod(other IntExpression) IntExpression {
	return e % other.(Int16Expression)
}

func (e Int16Expression) Mul(other IntExpression) IntExpression {
	return e * other.(Int16Expression)
}

func (e Int16Expression) Div(other IntExpression) IntExpression {
	return e / other.(Int16Expression)
}

func (e Int16Expression) Less(other IntExpression) BoolExpression {
	return e < other.(Int16Expression)
}

func (e Int16Expression) LessEqual(other IntExpression) BoolExpression {
	return e <= other.(Int16Expression)
}

func (e Int16Expression) Greater(other IntExpression) BoolExpression {
	return e > other.(Int16Expression)
}

func (e Int16Expression) GreaterEqual(other IntExpression) BoolExpression {
	return e >= other.(Int16Expression)
}

// Int32Expression

type Int32Expression int32

func (Int32Expression) isExpression() {}

func (e Int32Expression) Accept(v Visitor) Repr {
	return v.VisitInt32Expression(e)
}

func (Int32Expression) isIntExpression() {}

func (e Int32Expression) IntValue() int {
	return int(e)
}

func (e Int32Expression) Plus(other IntExpression) IntExpression {
	return e + other.(Int32Expression)
}

func (e Int32Expression) Minus(other IntExpression) IntExpression {
	return e - other.(Int32Expression)
}

func (e Int32Expression) Mod(other IntExpression) IntExpression {
	return e % other.(Int32Expression)
}

func (e Int32Expression) Mul(other IntExpression) IntExpression {
	return e * other.(Int32Expression)
}

func (e Int32Expression) Div(other IntExpression) IntExpression {
	return e / other.(Int32Expression)
}

func (e Int32Expression) Less(other IntExpression) BoolExpression {
	return e < other.(Int32Expression)
}

func (e Int32Expression) LessEqual(other IntExpression) BoolExpression {
	return e <= other.(Int32Expression)
}

func (e Int32Expression) Greater(other IntExpression) BoolExpression {
	return e > other.(Int32Expression)
}

func (e Int32Expression) GreaterEqual(other IntExpression) BoolExpression {
	return e >= other.(Int32Expression)
}

// Int64Expression

type Int64Expression int64

func (Int64Expression) isExpression() {}

func (e Int64Expression) Accept(v Visitor) Repr {
	return v.VisitInt64Expression(e)
}

func (Int64Expression) isIntExpression() {}

func (e Int64Expression) IntValue() int {
	return int(e)
}

func (e Int64Expression) Plus(other IntExpression) IntExpression {
	return e + other.(Int64Expression)
}

func (e Int64Expression) Minus(other IntExpression) IntExpression {
	return e - other.(Int64Expression)
}

func (e Int64Expression) Mod(other IntExpression) IntExpression {
	return e % other.(Int64Expression)
}

func (e Int64Expression) Mul(other IntExpression) IntExpression {
	return e * other.(Int64Expression)
}

func (e Int64Expression) Div(other IntExpression) IntExpression {
	return e / other.(Int64Expression)
}

func (e Int64Expression) Less(other IntExpression) BoolExpression {
	return e < other.(Int64Expression)
}

func (e Int64Expression) LessEqual(other IntExpression) BoolExpression {
	return e <= other.(Int64Expression)
}

func (e Int64Expression) Greater(other IntExpression) BoolExpression {
	return e > other.(Int64Expression)
}

func (e Int64Expression) GreaterEqual(other IntExpression) BoolExpression {
	return e >= other.(Int64Expression)
}

// UInt8Expression

type UInt8Expression uint8

func (UInt8Expression) isExpression() {}

func (e UInt8Expression) Accept(v Visitor) Repr {
	return v.VisitUInt8Expression(e)
}

func (UInt8Expression) isIntExpression() {}

func (e UInt8Expression) IntValue() int {
	return int(e)
}

func (e UInt8Expression) Plus(other IntExpression) IntExpression {
	return e + other.(UInt8Expression)
}

func (e UInt8Expression) Minus(other IntExpression) IntExpression {
	return e - other.(UInt8Expression)
}

func (e UInt8Expression) Mod(other IntExpression) IntExpression {
	return e % other.(UInt8Expression)
}

func (e UInt8Expression) Mul(other IntExpression) IntExpression {
	return e * other.(UInt8Expression)
}

func (e UInt8Expression) Div(other IntExpression) IntExpression {
	return e / other.(UInt8Expression)
}

func (e UInt8Expression) Less(other IntExpression) BoolExpression {
	return e < other.(UInt8Expression)
}

func (e UInt8Expression) LessEqual(other IntExpression) BoolExpression {
	return e <= other.(UInt8Expression)
}

func (e UInt8Expression) Greater(other IntExpression) BoolExpression {
	return e > other.(UInt8Expression)
}

func (e UInt8Expression) GreaterEqual(other IntExpression) BoolExpression {
	return e >= other.(UInt8Expression)
}

// UInt16Expression

type UInt16Expression uint16

func (UInt16Expression) isExpression() {}

func (e UInt16Expression) Accept(v Visitor) Repr {
	return v.VisitUInt16Expression(e)
}

func (UInt16Expression) isIntExpression() {}

func (e UInt16Expression) IntValue() int {
	return int(e)
}

func (e UInt16Expression) Plus(other IntExpression) IntExpression {
	return e + other.(UInt16Expression)
}

func (e UInt16Expression) Minus(other IntExpression) IntExpression {
	return e - other.(UInt16Expression)
}

func (e UInt16Expression) Mod(other IntExpression) IntExpression {
	return e % other.(UInt16Expression)
}

func (e UInt16Expression) Mul(other IntExpression) IntExpression {
	return e * other.(UInt16Expression)
}

func (e UInt16Expression) Div(other IntExpression) IntExpression {
	return e / other.(UInt16Expression)
}

func (e UInt16Expression) Less(other IntExpression) BoolExpression {
	return e < other.(UInt16Expression)
}

func (e UInt16Expression) LessEqual(other IntExpression) BoolExpression {
	return e <= other.(UInt16Expression)
}

func (e UInt16Expression) Greater(other IntExpression) BoolExpression {
	return e > other.(UInt16Expression)
}

func (e UInt16Expression) GreaterEqual(other IntExpression) BoolExpression {
	return e >= other.(UInt16Expression)
}

// UInt32Expression

type UInt32Expression uint32

func (UInt32Expression) isExpression() {}

func (e UInt32Expression) Accept(v Visitor) Repr {
	return v.VisitUInt32Expression(e)
}

func (UInt32Expression) isIntExpression() {}

func (e UInt32Expression) IntValue() int {
	return int(e)
}

func (e UInt32Expression) Plus(other IntExpression) IntExpression {
	return e + other.(UInt32Expression)
}

func (e UInt32Expression) Minus(other IntExpression) IntExpression {
	return e - other.(UInt32Expression)
}

func (e UInt32Expression) Mod(other IntExpression) IntExpression {
	return e % other.(UInt32Expression)
}

func (e UInt32Expression) Mul(other IntExpression) IntExpression {
	return e * other.(UInt32Expression)
}

func (e UInt32Expression) Div(other IntExpression) IntExpression {
	return e / other.(UInt32Expression)
}

func (e UInt32Expression) Less(other IntExpression) BoolExpression {
	return e < other.(UInt32Expression)
}

func (e UInt32Expression) LessEqual(other IntExpression) BoolExpression {
	return e <= other.(UInt32Expression)
}

func (e UInt32Expression) Greater(other IntExpression) BoolExpression {
	return e > other.(UInt32Expression)
}

func (e UInt32Expression) GreaterEqual(other IntExpression) BoolExpression {
	return e >= other.(UInt32Expression)
}

// UInt64Expression

type UInt64Expression uint64

func (UInt64Expression) isExpression() {}

func (e UInt64Expression) Accept(v Visitor) Repr {
	return v.VisitUInt64Expression(e)
}

func (UInt64Expression) isIntExpression() {}

func (e UInt64Expression) IntValue() int {
	return int(e)
}

func (e UInt64Expression) Plus(other IntExpression) IntExpression {
	return e + other.(UInt64Expression)
}

func (e UInt64Expression) Minus(other IntExpression) IntExpression {
	return e - other.(UInt64Expression)
}

func (e UInt64Expression) Mod(other IntExpression) IntExpression {
	return e % other.(UInt64Expression)
}

func (e UInt64Expression) Mul(other IntExpression) IntExpression {
	return e * other.(UInt64Expression)
}

func (e UInt64Expression) Div(other IntExpression) IntExpression {
	return e / other.(UInt64Expression)
}

func (e UInt64Expression) Less(other IntExpression) BoolExpression {
	return e < other.(UInt64Expression)
}

func (e UInt64Expression) LessEqual(other IntExpression) BoolExpression {
	return e <= other.(UInt64Expression)
}

func (e UInt64Expression) Greater(other IntExpression) BoolExpression {
	return e > other.(UInt64Expression)
}

func (e UInt64Expression) GreaterEqual(other IntExpression) BoolExpression {
	return e >= other.(UInt64Expression)
}

// ArrayExpression

type ArrayExpression struct {
	Values []Expression
}

func (ArrayExpression) isExpression() {}

func (e ArrayExpression) Accept(v Visitor) Repr {
	return v.VisitArrayExpression(e)
}

// IdentifierExpression

type IdentifierExpression struct {
	Identifier string
}

func (IdentifierExpression) isExpression() {}

func (e IdentifierExpression) Accept(v Visitor) Repr {
	return v.VisitIdentifierExpression(e)
}

// InvocationExpression

type InvocationExpression struct {
	Expression Expression
	Arguments  []Expression
}

func (InvocationExpression) isExpression() {}

func (e InvocationExpression) Accept(v Visitor) Repr {
	return v.VisitInvocationExpression(e)
}

// AccessExpression

type AccessExpression interface {
	isAccessExpression()
}

// MemberExpression

type MemberExpression struct {
	Expression Expression
	Identifier string
}

func (MemberExpression) isExpression()       {}
func (MemberExpression) isAccessExpression() {}

func (e MemberExpression) Accept(v Visitor) Repr {
	return v.VisitMemberExpression(e)
}

// IndexExpression

type IndexExpression struct {
	Expression Expression
	Index      Expression
}

func (IndexExpression) isExpression()       {}
func (IndexExpression) isAccessExpression() {}

func (e IndexExpression) Accept(v Visitor) Repr {
	return v.VisitIndexExpression(e)
}

// ConditionalExpression

type ConditionalExpression struct {
	Test Expression
	Then Expression
	Else Expression
}

func (ConditionalExpression) isExpression() {}

func (e ConditionalExpression) Accept(v Visitor) Repr {
	return v.VisitConditionalExpression(e)
}

// BinaryExpression

type BinaryExpression struct {
	Operation Operation
	Left      Expression
	Right     Expression
}

func (BinaryExpression) isExpression() {}

func (e BinaryExpression) Accept(v Visitor) Repr {
	return v.VisitBinaryExpression(e)
}

// FunctionExpression

type FunctionExpression struct {
	Parameters []Parameter
	ReturnType Type
	Block      Block
}

func (FunctionExpression) isExpression() {}

func (f FunctionExpression) Accept(visitor Visitor) Repr {
	return visitor.VisitFunctionExpression(f)
}

// ToExpression

// ToExpression converts a Go value into an expression
func ToExpression(value interface{}) Expression {
	// TODO: support more types
	switch value := value.(type) {
	case int8:
		return Int8Expression(value)
	case int16:
		return Int16Expression(value)
	case int32:
		return Int32Expression(value)
	case int64:
		return Int64Expression(value)
	case uint8:
		return UInt8Expression(value)
	case uint16:
		return UInt16Expression(value)
	case uint32:
		return UInt32Expression(value)
	case uint64:
		return UInt64Expression(value)
	case bool:
		return BoolExpression(value)
	}

	panic(fmt.Sprintf("can't convert Go value to expression: %#+v", value))
}
