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

package sema

import "github.com/onflow/cadence-initial/runtime/ast"

type MemberInfo struct {
	Member     *Member
	IsOptional bool
}

type Elaboration struct {
	FunctionDeclarationFunctionTypes    map[*ast.FunctionDeclaration]*FunctionType
	VariableDeclarationValueTypes       map[*ast.VariableDeclaration]Type
	VariableDeclarationSecondValueTypes map[*ast.VariableDeclaration]Type
	VariableDeclarationTargetTypes      map[*ast.VariableDeclaration]Type
	AssignmentStatementValueTypes       map[*ast.AssignmentStatement]Type
	AssignmentStatementTargetTypes      map[*ast.AssignmentStatement]Type
	CompositeDeclarationTypes           map[*ast.CompositeDeclaration]*CompositeType
	SpecialFunctionTypes                map[*ast.SpecialFunctionDeclaration]*SpecialFunctionType
	FunctionExpressionFunctionType      map[*ast.FunctionExpression]*FunctionType
	InvocationExpressionArgumentTypes   map[*ast.InvocationExpression][]Type
	InvocationExpressionParameterTypes  map[*ast.InvocationExpression][]Type
	InvocationExpressionReturnTypes     map[*ast.InvocationExpression]Type
	InterfaceDeclarationTypes           map[*ast.InterfaceDeclaration]*InterfaceType
	CastingStaticValueTypes             map[*ast.CastingExpression]Type
	CastingTargetTypes                  map[*ast.CastingExpression]Type
	ReturnStatementValueTypes           map[*ast.ReturnStatement]Type
	ReturnStatementReturnTypes          map[*ast.ReturnStatement]Type
	BinaryExpressionResultTypes         map[*ast.BinaryExpression]Type
	BinaryExpressionRightTypes          map[*ast.BinaryExpression]Type
	MemberExpressionMemberInfos         map[*ast.MemberExpression]MemberInfo
	ArrayExpressionArgumentTypes        map[*ast.ArrayExpression][]Type
	ArrayExpressionElementType          map[*ast.ArrayExpression]Type
	DictionaryExpressionType            map[*ast.DictionaryExpression]*DictionaryType
	DictionaryExpressionEntryTypes      map[*ast.DictionaryExpression][]DictionaryEntryType
	TransactionDeclarationTypes         map[*ast.TransactionDeclaration]*TransactionType
	// NOTE: not indexed by `ast.Type`, as IndexExpression might index
	//   with "type" which is an expression, i.e., an IdentifierExpression.
	//   See `Checker.visitTypeIndexingExpression`
	IndexExpressionIndexingTypes           map[*ast.IndexExpression]Type
	SwapStatementLeftTypes                 map[*ast.SwapStatement]Type
	SwapStatementRightTypes                map[*ast.SwapStatement]Type
	IsTypeIndexExpression                  map[*ast.IndexExpression]bool
	IsResourceMovingStorageIndexExpression map[*ast.IndexExpression]bool
	CompositeNestedDeclarations            map[*ast.CompositeDeclaration]map[string]ast.Declaration
	InterfaceNestedDeclarations            map[*ast.InterfaceDeclaration]map[string]ast.Declaration
	PostConditionsRewrite                  map[*ast.Conditions]PostConditionsRewrite
	EmitStatementEventTypes                map[*ast.EmitStatement]*CompositeType
	// Keyed by qualified identifier
	CompositeTypes                         map[TypeID]*CompositeType
	InterfaceTypes                         map[TypeID]*InterfaceType
	InvocationExpressionTypeParameterTypes map[*ast.InvocationExpression]map[*TypeParameter]Type
}

func NewElaboration() *Elaboration {
	return &Elaboration{
		FunctionDeclarationFunctionTypes:       map[*ast.FunctionDeclaration]*FunctionType{},
		VariableDeclarationValueTypes:          map[*ast.VariableDeclaration]Type{},
		VariableDeclarationSecondValueTypes:    map[*ast.VariableDeclaration]Type{},
		VariableDeclarationTargetTypes:         map[*ast.VariableDeclaration]Type{},
		AssignmentStatementValueTypes:          map[*ast.AssignmentStatement]Type{},
		AssignmentStatementTargetTypes:         map[*ast.AssignmentStatement]Type{},
		CompositeDeclarationTypes:              map[*ast.CompositeDeclaration]*CompositeType{},
		SpecialFunctionTypes:                   map[*ast.SpecialFunctionDeclaration]*SpecialFunctionType{},
		FunctionExpressionFunctionType:         map[*ast.FunctionExpression]*FunctionType{},
		InvocationExpressionArgumentTypes:      map[*ast.InvocationExpression][]Type{},
		InvocationExpressionParameterTypes:     map[*ast.InvocationExpression][]Type{},
		InvocationExpressionReturnTypes:        map[*ast.InvocationExpression]Type{},
		InterfaceDeclarationTypes:              map[*ast.InterfaceDeclaration]*InterfaceType{},
		CastingStaticValueTypes:                map[*ast.CastingExpression]Type{},
		CastingTargetTypes:                     map[*ast.CastingExpression]Type{},
		ReturnStatementValueTypes:              map[*ast.ReturnStatement]Type{},
		ReturnStatementReturnTypes:             map[*ast.ReturnStatement]Type{},
		BinaryExpressionResultTypes:            map[*ast.BinaryExpression]Type{},
		BinaryExpressionRightTypes:             map[*ast.BinaryExpression]Type{},
		MemberExpressionMemberInfos:            map[*ast.MemberExpression]MemberInfo{},
		ArrayExpressionArgumentTypes:           map[*ast.ArrayExpression][]Type{},
		ArrayExpressionElementType:             map[*ast.ArrayExpression]Type{},
		DictionaryExpressionType:               map[*ast.DictionaryExpression]*DictionaryType{},
		DictionaryExpressionEntryTypes:         map[*ast.DictionaryExpression][]DictionaryEntryType{},
		TransactionDeclarationTypes:            map[*ast.TransactionDeclaration]*TransactionType{},
		IndexExpressionIndexingTypes:           map[*ast.IndexExpression]Type{},
		SwapStatementLeftTypes:                 map[*ast.SwapStatement]Type{},
		SwapStatementRightTypes:                map[*ast.SwapStatement]Type{},
		IsTypeIndexExpression:                  map[*ast.IndexExpression]bool{},
		IsResourceMovingStorageIndexExpression: map[*ast.IndexExpression]bool{},
		CompositeNestedDeclarations:            map[*ast.CompositeDeclaration]map[string]ast.Declaration{},
		InterfaceNestedDeclarations:            map[*ast.InterfaceDeclaration]map[string]ast.Declaration{},
		PostConditionsRewrite:                  map[*ast.Conditions]PostConditionsRewrite{},
		EmitStatementEventTypes:                map[*ast.EmitStatement]*CompositeType{},
		CompositeTypes:                         map[TypeID]*CompositeType{},
		InterfaceTypes:                         map[TypeID]*InterfaceType{},
		InvocationExpressionTypeParameterTypes: map[*ast.InvocationExpression]map[*TypeParameter]Type{},
	}
}
