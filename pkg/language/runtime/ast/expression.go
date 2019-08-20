package ast

import (
	"fmt"
	"math/big"
	"strings"
)

const NilConstant = "nil"

type Expression interface {
	Element
	fmt.Stringer
	IfStatementTest
	isExpression()
	AcceptExp(ExpressionVisitor) Repr
}

// BoolExpression

type BoolExpression struct {
	Value    bool
	StartPos Position
	EndPos   Position
}

func (e *BoolExpression) String() string {
	if e.Value {
		return "true"
	} else {
		return "false"
	}
}

func (e *BoolExpression) StartPosition() Position {
	return e.StartPos
}

func (e *BoolExpression) EndPosition() Position {
	return e.EndPos
}

func (*BoolExpression) isIfStatementTest() {}

func (*BoolExpression) isExpression() {}

func (e *BoolExpression) Accept(v Visitor) Repr {
	return v.VisitBoolExpression(e)
}

func (e *BoolExpression) AcceptExp(v ExpressionVisitor) Repr {
	return v.VisitBoolExpression(e)
}

// NilExpression

type NilExpression struct {
	Pos Position
}

func (e *NilExpression) String() string {
	return NilConstant
}

func (e *NilExpression) StartPosition() Position {
	return e.Pos
}

func (e *NilExpression) EndPosition() Position {
	return e.Pos.Shifted(len(NilConstant) - 1)
}

func (*NilExpression) isIfStatementTest() {}

func (*NilExpression) isExpression() {}

func (e *NilExpression) Accept(v Visitor) Repr {
	return v.VisitNilExpression(e)
}

func (e *NilExpression) AcceptExp(v ExpressionVisitor) Repr {
	return v.VisitNilExpression(e)
}

// StringExpression

type StringExpression struct {
	Value    string
	StartPos Position
	EndPos   Position
}

func (e *StringExpression) String() string {
	// TODO:
	return ""
}

func (e *StringExpression) StartPosition() Position {
	return e.StartPos
}

func (e *StringExpression) EndPosition() Position {
	return e.EndPos
}

func (*StringExpression) isIfStatementTest() {}

func (*StringExpression) isExpression() {}

func (e *StringExpression) Accept(v Visitor) Repr {
	return v.VisitStringExpression(e)
}

func (e *StringExpression) AcceptExp(v ExpressionVisitor) Repr {
	return v.VisitStringExpression(e)
}

// IntExpression

type IntExpression struct {
	Value    *big.Int
	StartPos Position
	EndPos   Position
}

func (e *IntExpression) String() string {
	return e.Value.String()
}

func (e *IntExpression) StartPosition() Position {
	return e.StartPos
}

func (e *IntExpression) EndPosition() Position {
	return e.EndPos
}

func (*IntExpression) isIfStatementTest() {}

func (*IntExpression) isExpression() {}

func (e *IntExpression) Accept(v Visitor) Repr {
	return v.VisitIntExpression(e)
}

func (e *IntExpression) AcceptExp(v ExpressionVisitor) Repr {
	return v.VisitIntExpression(e)
}

// ArrayExpression

type ArrayExpression struct {
	Values   []Expression
	StartPos Position
	EndPos   Position
}

func (e *ArrayExpression) String() string {
	var builder strings.Builder
	builder.WriteString("[")
	for i, value := range e.Values {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(value.String())
	}
	builder.WriteString("]")
	return builder.String()
}

func (e *ArrayExpression) StartPosition() Position {
	return e.StartPos
}

func (e *ArrayExpression) EndPosition() Position {
	return e.EndPos
}

func (*ArrayExpression) isIfStatementTest() {}

func (*ArrayExpression) isExpression() {}

func (e *ArrayExpression) Accept(v Visitor) Repr {
	return v.VisitArrayExpression(e)
}

func (e *ArrayExpression) AcceptExp(v ExpressionVisitor) Repr {
	return v.VisitArrayExpression(e)
}

// IdentifierExpression

type IdentifierExpression struct {
	Identifier string
	StartPos   Position
	EndPos     Position
}

func (e *IdentifierExpression) String() string {
	return e.Identifier
}

func (e *IdentifierExpression) StartPosition() Position {
	return e.StartPos
}

func (e *IdentifierExpression) EndPosition() Position {
	return e.EndPos
}

func (*IdentifierExpression) isIfStatementTest() {}

func (*IdentifierExpression) isExpression() {}

func (e *IdentifierExpression) Accept(v Visitor) Repr {
	return v.VisitIdentifierExpression(e)
}

func (e *IdentifierExpression) AcceptExp(v ExpressionVisitor) Repr {
	return v.VisitIdentifierExpression(e)
}

// InvocationExpression

type InvocationExpression struct {
	InvokedExpression Expression
	Arguments         []*Argument
	EndPos            Position
}

func (e *InvocationExpression) String() string {
	var builder strings.Builder
	builder.WriteString(e.InvokedExpression.String())
	builder.WriteString("(")
	for i, argument := range e.Arguments {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(argument.String())
	}
	builder.WriteString(")")
	return builder.String()
}

func (e *InvocationExpression) StartPosition() Position {
	return e.InvokedExpression.StartPosition()
}

func (e *InvocationExpression) EndPosition() Position {
	return e.EndPos
}

func (*InvocationExpression) isIfStatementTest() {}

func (*InvocationExpression) isExpression() {}

func (e *InvocationExpression) Accept(v Visitor) Repr {
	return v.VisitInvocationExpression(e)
}

func (e *InvocationExpression) AcceptExp(v ExpressionVisitor) Repr {
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
	StartPos   Position
	EndPos     Position
}

func (e *MemberExpression) String() string {
	return fmt.Sprintf(
		"%s.%s",
		e.Expression, e.Identifier,
	)
}

func (e *MemberExpression) StartPosition() Position {
	return e.StartPos
}

func (e *MemberExpression) EndPosition() Position {
	return e.EndPos
}

func (*MemberExpression) isIfStatementTest() {}

func (*MemberExpression) isExpression() {}

func (*MemberExpression) isAccessExpression() {}

func (e *MemberExpression) Accept(v Visitor) Repr {
	return v.VisitMemberExpression(e)
}

func (e *MemberExpression) AcceptExp(v ExpressionVisitor) Repr {
	return v.VisitMemberExpression(e)
}

// IndexExpression

type IndexExpression struct {
	Expression Expression
	Index      Expression
	StartPos   Position
	EndPos     Position
}

func (e *IndexExpression) String() string {
	return fmt.Sprintf(
		"%s[%s]",
		e.Expression, e.Index,
	)
}

func (e *IndexExpression) StartPosition() Position {
	return e.StartPos
}

func (e *IndexExpression) EndPosition() Position {
	return e.EndPos
}

func (*IndexExpression) isIfStatementTest() {}

func (*IndexExpression) isExpression() {}

func (*IndexExpression) isAccessExpression() {}

func (e *IndexExpression) Accept(v Visitor) Repr {
	return v.VisitIndexExpression(e)
}

func (e *IndexExpression) AcceptExp(v ExpressionVisitor) Repr {
	return v.VisitIndexExpression(e)
}

// ConditionalExpression

type ConditionalExpression struct {
	Test Expression
	Then Expression
	Else Expression
}

func (e *ConditionalExpression) String() string {
	return fmt.Sprintf(
		"(%s ? %s : %s)",
		e.Test, e.Then, e.Else,
	)
}

func (e *ConditionalExpression) StartPosition() Position {
	return e.Test.StartPosition()
}

func (e *ConditionalExpression) EndPosition() Position {
	return e.Else.EndPosition()
}

func (*ConditionalExpression) isIfStatementTest() {}

func (*ConditionalExpression) isExpression() {}

func (e *ConditionalExpression) Accept(v Visitor) Repr {
	return v.VisitConditionalExpression(e)
}

func (e *ConditionalExpression) AcceptExp(v ExpressionVisitor) Repr {
	return v.VisitConditionalExpression(e)
}

// UnaryExpression

type UnaryExpression struct {
	Operation  Operation
	Expression Expression
	StartPos   Position
	EndPos     Position
}

func (e *UnaryExpression) String() string {
	return fmt.Sprintf(
		"%s%s",
		e.Operation.Symbol(), e.Expression,
	)
}

func (e *UnaryExpression) StartPosition() Position {
	return e.StartPos
}

func (e *UnaryExpression) EndPosition() Position {
	return e.EndPos
}

func (*UnaryExpression) isIfStatementTest() {}

func (*UnaryExpression) isExpression() {}

func (e *UnaryExpression) Accept(v Visitor) Repr {
	return v.VisitUnaryExpression(e)
}

func (e *UnaryExpression) AcceptExp(v ExpressionVisitor) Repr {
	return v.VisitUnaryExpression(e)
}

// BinaryExpression

type BinaryExpression struct {
	Operation Operation
	Left      Expression
	Right     Expression
}

func (e *BinaryExpression) String() string {
	return fmt.Sprintf(
		"(%s %s %s)",
		e.Left, e.Operation.Symbol(), e.Right,
	)
}

func (e *BinaryExpression) StartPosition() Position {
	return e.Left.StartPosition()
}

func (e *BinaryExpression) EndPosition() Position {
	return e.Right.EndPosition()
}

func (*BinaryExpression) isIfStatementTest() {}

func (*BinaryExpression) isExpression() {}

func (e *BinaryExpression) Accept(v Visitor) Repr {
	return v.VisitBinaryExpression(e)
}

func (e *BinaryExpression) AcceptExp(v ExpressionVisitor) Repr {
	return v.VisitBinaryExpression(e)
}

// FunctionExpression

type FunctionExpression struct {
	Parameters    []*Parameter
	ReturnType    Type
	FunctionBlock *FunctionBlock
	StartPos      Position
}

func (e *FunctionExpression) String() string {
	// TODO:
	return "..."
}

func (e *FunctionExpression) StartPosition() Position {
	return e.StartPos
}

func (e *FunctionExpression) EndPosition() Position {
	return e.FunctionBlock.EndPosition()
}

func (*FunctionExpression) isIfStatementTest() {}

func (*FunctionExpression) isExpression() {}

func (e *FunctionExpression) Accept(visitor Visitor) Repr {
	return visitor.VisitFunctionExpression(e)
}

func (e *FunctionExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitFunctionExpression(e)
}
