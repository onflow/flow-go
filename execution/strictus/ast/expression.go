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

func (e BoolExpression) GetStartPosition() Position {
	return e.Position
}

func (e BoolExpression) GetEndPosition() Position {
	return e.Position
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

func (e IntExpression) GetStartPosition() Position {
	return e.Position
}

func (e IntExpression) GetEndPosition() Position {
	return e.Position
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

func (e ArrayExpression) GetStartPosition() Position {
	return e.StartPosition
}

func (e ArrayExpression) GetEndPosition() Position {
	return e.EndPosition
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

func (e IdentifierExpression) GetStartPosition() Position {
	return e.Position
}

func (e IdentifierExpression) GetEndPosition() Position {
	return e.Position
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

func (e InvocationExpression) GetStartPosition() Position {
	return e.StartPosition
}

func (e InvocationExpression) GetEndPosition() Position {
	return e.EndPosition
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

func (e MemberExpression) GetStartPosition() Position {
	return e.StartPosition
}

func (e MemberExpression) GetEndPosition() Position {
	return e.EndPosition
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

func (e IndexExpression) GetStartPosition() Position {
	return e.StartPosition
}

func (e IndexExpression) GetEndPosition() Position {
	return e.EndPosition
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

func (e ConditionalExpression) GetStartPosition() Position {
	return e.StartPosition
}

func (e ConditionalExpression) GetEndPosition() Position {
	return e.EndPosition
}

func (ConditionalExpression) isExpression() {}

func (e ConditionalExpression) Accept(v Visitor) Repr {
	return v.VisitConditionalExpression(e)
}

// UnaryExpression

type UnaryExpression struct {
	Operation     Operation
	Expression    Expression
	StartPosition Position
	EndPosition   Position
}

func (e UnaryExpression) GetStartPosition() Position {
	return e.StartPosition
}

func (e UnaryExpression) GetEndPosition() Position {
	return e.EndPosition
}

func (UnaryExpression) isExpression() {}

func (e UnaryExpression) Accept(v Visitor) Repr {
	return v.VisitUnaryExpression(e)
}

// BinaryExpression

type BinaryExpression struct {
	Operation     Operation
	Left          Expression
	Right         Expression
	StartPosition Position
	EndPosition   Position
}

func (e BinaryExpression) GetStartPosition() Position {
	return e.StartPosition
}

func (e BinaryExpression) GetEndPosition() Position {
	return e.EndPosition
}

func (BinaryExpression) isExpression() {}

func (e BinaryExpression) Accept(v Visitor) Repr {
	return v.VisitBinaryExpression(e)
}

// FunctionExpression

type FunctionExpression struct {
	Parameters    []Parameter
	ReturnType    Type
	Block         Block
	StartPosition Position
	EndPosition   Position
}

func (e FunctionExpression) GetStartPosition() Position {
	return e.StartPosition
}

func (e FunctionExpression) GetEndPosition() Position {
	return e.EndPosition
}

func (FunctionExpression) isExpression() {}

func (e FunctionExpression) Accept(visitor Visitor) Repr {
	return visitor.VisitFunctionExpression(e)
}
