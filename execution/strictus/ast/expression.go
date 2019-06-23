package ast

import "math/big"

type Expression interface {
	Element
	isExpression()
}

// BoolExpression

type BoolExpression struct {
	Value    bool
	Position Position
}

func (BoolExpression) isExpression() {}

func (e BoolExpression) Accept(v Visitor) Repr {
	return v.VisitBoolExpression(e)
}

// IntExpression

type IntExpression struct {
	Value    *big.Int
	Position Position
}

func (IntExpression) isExpression() {}

func (e IntExpression) Accept(v Visitor) Repr {
	return v.VisitIntExpression(e)
}

// ArrayExpression

type ArrayExpression struct {
	Values        []Expression
	StartPosition Position
	EndPosition   Position
}

func (ArrayExpression) isExpression() {}

func (e ArrayExpression) Accept(v Visitor) Repr {
	return v.VisitArrayExpression(e)
}

// IdentifierExpression

type IdentifierExpression struct {
	Identifier string
	Position   Position
}

func (IdentifierExpression) isExpression() {}

func (e IdentifierExpression) Accept(v Visitor) Repr {
	return v.VisitIdentifierExpression(e)
}

// InvocationExpression

type InvocationExpression struct {
	Expression    Expression
	Arguments     []Expression
	StartPosition Position
	EndPosition   Position
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
	Expression    Expression
	Identifier    string
	StartPosition Position
	EndPosition   Position
}

func (MemberExpression) isExpression()       {}
func (MemberExpression) isAccessExpression() {}

func (e MemberExpression) Accept(v Visitor) Repr {
	return v.VisitMemberExpression(e)
}

// IndexExpression

type IndexExpression struct {
	Expression    Expression
	Index         Expression
	StartPosition Position
	EndPosition   Position
}

func (IndexExpression) isExpression()       {}
func (IndexExpression) isAccessExpression() {}

func (e IndexExpression) Accept(v Visitor) Repr {
	return v.VisitIndexExpression(e)
}

// ConditionalExpression

type ConditionalExpression struct {
	Test          Expression
	Then          Expression
	Else          Expression
	StartPosition Position
	EndPosition   Position
}

func (ConditionalExpression) isExpression() {}

func (e ConditionalExpression) Accept(v Visitor) Repr {
	return v.VisitConditionalExpression(e)
}

// UnaryExpression

type UnaryExpression struct {
	Operation  Operation
	Expression Expression
}

func (UnaryExpression) isExpression() {}

func (e UnaryExpression) Accept(v Visitor) Repr {
	return v.VisitUnaryExpression(e)
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
