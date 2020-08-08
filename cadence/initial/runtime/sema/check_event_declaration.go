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
)

// checkEventParameters checks that the event initializer's parameters are valid,
// as determined by `isValidEventParameterType`.
//
func (checker *Checker) checkEventParameters(
	parameterList *ast.ParameterList,
	parameters []*Parameter,
) {

	for i, parameter := range parameterList.Parameters {
		parameterType := parameters[i].TypeAnnotation.Type

		if !parameterType.IsInvalidType() &&
			!isValidEventParameterType(parameterType) {

			checker.report(
				&InvalidEventParameterTypeError{
					Type: parameterType,
					Range: ast.Range{
						StartPos: parameter.StartPos,
						EndPos:   parameter.TypeAnnotation.EndPosition(),
					},
				},
			)
		}
	}
}

// isValidEventParameterType returns true if the given type is a valid event parameter type.
//
// Events currently only support a few simple Cadence types.
//
func isValidEventParameterType(t Type) bool {
	switch t := t.(type) {
	case *BoolType, *StringType, *CharacterType, *AddressType:
		return true

	case *OptionalType:
		return isValidEventParameterType(t.Type)

	case *VariableSizedType:
		return isValidEventParameterType(t.ElementType(false))

	case *ConstantSizedType:
		return isValidEventParameterType(t.ElementType(false))

	case *DictionaryType:
		return isValidEventParameterType(t.KeyType) &&
			isValidEventParameterType(t.ValueType)

	default:
		return IsSubType(t, &NumberType{})
	}
}
