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

func (checker *Checker) VisitSwapStatement(swap *ast.SwapStatement) ast.Repr {

	// The types of both sides must be subtypes of each other,
	// so that assignment can be performed in both directions.
	//
	// This is checked through the two `visitAssignmentValueType` calls.

	leftType := swap.Left.Accept(checker).(Type)
	rightType := swap.Right.Accept(checker).(Type)

	checker.Elaboration.SwapStatementLeftTypes[swap] = leftType
	checker.Elaboration.SwapStatementRightTypes[swap] = rightType

	// Both sides must be a target expression (e.g. identifier expression,
	// indexing expression, or member access expression)

	checkRight := true

	if _, leftIsTarget := swap.Left.(ast.TargetExpression); !leftIsTarget {
		checker.report(
			&InvalidSwapExpressionError{
				Side:  common.OperandSideLeft,
				Range: ast.NewRangeFromPositioned(swap.Left),
			},
		)
	} else if !leftType.IsInvalidType() {
		// Only check the right-hand side if checking the left-hand side
		// doesn't produce errors. This prevents potentially confusing
		// duplicate errors

		errorCountBefore := len(checker.errors)

		checker.visitAssignmentValueType(swap.Left, swap.Right, rightType)

		errorCountAfter := len(checker.errors)
		if errorCountAfter != errorCountBefore {
			checkRight = false
		}
	}

	if _, rightIsTarget := swap.Right.(ast.TargetExpression); !rightIsTarget {
		checker.report(
			&InvalidSwapExpressionError{
				Side:  common.OperandSideRight,
				Range: ast.NewRangeFromPositioned(swap.Right),
			},
		)
	} else if !rightType.IsInvalidType() {
		// Only check the right-hand side if checking the left-hand side
		// doesn't produce errors. This prevents potentially confusing
		// duplicate errors

		if checkRight {
			checker.visitAssignmentValueType(swap.Right, swap.Left, leftType)
		}
	}

	if leftType.IsResourceType() {
		checker.elaboratePotentialResourceStorageMove(swap.Left)
	}

	if rightType.IsResourceType() {
		checker.elaboratePotentialResourceStorageMove(swap.Right)
	}

	return nil
}
