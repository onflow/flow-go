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

type Repr interface{}

type Element interface {
	HasPosition
	Accept(Visitor) Repr
}

type NotAnElement struct{}

func (NotAnElement) Accept(Visitor) Repr {
	// NO-OP
	return nil
}

func (NotAnElement) StartPosition() Position {
	return Position{}
}

func (NotAnElement) EndPosition() Position {
	return Position{}
}

type StatementVisitor interface {
	VisitReturnStatement(*ReturnStatement) Repr
	VisitBreakStatement(*BreakStatement) Repr
	VisitContinueStatement(*ContinueStatement) Repr
	VisitIfStatement(*IfStatement) Repr
	VisitWhileStatement(*WhileStatement) Repr
	VisitForStatement(*ForStatement) Repr
	VisitEmitStatement(*EmitStatement) Repr
	VisitVariableDeclaration(*VariableDeclaration) Repr
	VisitAssignmentStatement(*AssignmentStatement) Repr
	VisitSwapStatement(*SwapStatement) Repr
	VisitExpressionStatement(*ExpressionStatement) Repr
}

type ExpressionVisitor interface {
	VisitBoolExpression(*BoolExpression) Repr
	VisitNilExpression(*NilExpression) Repr
	VisitIntegerExpression(*IntegerExpression) Repr
	VisitFixedPointExpression(*FixedPointExpression) Repr
	VisitArrayExpression(*ArrayExpression) Repr
	VisitDictionaryExpression(*DictionaryExpression) Repr
	VisitIdentifierExpression(*IdentifierExpression) Repr
	VisitInvocationExpression(*InvocationExpression) Repr
	VisitMemberExpression(*MemberExpression) Repr
	VisitIndexExpression(*IndexExpression) Repr
	VisitConditionalExpression(*ConditionalExpression) Repr
	VisitUnaryExpression(*UnaryExpression) Repr
	VisitBinaryExpression(*BinaryExpression) Repr
	VisitFunctionExpression(*FunctionExpression) Repr
	VisitStringExpression(*StringExpression) Repr
	VisitCastingExpression(*CastingExpression) Repr
	VisitCreateExpression(*CreateExpression) Repr
	VisitDestroyExpression(*DestroyExpression) Repr
	VisitReferenceExpression(*ReferenceExpression) Repr
	VisitForceExpression(*ForceExpression) Repr
	VisitPathExpression(*PathExpression) Repr
}

type Visitor interface {
	StatementVisitor
	ExpressionVisitor
	VisitProgram(*Program) Repr
	VisitFunctionDeclaration(*FunctionDeclaration) Repr
	VisitBlock(*Block) Repr
	VisitFunctionBlock(*FunctionBlock) Repr
	VisitCompositeDeclaration(*CompositeDeclaration) Repr
	VisitInterfaceDeclaration(*InterfaceDeclaration) Repr
	VisitFieldDeclaration(*FieldDeclaration) Repr
	VisitCondition(*Condition) Repr
	VisitImportDeclaration(*ImportDeclaration) Repr
	VisitTransactionDeclaration(*TransactionDeclaration) Repr
}
