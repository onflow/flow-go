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

func (checker *Checker) VisitArrayExpression(expression *ast.ArrayExpression) ast.Repr {

	// visit all elements, ensure they are all the same type

	var elementType Type

	argumentTypes := make([]Type, len(expression.Values))

	for i, value := range expression.Values {
		valueType := value.Accept(checker).(Type)

		argumentTypes[i] = valueType

		checker.checkVariableMove(value)
		checker.checkResourceMoveOperation(value, valueType)

		// infer element type from first element
		// TODO: find common super type?
		if elementType == nil {
			elementType = valueType
		} else if !valueType.IsInvalidType() &&
			!IsSubType(valueType, elementType) {

			checker.report(
				&TypeMismatchError{
					ExpectedType: elementType,
					ActualType:   valueType,
					Range:        ast.NewRangeFromPositioned(value),
				},
			)
		}
	}

	checker.Elaboration.ArrayExpressionArgumentTypes[expression] = argumentTypes

	if elementType == nil {
		elementType = &NeverType{}
	}

	checker.Elaboration.ArrayExpressionElementType[expression] = elementType

	return &VariableSizedType{
		Type: elementType,
	}
}
