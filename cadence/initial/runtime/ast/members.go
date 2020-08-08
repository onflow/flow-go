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

import "example.com/cadence-initial/runtime/common"

// Members

type Members struct {
	Fields []*FieldDeclaration
	// Use `FieldsByIdentifier()` instead
	_fieldsByIdentifier map[string]*FieldDeclaration
	// All special functions, such as initializers and destructors.
	// Use `Initializers()` and `Destructors()` to get subsets
	SpecialFunctions []*SpecialFunctionDeclaration
	// Use `Initializers()` instead
	_initializers []*SpecialFunctionDeclaration
	// Semantically only one destructor is allowed,
	// but the program might illegally declare multiple.
	// Use `Destructors()` instead
	_destructors []*SpecialFunctionDeclaration
	Functions    []*FunctionDeclaration
	// Use `FunctionsByIdentifier()` instead
	_functionsByIdentifier map[string]*FunctionDeclaration
}

func (m *Members) FieldsByIdentifier() map[string]*FieldDeclaration {
	if m._fieldsByIdentifier == nil {
		fieldsByIdentifier := make(map[string]*FieldDeclaration, len(m.Fields))
		for _, field := range m.Fields {
			fieldsByIdentifier[field.Identifier.Identifier] = field
		}
		m._fieldsByIdentifier = fieldsByIdentifier
	}
	return m._fieldsByIdentifier
}

func (m *Members) FunctionsByIdentifier() map[string]*FunctionDeclaration {
	if m._functionsByIdentifier == nil {
		functionsByIdentifier := make(map[string]*FunctionDeclaration, len(m.Functions))
		for _, function := range m.Functions {
			functionsByIdentifier[function.Identifier.Identifier] = function
		}
		m._functionsByIdentifier = functionsByIdentifier
	}
	return m._functionsByIdentifier
}

func (m *Members) Initializers() []*SpecialFunctionDeclaration {
	if m._initializers == nil {
		initializers := []*SpecialFunctionDeclaration{}
		for _, function := range m.SpecialFunctions {
			if function.Kind != common.DeclarationKindInitializer {
				continue
			}
			initializers = append(initializers, function)
		}
		m._initializers = initializers
	}
	return m._initializers
}

func (m *Members) Destructors() []*SpecialFunctionDeclaration {
	if m._destructors == nil {
		destructors := []*SpecialFunctionDeclaration{}
		for _, function := range m.SpecialFunctions {
			if function.Kind != common.DeclarationKindDestructor {
				continue
			}
			destructors = append(destructors, function)
		}
		m._destructors = destructors
	}
	return m._destructors
}

// Destructor returns the first destructor, if any
func (m *Members) Destructor() *SpecialFunctionDeclaration {
	destructors := m.Destructors()
	if len(destructors) == 0 {
		return nil
	}
	return destructors[0]
}

func (m *Members) FieldPosition(name string, compositeKind common.CompositeKind) Position {
	if compositeKind == common.CompositeKindEvent {
		parameters := m.Initializers()[0].ParameterList.ParametersByIdentifier()
		parameter := parameters[name]
		return parameter.Identifier.Pos
	} else {
		fields := m.FieldsByIdentifier()
		field := fields[name]
		return field.Identifier.Pos
	}
}
