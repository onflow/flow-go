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

func (e *BoolExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *BoolExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitBoolExpression(e)
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

func (e *NilExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *NilExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitNilExpression(e)
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

func (e *StringExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *StringExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitStringExpression(e)
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

func (e *IntExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *IntExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitIntExpression(e)
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

func (e *ArrayExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *ArrayExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitArrayExpression(e)
}

// DictionaryExpression

type DictionaryExpression struct {
	Entries  []Entry
	StartPos Position
	EndPos   Position
}

func (e *DictionaryExpression) String() string {
	var builder strings.Builder
	builder.WriteString("{")
	for i, entry := range e.Entries {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(entry.Key.String())
		builder.WriteString(": ")
		builder.WriteString(entry.Value.String())
	}
	builder.WriteString("}")
	return builder.String()
}

func (e *DictionaryExpression) StartPosition() Position {
	return e.StartPos
}

func (e *DictionaryExpression) EndPosition() Position {
	return e.EndPos
}

func (*DictionaryExpression) isIfStatementTest() {}

func (*DictionaryExpression) isExpression() {}

func (e *DictionaryExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *DictionaryExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitDictionaryExpression(e)
}

type Entry struct {
	Key   Expression
	Value Expression
}

// IdentifierExpression

type IdentifierExpression struct {
	Identifier
}

func (e *IdentifierExpression) String() string {
	return e.Identifier.Identifier
}

func (*IdentifierExpression) isIfStatementTest() {}

func (*IdentifierExpression) isExpression() {}

func (e *IdentifierExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *IdentifierExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitIdentifierExpression(e)
}

// Arguments

type Arguments []*Argument

func (args Arguments) String() string {
	var builder strings.Builder
	builder.WriteString("(")
	for i, argument := range args {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(argument.String())
	}
	builder.WriteString(")")
	return builder.String()
}

// InvocationExpression

type InvocationExpression struct {
	InvokedExpression Expression
	Arguments         Arguments
	EndPos            Position
}

func (e *InvocationExpression) String() string {
	var builder strings.Builder
	builder.WriteString(e.InvokedExpression.String())
	builder.WriteString(e.Arguments.String())
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

func (e *InvocationExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *InvocationExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitInvocationExpression(e)
}

// AccessExpression

type AccessExpression interface {
	isAccessExpression()
}

// MemberExpression

type MemberExpression struct {
	Expression Expression
	Identifier Identifier
}

func (e *MemberExpression) String() string {
	return fmt.Sprintf(
		"%s.%s",
		e.Expression, e.Identifier,
	)
}

func (e *MemberExpression) StartPosition() Position {
	return e.Expression.StartPosition()
}

func (e *MemberExpression) EndPosition() Position {
	return e.Identifier.EndPosition()
}

func (*MemberExpression) isIfStatementTest() {}

func (*MemberExpression) isExpression() {}

func (*MemberExpression) isAccessExpression() {}

func (e *MemberExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *MemberExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitMemberExpression(e)
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

func (e *IndexExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *IndexExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitIndexExpression(e)
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

func (e *ConditionalExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *ConditionalExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitConditionalExpression(e)
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

func (e *UnaryExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *UnaryExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitUnaryExpression(e)
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

func (e *BinaryExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *BinaryExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitBinaryExpression(e)
}

// FunctionExpression

type FunctionExpression struct {
	Parameters           []*Parameter
	ReturnTypeAnnotation *TypeAnnotation
	FunctionBlock        *FunctionBlock
	StartPos             Position
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
	return e.AcceptExp(visitor)
}

func (e *FunctionExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitFunctionExpression(e)
}

// FailableDowncastExpression

type FailableDowncastExpression struct {
	Expression     Expression
	TypeAnnotation *TypeAnnotation
}

func (e *FailableDowncastExpression) String() string {
	return fmt.Sprintf(
		"(%s as? %s)",
		e.Expression, e.TypeAnnotation,
	)
}

func (e *FailableDowncastExpression) StartPosition() Position {
	return e.Expression.StartPosition()
}

func (e *FailableDowncastExpression) EndPosition() Position {
	return e.TypeAnnotation.EndPosition()
}

func (*FailableDowncastExpression) isIfStatementTest() {}

func (*FailableDowncastExpression) isExpression() {}

func (e *FailableDowncastExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *FailableDowncastExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitFailableDowncastExpression(e)
}

// CreateExpression

type CreateExpression struct {
	Identifier Identifier
	Arguments  Arguments
	StartPos   Position
	EndPos     Position
}

func (e *CreateExpression) String() string {
	return fmt.Sprintf(
		"(create %s%s)",
		e.Identifier, e.Arguments,
	)
}

func (e *CreateExpression) StartPosition() Position {
	return e.StartPos
}

func (e *CreateExpression) EndPosition() Position {
	return e.EndPos
}

func (*CreateExpression) isIfStatementTest() {}

func (*CreateExpression) isExpression() {}

func (e *CreateExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *CreateExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitCreateExpression(e)
}

// DestroyExpression

type DestroyExpression struct {
	Expression Expression
	StartPos   Position
}

func (e *DestroyExpression) String() string {
	return fmt.Sprintf(
		"(destroy %s)",
		e.Expression.String(),
	)
}

func (e *DestroyExpression) StartPosition() Position {
	return e.StartPos
}

func (e *DestroyExpression) EndPosition() Position {
	return e.Expression.EndPosition()
}

func (*DestroyExpression) isIfStatementTest() {}

func (*DestroyExpression) isExpression() {}

func (e *DestroyExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *DestroyExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitDestroyExpression(e)
}
