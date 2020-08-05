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
	"github.com/onflow/cadence-initial/runtime/ast"
	"github.com/onflow/cadence-initial/runtime/common"
)

func (checker *Checker) VisitForStatement(statement *ast.ForStatement) ast.Repr {

	checker.enterValueScope()
	defer checker.leaveValueScope(true)

	valueExpression := statement.Value
	valueType := valueExpression.Accept(checker).(Type)

	var elementType Type = &InvalidType{}

	if !valueType.IsInvalidType() {

		// Only get the element type if the array is not a resource array.
		// Otherwise, in addition to the `UnsupportedResourceForLoopError`,
		// the loop variable will be declared with the resource-typed element type,
		// leading to an additional `ResourceLossError`.

		if valueType.IsResourceType() {
			checker.report(
				&UnsupportedResourceForLoopError{
					Range: ast.NewRangeFromPositioned(valueExpression),
				},
			)
		} else if arrayType, ok := valueType.(ArrayType); ok {
			elementType = arrayType.ElementType(false)
		} else {
			checker.report(
				&TypeMismatchWithDescriptionError{
					ExpectedTypeDescription: "array",
					ActualType:              valueType,
					Range:                   ast.NewRangeFromPositioned(valueExpression),
				},
			)
		}
	}

	identifier := statement.Identifier.Identifier

	variable, err := checker.valueActivations.Declare(variableDeclaration{
		identifier:               identifier,
		ty:                       elementType,
		kind:                     common.DeclarationKindConstant,
		pos:                      statement.Identifier.Pos,
		isConstant:               true,
		argumentLabels:           nil,
		allowOuterScopeShadowing: false,
	})
	checker.report(err)
	checker.recordVariableDeclarationOccurrence(identifier, variable)

	// The body of the loop will maybe be evaluated.
	// That means that resource invalidations and
	// returns are not definite, but only potential.

	_ = checker.checkPotentiallyUnevaluated(func() Type {
		checker.functionActivations.WithLoop(func() {
			statement.Block.Accept(checker)
		})

		// ignored
		return nil
	})

	checker.reportResourceUsesInLoop(statement.StartPos, statement.EndPosition())

	return nil
}
