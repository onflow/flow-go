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

type TransactionDeclaration struct {
	ParameterList  *ParameterList
	Fields         []*FieldDeclaration
	Prepare        *SpecialFunctionDeclaration
	PreConditions  *Conditions
	PostConditions *Conditions
	Execute        *SpecialFunctionDeclaration
	Range
}

func (d *TransactionDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitTransactionDeclaration(d)
}

func (*TransactionDeclaration) isDeclaration() {}
func (*TransactionDeclaration) isStatement()   {}

func (d *TransactionDeclaration) DeclarationIdentifier() *Identifier {
	return nil
}

func (d *TransactionDeclaration) DeclarationKind() common.DeclarationKind {
	return common.DeclarationKindTransaction
}

func (d *TransactionDeclaration) DeclarationAccess() Access {
	return AccessNotSpecified
}
