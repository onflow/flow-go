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

func (checker *Checker) VisitUnaryExpression(expression *ast.UnaryExpression) ast.Repr {

	valueType := expression.Expression.Accept(checker).(Type)

	reportInvalidUnaryOperator := func(expectedType Type) {
		checker.report(
			&InvalidUnaryOperandError{
				Operation:    expression.Operation,
				ExpectedType: expectedType,
				ActualType:   valueType,
				Range:        ast.NewRangeFromPositioned(expression.Expression),
			},
		)
	}

	switch expression.Operation {
	case ast.OperationNegate:
		expectedType := &BoolType{}
		if !IsSubType(valueType, expectedType) {
			reportInvalidUnaryOperator(expectedType)
		}
		return valueType

	case ast.OperationMinus:
		expectedType := &SignedNumberType{}
		if !IsSubType(valueType, expectedType) {
			reportInvalidUnaryOperator(expectedType)
		}

		return valueType

	case ast.OperationMove:
		if !valueType.IsInvalidType() &&
			!valueType.IsResourceType() {

			checker.report(
				&InvalidMoveOperationError{
					Range: ast.Range{
						StartPos: expression.StartPos,
						EndPos:   expression.Expression.StartPosition(),
					},
				},
			)
		}

		return valueType
	}

	panic(&unsupportedOperation{
		kind:      common.OperationKindUnary,
		operation: expression.Operation,
		Range:     ast.NewRangeFromPositioned(expression),
	})
}
