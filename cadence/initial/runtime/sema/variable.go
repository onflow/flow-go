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

import (
	"example.com/cadence-initial/runtime/ast"
	"example.com/cadence-initial/runtime/common"
)

type Variable struct {
	Identifier      string
	DeclarationKind common.DeclarationKind
	// Type is the type of the variable
	Type Type
	// Access is the access modifier
	Access ast.Access
	// IsConstant indicates if the variable is read-only
	IsConstant bool
	// IsBaseValue indicates if the variable is a base value,
	// i.e. it is defined by the checker and not the program
	IsBaseValue bool
	// Depth is the depth of scopes in which the variable was declared
	Depth int
	// ArgumentLabels are the argument labels that must be used in an invocation of the variable
	ArgumentLabels []string
	// Pos is the position where the variable was declared
	Pos *ast.Position
}
