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
	"github.com/onflow/cadence-initial/runtime/common"
)

// InterfaceDeclaration

type InterfaceDeclaration struct {
	Access                Access
	CompositeKind         common.CompositeKind
	Identifier            Identifier
	Members               *Members
	CompositeDeclarations []*CompositeDeclaration
	InterfaceDeclarations []*InterfaceDeclaration
	Range
}

func (d *InterfaceDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitInterfaceDeclaration(d)
}

func (*InterfaceDeclaration) isDeclaration() {}

// NOTE: statement, so it can be represented in the AST,
// but will be rejected in semantic analysis
//
func (*InterfaceDeclaration) isStatement() {}

func (d *InterfaceDeclaration) DeclarationIdentifier() *Identifier {
	return &d.Identifier
}

func (d *InterfaceDeclaration) DeclarationAccess() Access {
	return d.Access
}

func (d *InterfaceDeclaration) DeclarationKind() common.DeclarationKind {
	return d.CompositeKind.DeclarationKind(true)
}
