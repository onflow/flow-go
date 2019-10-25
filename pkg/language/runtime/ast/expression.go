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

// TargetExpression

type TargetExpression interface {
	isTargetExpression()
}

// BoolExpression

type BoolExpression struct {
	Value bool
	Range
}

func (*BoolExpression) isExpression() {}

func (*BoolExpression) isIfStatementTest() {}

func (e *BoolExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *BoolExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitBoolExpression(e)
}

func (e *BoolExpression) String() string {
	if e.Value {
		return "true"
	} else {
		return "false"
	}
}

// NilExpression

type NilExpression struct {
	Pos Position
}

func (*NilExpression) isExpression() {}

func (*NilExpression) isIfStatementTest() {}

func (e *NilExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *NilExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitNilExpression(e)
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

// StringExpression

type StringExpression struct {
	Value string
	Range
}

func (*StringExpression) isExpression() {}

func (*StringExpression) isIfStatementTest() {}

func (e *StringExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *StringExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitStringExpression(e)
}

func (e *StringExpression) String() string {
	// TODO:
	return ""
}

// IntExpression

type IntExpression struct {
	Value *big.Int
	Range
}

func (*IntExpression) isExpression() {}

func (*IntExpression) isIfStatementTest() {}

func (e *IntExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *IntExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitIntExpression(e)
}

func (e *IntExpression) String() string {
	return e.Value.String()
}

// ArrayExpression

type ArrayExpression struct {
	Values []Expression
	Range
}

func (*ArrayExpression) isExpression() {}

func (*ArrayExpression) isIfStatementTest() {}

func (e *ArrayExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *ArrayExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitArrayExpression(e)
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

// DictionaryExpression

type DictionaryExpression struct {
	Entries []Entry
	Range
}

func (*DictionaryExpression) isExpression() {}

func (*DictionaryExpression) isIfStatementTest() {}

func (e *DictionaryExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *DictionaryExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitDictionaryExpression(e)
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

type Entry struct {
	Key   Expression
	Value Expression
}

// IdentifierExpression

type IdentifierExpression struct {
	Identifier
}

func (*IdentifierExpression) isExpression() {}

func (*IdentifierExpression) isTargetExpression() {}

func (*IdentifierExpression) isIfStatementTest() {}

func (e *IdentifierExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *IdentifierExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitIdentifierExpression(e)
}

func (e *IdentifierExpression) String() string {
	return e.Identifier.Identifier
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

func (*InvocationExpression) isExpression() {}

func (*InvocationExpression) isIfStatementTest() {}

func (e *InvocationExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *InvocationExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitInvocationExpression(e)
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

// AccessExpression

type AccessExpression interface {
	isAccessExpression()
	AccessedExpression() Expression
}

// MemberExpression

type MemberExpression struct {
	Expression Expression
	Identifier Identifier
}

func (*MemberExpression) isExpression() {}

func (*MemberExpression) isTargetExpression() {}

func (*MemberExpression) isIfStatementTest() {}

func (*MemberExpression) isAccessExpression() {}

func (e *MemberExpression) AccessedExpression() Expression {
	return e.Expression
}

func (e *MemberExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *MemberExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitMemberExpression(e)
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

// IndexExpression

type IndexExpression struct {
	TargetExpression Expression
	// only IndexingExpression or IndexingType is set
	IndexingExpression Expression
	IndexingType       Type
	Range
}

func (*IndexExpression) isExpression() {}

func (*IndexExpression) isTargetExpression() {}

func (*IndexExpression) isIfStatementTest() {}

func (*IndexExpression) isAccessExpression() {}

func (e *IndexExpression) AccessedExpression() Expression {
	return e.TargetExpression
}

func (e *IndexExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *IndexExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitIndexExpression(e)
}
func (e *IndexExpression) String() string {
	var indexString string
	if e.IndexingExpression != nil {
		indexString = e.IndexingExpression.String()
	} else {
		indexString = e.IndexingType.String()
	}

	return fmt.Sprintf(
		"%s[%s]",
		e.TargetExpression, indexString,
	)
}

// ConditionalExpression

type ConditionalExpression struct {
	Test Expression
	Then Expression
	Else Expression
}

func (*ConditionalExpression) isExpression() {}

func (*ConditionalExpression) isIfStatementTest() {}

func (e *ConditionalExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *ConditionalExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitConditionalExpression(e)
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

// UnaryExpression

type UnaryExpression struct {
	Operation  Operation
	Expression Expression
	Range
}

func (*UnaryExpression) isExpression() {}

func (*UnaryExpression) isIfStatementTest() {}

func (e *UnaryExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *UnaryExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitUnaryExpression(e)
}

func (e *UnaryExpression) String() string {
	return fmt.Sprintf(
		"%s%s",
		e.Operation.Symbol(), e.Expression,
	)
}

// BinaryExpression

type BinaryExpression struct {
	Operation Operation
	Left      Expression
	Right     Expression
}

func (*BinaryExpression) isExpression() {}

func (*BinaryExpression) isIfStatementTest() {}

func (e *BinaryExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *BinaryExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitBinaryExpression(e)
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

// FunctionExpression

type FunctionExpression struct {
	ParameterList        *ParameterList
	ReturnTypeAnnotation *TypeAnnotation
	FunctionBlock        *FunctionBlock
	StartPos             Position
}

func (*FunctionExpression) isExpression() {}

func (*FunctionExpression) isIfStatementTest() {}

func (e *FunctionExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *FunctionExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitFunctionExpression(e)
}

func (e *FunctionExpression) String() string {
	// TODO:
	return "func ..."
}

func (e *FunctionExpression) StartPosition() Position {
	return e.StartPos
}

func (e *FunctionExpression) EndPosition() Position {
	return e.FunctionBlock.EndPosition()
}

// FailableDowncastExpression

type FailableDowncastExpression struct {
	Expression     Expression
	TypeAnnotation *TypeAnnotation
}

func (*FailableDowncastExpression) isExpression() {}

func (*FailableDowncastExpression) isIfStatementTest() {}

func (e *FailableDowncastExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *FailableDowncastExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitFailableDowncastExpression(e)
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

// CreateExpression

type CreateExpression struct {
	InvocationExpression *InvocationExpression
	StartPos             Position
}

func (*CreateExpression) isExpression() {}

func (*CreateExpression) isIfStatementTest() {}

func (e *CreateExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *CreateExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitCreateExpression(e)
}

func (e *CreateExpression) String() string {
	return fmt.Sprintf(
		"(create %s)",
		e.InvocationExpression,
	)
}

func (e *CreateExpression) StartPosition() Position {
	return e.StartPos
}

func (e *CreateExpression) EndPosition() Position {
	return e.InvocationExpression.EndPos
}

// DestroyExpression

type DestroyExpression struct {
	Expression Expression
	StartPos   Position
}

func (*DestroyExpression) isExpression() {}

func (*DestroyExpression) isIfStatementTest() {}

func (e *DestroyExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *DestroyExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitDestroyExpression(e)
}

func (e *DestroyExpression) String() string {
	return fmt.Sprintf(
		"(destroy %s)",
		e.Expression,
	)
}

func (e *DestroyExpression) StartPosition() Position {
	return e.StartPos
}

func (e *DestroyExpression) EndPosition() Position {
	return e.Expression.EndPosition()
}

// ReferenceExpression

type ReferenceExpression struct {
	Expression Expression
	Type       Type
	StartPos   Position
}

func (*ReferenceExpression) isExpression() {}

func (*ReferenceExpression) isIfStatementTest() {}

func (e *ReferenceExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *ReferenceExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitReferenceExpression(e)
}

func (e *ReferenceExpression) String() string {
	return fmt.Sprintf(
		"(&%s as %s)",
		e.Expression,
		e.Type,
	)
}

func (e *ReferenceExpression) StartPosition() Position {
	return e.StartPos
}

func (e *ReferenceExpression) EndPosition() Position {
	return e.Type.EndPosition()
}
