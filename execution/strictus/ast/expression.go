package ast

import "fmt"

type Expression interface {
	Element
	isExpression()
}

// BoolExpression

type BoolExpression struct {
	Value bool
}

func (e BoolExpression) Accept(v Visitor) Repr {
	return v.VisitBoolExpression(e)
}

func (BoolExpression) isExpression() {}

// IntExpression

type IntExpression struct {
	Value int64
}

func (IntExpression) isExpression() {}

func (e IntExpression) Accept(v Visitor) Repr {
	return v.VisitIntExpression(e)
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

// MemberExpression

type MemberExpression struct {
	Expression Expression
	Identifier string
}

func (MemberExpression) isExpression() {}

func (e MemberExpression) Accept(v Visitor) Repr {
	return v.VisitMemberExpression(e)
}

// IndexExpression

type IndexExpression struct {
	Expression Expression
	Index      Expression
}

func (IndexExpression) isExpression() {}

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
	case int:
		return IntExpression{Value: int64(value)}
	case int64:
		return IntExpression{Value: value}
	case bool:
		return BoolExpression{Value: value}
	}

	panic(fmt.Sprintf("can't convert Go value to expression: %#+v", value))
}
