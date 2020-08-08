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

func (checker *Checker) VisitWhileStatement(statement *ast.WhileStatement) ast.Repr {

	testExpression := statement.Test
	testType := testExpression.Accept(checker).(Type)

	if !testType.IsInvalidType() &&
		!IsSubType(testType, &BoolType{}) {

		checker.report(
			&TypeMismatchError{
				ExpectedType: &BoolType{},
				ActualType:   testType,
				Range:        ast.NewRangeFromPositioned(testExpression),
			},
		)
	}

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

func (checker *Checker) reportResourceUsesInLoop(startPos, endPos ast.Position) {
	var resource interface{}
	var info ResourceInfo

	resources := checker.resources
	for resources.Size() != 0 {
		resource, info, resources = resources.FirstRest()

		// If the resource is a variable,
		// only report an error if the variable was declared outside the loop

		if variable, isVariable := resource.(*Variable); isVariable &&
			variable.Pos != nil &&
			variable.Pos.Compare(startPos) > 0 &&
			variable.Pos.Compare(endPos) < 0 {

			continue
		}

		// Only report an error if the resource was invalidated

		if info.Invalidations.IsEmpty() {
			continue
		}

		invalidations := info.Invalidations.All()

		for _, usePosition := range info.UsePositions.AllPositions() {

			// Only report an error if the use is inside the loop

			if usePosition.Compare(startPos) < 0 ||
				usePosition.Compare(endPos) > 0 {

				continue
			}

			if checker.resources.IsUseAfterInvalidationReported(resource, usePosition) {
				continue
			}

			checker.resources.MarkUseAfterInvalidationReported(resource, usePosition)

			checker.report(
				&ResourceUseAfterInvalidationError{
					// TODO: improve position information
					StartPos:      usePosition,
					EndPos:        usePosition,
					Invalidations: invalidations,
					InLoop:        true,
				},
			)
		}
	}
}

func (checker *Checker) VisitBreakStatement(statement *ast.BreakStatement) ast.Repr {

	// check statement is inside loop

	if !checker.inLoop() {
		checker.report(
			&ControlStatementError{
				ControlStatement: common.ControlStatementBreak,
				Range:            ast.NewRangeFromPositioned(statement),
			},
		)
	}

	return nil
}

func (checker *Checker) VisitContinueStatement(statement *ast.ContinueStatement) ast.Repr {

	// check statement is inside loop

	if !checker.inLoop() {
		checker.report(
			&ControlStatementError{
				ControlStatement: common.ControlStatementContinue,
				Range:            ast.NewRangeFromPositioned(statement),
			},
		)
	}

	return nil
}
