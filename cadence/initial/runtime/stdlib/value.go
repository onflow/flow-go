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

package stdlib

import (
	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"
)

type StandardLibraryValue struct {
	Name       string
	Type       sema.Type
	Kind       common.DeclarationKind
	IsConstant bool
}

func (v StandardLibraryValue) ValueDeclarationType() sema.Type {
	return v.Type
}

func (v StandardLibraryValue) ValueDeclarationKind() common.DeclarationKind {
	if v.IsConstant {
		return common.DeclarationKindConstant
	}
	return common.DeclarationKindVariable
}

func (StandardLibraryValue) ValueDeclarationPosition() ast.Position {
	return ast.Position{}
}

func (v StandardLibraryValue) ValueDeclarationIsConstant() bool {
	return v.IsConstant
}

func (StandardLibraryValue) ValueDeclarationArgumentLabels() []string {
	return nil
}

// StandardLibraryValues

type StandardLibraryValues []StandardLibraryValue

func (functions StandardLibraryValues) ToValueDeclarations() map[string]sema.ValueDeclaration {
	valueDeclarations := make(map[string]sema.ValueDeclaration, len(functions))
	for _, function := range functions {
		valueDeclarations[function.Name] = function
	}
	return valueDeclarations
}
