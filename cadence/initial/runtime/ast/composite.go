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
	"example.com/cadence-initial/runtime/common"
)

// CompositeDeclaration

// NOTE: For events, only an empty initializer is declared

type CompositeDeclaration struct {
	Access                Access
	CompositeKind         common.CompositeKind
	Identifier            Identifier
	Conformances          []*NominalType
	Members               *Members
	CompositeDeclarations []*CompositeDeclaration
	InterfaceDeclarations []*InterfaceDeclaration
	Range
}

func (d *CompositeDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitCompositeDeclaration(d)
}

func (*CompositeDeclaration) isDeclaration() {}

// NOTE: statement, so it can be represented in the AST,
// but will be rejected in semantic analysis
//
func (*CompositeDeclaration) isStatement() {}

func (d *CompositeDeclaration) DeclarationIdentifier() *Identifier {
	return &d.Identifier
}

func (d *CompositeDeclaration) DeclarationKind() common.DeclarationKind {
	return d.CompositeKind.DeclarationKind(false)
}

func (d *CompositeDeclaration) DeclarationAccess() Access {
	return d.Access
}

// FieldDeclaration

type FieldDeclaration struct {
	Access         Access
	VariableKind   VariableKind
	Identifier     Identifier
	TypeAnnotation *TypeAnnotation
	Range
}

func (f *FieldDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitFieldDeclaration(f)
}

func (*FieldDeclaration) isDeclaration() {}

func (f *FieldDeclaration) DeclarationIdentifier() *Identifier {
	return &f.Identifier
}

func (f *FieldDeclaration) DeclarationKind() common.DeclarationKind {
	return common.DeclarationKindField
}

func (f *FieldDeclaration) DeclarationAccess() Access {
	return f.Access
}
