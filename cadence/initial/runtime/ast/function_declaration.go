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

import "github.com/onflow/cadence-initial/runtime/common"

type FunctionDeclaration struct {
	Access               Access
	Identifier           Identifier
	ParameterList        *ParameterList
	ReturnTypeAnnotation *TypeAnnotation
	FunctionBlock        *FunctionBlock
	StartPos             Position
}

func (f *FunctionDeclaration) StartPosition() Position {
	return f.StartPos
}

func (f *FunctionDeclaration) EndPosition() Position {
	if f.FunctionBlock != nil {
		return f.FunctionBlock.EndPosition()
	}
	if f.ReturnTypeAnnotation != nil {
		return f.ReturnTypeAnnotation.EndPosition()
	}
	return f.ParameterList.EndPosition()
}

func (f *FunctionDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitFunctionDeclaration(f)
}

func (*FunctionDeclaration) isDeclaration() {}
func (*FunctionDeclaration) isStatement()   {}

func (f *FunctionDeclaration) DeclarationIdentifier() *Identifier {
	return &f.Identifier
}

func (f *FunctionDeclaration) DeclarationKind() common.DeclarationKind {
	return common.DeclarationKindFunction
}

func (f *FunctionDeclaration) DeclarationAccess() Access {
	return f.Access
}

func (f *FunctionDeclaration) ToExpression() *FunctionExpression {
	return &FunctionExpression{
		ParameterList:        f.ParameterList,
		ReturnTypeAnnotation: f.ReturnTypeAnnotation,
		FunctionBlock:        f.FunctionBlock,
		StartPos:             f.StartPos,
	}
}

// SpecialFunctionDeclaration

type SpecialFunctionDeclaration struct {
	Kind common.DeclarationKind
	*FunctionDeclaration
}

func (f *SpecialFunctionDeclaration) DeclarationKind() common.DeclarationKind {
	return f.Kind
}
