/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
	}
	return "false"
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

// IntegerExpression

type IntegerExpression struct {
	Value *big.Int
	Base  int
	Range
}

func (*IntegerExpression) isExpression() {}

func (*IntegerExpression) isIfStatementTest() {}

func (e *IntegerExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *IntegerExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitIntegerExpression(e)
}

func (e *IntegerExpression) String() string {
	return e.Value.String()
}

// FixedPointExpression

type FixedPointExpression struct {
	Negative        bool
	UnsignedInteger *big.Int
	Fractional      *big.Int
	Scale           uint
	Range
}

func (*FixedPointExpression) isExpression() {}

func (*FixedPointExpression) isIfStatementTest() {}

func (e *FixedPointExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *FixedPointExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitFixedPointExpression(e)
}

func (e *FixedPointExpression) String() string {
	var builder strings.Builder
	if e.Negative {
		builder.WriteRune('-')
	}
	builder.WriteString(e.UnsignedInteger.String())
	builder.WriteRune('.')
	fractional := e.Fractional.String()
	for i := uint(0); i < (e.Scale - uint(len(fractional))); i++ {
		builder.WriteRune('0')
	}
	builder.WriteString(fractional)
	return builder.String()
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
	builder.WriteRune('(')
	for i, argument := range args {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(argument.String())
	}
	builder.WriteRune(')')
	return builder.String()
}

// InvocationExpression

type InvocationExpression struct {
	InvokedExpression Expression
	TypeArguments     []*TypeAnnotation
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
	if len(e.TypeArguments) > 0 {
		builder.WriteRune('<')
		for i, ty := range e.TypeArguments {
			if i > 0 {
				builder.WriteString(", ")
			}
			builder.WriteString(ty.String())
		}
		builder.WriteRune('>')
	}
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
	Expression
	isAccessExpression()
	AccessedExpression() Expression
}

// MemberExpression

type MemberExpression struct {
	Expression Expression
	Optional   bool
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
	optional := ""
	if e.Optional {
		optional = "?"
	}
	return fmt.Sprintf(
		"%s%s.%s",
		e.Expression, optional, e.Identifier,
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
	TargetExpression   Expression
	IndexingExpression Expression
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
	return fmt.Sprintf(
		"%s[%s]",
		e.TargetExpression, e.IndexingExpression,
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
	StartPos   Position
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

func (e *UnaryExpression) StartPosition() Position {
	return e.StartPos
}

func (e *UnaryExpression) EndPosition() Position {
	return e.Expression.EndPosition()
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

// CastingExpression

type CastingExpression struct {
	Expression                Expression
	Operation                 Operation
	TypeAnnotation            *TypeAnnotation
	ParentVariableDeclaration *VariableDeclaration
}

func (*CastingExpression) isExpression() {}

func (*CastingExpression) isIfStatementTest() {}

func (e *CastingExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *CastingExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitCastingExpression(e)
}

func (e *CastingExpression) String() string {
	return fmt.Sprintf(
		"(%s %s %s)",
		e.Expression, e.Operation.Symbol(), e.TypeAnnotation,
	)
}

func (e *CastingExpression) StartPosition() Position {
	return e.Expression.StartPosition()
}

func (e *CastingExpression) EndPosition() Position {
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

// ForceExpression

type ForceExpression struct {
	Expression Expression
	EndPos     Position
}

func (*ForceExpression) isExpression() {}

func (*ForceExpression) isIfStatementTest() {}

func (e *ForceExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *ForceExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitForceExpression(e)
}

func (e *ForceExpression) String() string {
	return fmt.Sprintf("%s!", e.Expression)
}

func (e *ForceExpression) StartPosition() Position {
	return e.Expression.StartPosition()
}

func (e *ForceExpression) EndPosition() Position {
	return e.EndPos
}

// PathExpression

type PathExpression struct {
	StartPos   Position
	Domain     Identifier
	Identifier Identifier
}

func (*PathExpression) isExpression() {}

func (*PathExpression) isIfStatementTest() {}

func (e *PathExpression) Accept(visitor Visitor) Repr {
	return e.AcceptExp(visitor)
}

func (e *PathExpression) AcceptExp(visitor ExpressionVisitor) Repr {
	return visitor.VisitPathExpression(e)
}

func (e *PathExpression) String() string {
	return fmt.Sprintf("/%s/%s", e.Domain, e.Identifier)
}

func (e *PathExpression) StartPosition() Position {
	return e.StartPos
}

func (e *PathExpression) EndPosition() Position {
	return e.Domain.EndPosition()
}
