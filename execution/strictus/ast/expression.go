package ast

import "math/big"

type Expression interface {
	Element
	isExpression()
}

// BoolExpression

type BoolExpression struct {
	Value bool
	Pos   *Position
}

func (e *BoolExpression) StartPosition() *Position {
	return e.Pos
}

func (e *BoolExpression) EndPosition() *Position {
	return e.Pos
}

func (*BoolExpression) isExpression() {}

func (e *BoolExpression) Accept(v Visitor) Repr {
	return v.VisitBoolExpression(e)
}

// IntExpression

type IntExpression struct {
	Value *big.Int
	Pos   *Position
}

func (e *IntExpression) StartPosition() *Position {
	return e.Pos
}

func (e *IntExpression) EndPosition() *Position {
	return e.Pos
}

func (*IntExpression) isExpression() {}

func (e *IntExpression) Accept(v Visitor) Repr {
	return v.VisitIntExpression(e)
}

// ArrayExpression

type ArrayExpression struct {
	Values   []Expression
	StartPos *Position
	EndPos   *Position
}

func (e *ArrayExpression) StartPosition() *Position {
	return e.StartPos
}

func (e *ArrayExpression) EndPosition() *Position {
	return e.EndPos
}

func (*ArrayExpression) isExpression() {}

func (e *ArrayExpression) Accept(v Visitor) Repr {
	return v.VisitArrayExpression(e)
}

// IdentifierExpression

type IdentifierExpression struct {
	Identifier string
	Pos        *Position
}

func (e *IdentifierExpression) StartPosition() *Position {
	return e.Pos
}

func (e *IdentifierExpression) EndPosition() *Position {
	return e.Pos
}

func (*IdentifierExpression) isExpression() {}

func (e *IdentifierExpression) Accept(v Visitor) Repr {
	return v.VisitIdentifierExpression(e)
}

// InvocationExpression

type InvocationExpression struct {
	Expression Expression
	Arguments  []Expression
	StartPos   *Position
	EndPos     *Position
}

func (e *InvocationExpression) StartPosition() *Position {
	return e.StartPos
}

func (e *InvocationExpression) EndPosition() *Position {
	return e.EndPos
}

func (*InvocationExpression) isExpression() {}

func (e *InvocationExpression) Accept(v Visitor) Repr {
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
	StartPos   *Position
	EndPos     *Position
}

func (e *MemberExpression) StartPosition() *Position {
	return e.StartPos
}

func (e *MemberExpression) EndPosition() *Position {
	return e.EndPos
}

func (*MemberExpression) isExpression()       {}
func (*MemberExpression) isAccessExpression() {}

func (e *MemberExpression) Accept(v Visitor) Repr {
	return v.VisitMemberExpression(e)
}

// IndexExpression

type IndexExpression struct {
	Expression Expression
	Index      Expression
	StartPos   *Position
	EndPos     *Position
}

func (e *IndexExpression) StartPosition() *Position {
	return e.StartPos
}

func (e *IndexExpression) EndPosition() *Position {
	return e.EndPos
}

func (*IndexExpression) isExpression()       {}
func (*IndexExpression) isAccessExpression() {}

func (e *IndexExpression) Accept(v Visitor) Repr {
	return v.VisitIndexExpression(e)
}

// ConditionalExpression

type ConditionalExpression struct {
	Test     Expression
	Then     Expression
	Else     Expression
	StartPos *Position
	EndPos   *Position
}

func (e *ConditionalExpression) StartPosition() *Position {
	return e.StartPos
}

func (e *ConditionalExpression) EndPosition() *Position {
	return e.EndPos
}

func (*ConditionalExpression) isExpression() {}

func (e *ConditionalExpression) Accept(v Visitor) Repr {
	return v.VisitConditionalExpression(e)
}

// UnaryExpression

type UnaryExpression struct {
	Operation  Operation
	Expression Expression
	StartPos   *Position
	EndPos     *Position
}

func (e *UnaryExpression) StartPosition() *Position {
	return e.StartPos
}

func (e *UnaryExpression) EndPosition() *Position {
	return e.EndPos
}

func (*UnaryExpression) isExpression() {}

func (e *UnaryExpression) Accept(v Visitor) Repr {
	return v.VisitUnaryExpression(e)
}

// BinaryExpression

type BinaryExpression struct {
	Operation Operation
	Left      Expression
	Right     Expression
	StartPos  *Position
	EndPos    *Position
}

func (e *BinaryExpression) StartPosition() *Position {
	return e.StartPos
}

func (e *BinaryExpression) EndPosition() *Position {
	return e.EndPos
}

func (*BinaryExpression) isExpression() {}

func (e *BinaryExpression) Accept(v Visitor) Repr {
	return v.VisitBinaryExpression(e)
}

// FunctionExpression

type FunctionExpression struct {
	Parameters []*Parameter
	ReturnType Type
	Block      *Block
	StartPos   *Position
	EndPos     *Position
}

func (e *FunctionExpression) StartPosition() *Position {
	return e.StartPos
}

func (e *FunctionExpression) EndPosition() *Position {
	return e.EndPos
}

func (*FunctionExpression) isExpression() {}

func (e *FunctionExpression) Accept(visitor Visitor) Repr {
	return visitor.VisitFunctionExpression(e)
}
