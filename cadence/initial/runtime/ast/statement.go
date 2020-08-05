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

type Statement interface {
	Element
	isStatement()
}

// ReturnStatement

type ReturnStatement struct {
	Expression Expression
	Range
}

func (*ReturnStatement) isStatement() {}

func (s *ReturnStatement) Accept(visitor Visitor) Repr {
	return visitor.VisitReturnStatement(s)
}

// BreakStatement

type BreakStatement struct {
	Range
}

func (*BreakStatement) isStatement() {}

func (s *BreakStatement) Accept(visitor Visitor) Repr {
	return visitor.VisitBreakStatement(s)
}

// ContinueStatement

type ContinueStatement struct {
	Range
}

func (*ContinueStatement) isStatement() {}

func (s *ContinueStatement) Accept(visitor Visitor) Repr {
	return visitor.VisitContinueStatement(s)
}

// IfStatementTest

type IfStatementTest interface {
	isIfStatementTest()
}

// IfStatement

type IfStatement struct {
	Test     IfStatementTest
	Then     *Block
	Else     *Block
	StartPos Position
}

func (s *IfStatement) StartPosition() Position {
	return s.StartPos
}

func (s *IfStatement) EndPosition() Position {
	if s.Else != nil {
		return s.Else.EndPosition()
	}
	return s.Then.EndPosition()
}

func (*IfStatement) isStatement() {}

func (s *IfStatement) Accept(visitor Visitor) Repr {
	return visitor.VisitIfStatement(s)
}

// WhileStatement

type WhileStatement struct {
	Test     Expression
	Block    *Block
	StartPos Position
}

func (*WhileStatement) isStatement() {}

func (s *WhileStatement) Accept(visitor Visitor) Repr {
	return visitor.VisitWhileStatement(s)
}

func (s *WhileStatement) StartPosition() Position {
	return s.StartPos
}

func (s *WhileStatement) EndPosition() Position {
	return s.Block.EndPosition()
}

// ForStatement

type ForStatement struct {
	Identifier Identifier
	Value      Expression
	Block      *Block
	StartPos   Position
}

func (*ForStatement) isStatement() {}

func (s *ForStatement) Accept(visitor Visitor) Repr {
	return visitor.VisitForStatement(s)
}

func (s *ForStatement) StartPosition() Position {
	return s.StartPos
}

func (s *ForStatement) EndPosition() Position {
	return s.Block.EndPosition()
}

// EmitStatement

type EmitStatement struct {
	InvocationExpression *InvocationExpression
	StartPos             Position
}

func (s *EmitStatement) StartPosition() Position {
	return s.StartPos
}

func (s *EmitStatement) EndPosition() Position {
	return s.InvocationExpression.EndPosition()
}

func (*EmitStatement) isStatement() {}

func (s *EmitStatement) Accept(visitor Visitor) Repr {
	return visitor.VisitEmitStatement(s)
}

// AssignmentStatement

type AssignmentStatement struct {
	Target   Expression
	Transfer *Transfer
	Value    Expression
}

func (s *AssignmentStatement) StartPosition() Position {
	return s.Target.StartPosition()
}

func (s *AssignmentStatement) EndPosition() Position {
	return s.Value.EndPosition()
}

func (*AssignmentStatement) isStatement() {}

func (s *AssignmentStatement) Accept(visitor Visitor) Repr {
	return visitor.VisitAssignmentStatement(s)
}

// SwapStatement

type SwapStatement struct {
	Left  Expression
	Right Expression
}

func (s *SwapStatement) StartPosition() Position {
	return s.Left.StartPosition()
}

func (s *SwapStatement) EndPosition() Position {
	return s.Right.EndPosition()
}

func (*SwapStatement) isStatement() {}

func (s *SwapStatement) Accept(visitor Visitor) Repr {
	return visitor.VisitSwapStatement(s)
}

// ExpressionStatement

type ExpressionStatement struct {
	Expression Expression
}

func (s *ExpressionStatement) StartPosition() Position {
	return s.Expression.StartPosition()
}

func (s *ExpressionStatement) EndPosition() Position {
	return s.Expression.EndPosition()
}

func (*ExpressionStatement) isStatement() {}

func (s *ExpressionStatement) Accept(visitor Visitor) Repr {
	return visitor.VisitExpressionStatement(s)
}
