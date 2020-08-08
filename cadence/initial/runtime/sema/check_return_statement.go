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

import "example.com/cadence-initial/runtime/ast"

func (checker *Checker) VisitReturnStatement(statement *ast.ReturnStatement) ast.Repr {
	functionActivation := checker.functionActivations.Current()

	defer func() {
		checker.checkResourceLossForFunction()
		checker.resources.Returns = true
		functionActivation.ReturnInfo.MaybeReturned = true
		functionActivation.ReturnInfo.DefinitelyReturned = true
	}()

	// check value type matches enclosing function's return type

	if statement.Expression == nil {
		return nil
	}

	valueType := statement.Expression.Accept(checker).(Type)
	returnType := functionActivation.ReturnType

	checker.Elaboration.ReturnStatementValueTypes[statement] = valueType
	checker.Elaboration.ReturnStatementReturnTypes[statement] = returnType

	if valueType == nil {
		return nil
	}

	// return statement has expression, but function has Void return type?
	if _, ok := returnType.(*VoidType); ok {
		checker.report(
			&InvalidReturnValueError{
				Range: ast.NewRangeFromPositioned(statement.Expression),
			},
		)
	} else {

		if !valueType.IsInvalidType() &&
			!returnType.IsInvalidType() &&
			!checker.checkTypeCompatibility(statement.Expression, valueType, returnType) {

			checker.report(
				&TypeMismatchError{
					ExpectedType: returnType,
					ActualType:   valueType,
					Range:        ast.NewRangeFromPositioned(statement.Expression),
				},
			)
		}

		checker.checkVariableMove(statement.Expression)
		checker.checkResourceMoveOperation(statement.Expression, valueType)
	}

	return nil
}

func (checker *Checker) checkResourceLossForFunction() {
	functionValueActivationDepth :=
		checker.functionActivations.Current().ValueActivationDepth
	checker.checkResourceLoss(functionValueActivationDepth)
}
