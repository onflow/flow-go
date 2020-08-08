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
	"github.com/raviqqe/hamt"

	"example.com/cadence-initial/runtime/activations"
	"example.com/cadence-initial/runtime/ast"
	"example.com/cadence-initial/runtime/common"
)

type VariableActivations struct {
	activations *activations.Activations
}

func NewValueActivations() *VariableActivations {
	valueActivations := &activations.Activations{}
	valueActivations.Push(hamt.NewMap())
	return &VariableActivations{
		activations: valueActivations,
	}
}

func (a *VariableActivations) Enter() {
	a.activations.PushCurrent()
}

func (a *VariableActivations) Leave() {
	a.activations.Pop()
}

func (a *VariableActivations) Set(name string, variable *Variable) {
	a.activations.Set(name, variable)
}

func (a *VariableActivations) Find(name string) *Variable {
	value := a.activations.Find(name)
	if value == nil {
		return nil
	}
	variable, ok := value.(*Variable)
	if !ok {
		return nil
	}
	return variable
}

func (a *VariableActivations) Depth() int {
	return a.activations.Depth()
}

type variableDeclaration struct {
	identifier               string
	ty                       Type
	access                   ast.Access
	kind                     common.DeclarationKind
	pos                      ast.Position
	isConstant               bool
	argumentLabels           []string
	allowOuterScopeShadowing bool
}

func (a *VariableActivations) Declare(declaration variableDeclaration) (variable *Variable, err error) {

	depth := a.activations.Depth()

	// Check if a variable with this name is already declared.
	// Report an error if shadowing variables of outer scopes is not allowed,
	// or the existing variable is declared in the current scope.

	existingVariable := a.Find(declaration.identifier)
	if existingVariable != nil &&
		(!declaration.allowOuterScopeShadowing ||
			existingVariable.Depth == depth) {

		err = &RedeclarationError{
			Kind:        declaration.kind,
			Name:        declaration.identifier,
			Pos:         declaration.pos,
			PreviousPos: existingVariable.Pos,
		}

		// NOTE: Don't return if there is an error,
		// still declare the variable and return it
	}

	// A variable with this name is not yet declared in the current scope,
	// declare it.

	variable = &Variable{
		Identifier:      declaration.identifier,
		Access:          declaration.access,
		DeclarationKind: declaration.kind,
		IsConstant:      declaration.isConstant,
		Depth:           depth,
		Type:            declaration.ty,
		Pos:             &declaration.pos,
		ArgumentLabels:  declaration.argumentLabels,
	}
	a.activations.Set(declaration.identifier, variable)
	return variable, err
}

type typeDeclaration struct {
	identifier               ast.Identifier
	ty                       Type
	declarationKind          common.DeclarationKind
	access                   ast.Access
	allowOuterScopeShadowing bool
}

func (a *VariableActivations) DeclareType(declaration typeDeclaration) (*Variable, error) {
	return a.Declare(
		variableDeclaration{
			identifier:               declaration.identifier.Identifier,
			ty:                       declaration.ty,
			access:                   declaration.access,
			kind:                     declaration.declarationKind,
			pos:                      declaration.identifier.Pos,
			isConstant:               true,
			argumentLabels:           nil,
			allowOuterScopeShadowing: declaration.allowOuterScopeShadowing,
		},
	)
}

func (a *VariableActivations) DeclareImplicitConstant(
	identifier string,
	ty Type,
	kind common.DeclarationKind,
) (*Variable, error) {
	return a.Declare(
		variableDeclaration{
			identifier:               identifier,
			ty:                       ty,
			access:                   ast.AccessPublic,
			kind:                     kind,
			isConstant:               true,
			allowOuterScopeShadowing: false,
		},
	)
}

func (a *VariableActivations) VariablesDeclaredInAndBelow(depth int) map[string]*Variable {
	variables := map[string]*Variable{}

	values := a.activations.CurrentOrNew()

	var entry hamt.Entry
	var value interface{}

	for {
		entry, value, values = values.FirstRest()
		if entry == nil {
			break
		}

		variable := value.(*Variable)

		if variable.Depth < depth {
			continue
		}

		name := string(entry.(common.StringEntry))

		variables[name] = variable
	}

	return variables
}
