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

func (checker *Checker) VisitBlock(block *ast.Block) ast.Repr {
	checker.enterValueScope()
	defer checker.leaveValueScope(true)

	checker.visitStatements(block.Statements)

	return nil
}

func (checker *Checker) visitStatements(statements []ast.Statement) {

	functionActivation := checker.functionActivations.Current()

	// check all statements
	for _, statement := range statements {

		// Is this statement unreachable? Report it once for this statement,
		// but avoid noise and don't report it for all remaining unreachable statements

		definitelyReturnedOrHalted :=
			functionActivation.ReturnInfo.DefinitelyReturned ||
				functionActivation.ReturnInfo.DefinitelyHalted

		if definitelyReturnedOrHalted && !functionActivation.ReportedDeadCode {

			lastStatement := statements[len(statements)-1]

			checker.report(
				&UnreachableStatementError{
					Range: ast.Range{
						StartPos: statement.StartPosition(),
						EndPos:   lastStatement.EndPosition(),
					},
				},
			)

			functionActivation.ReportedDeadCode = true
		}

		if !checker.checkValidStatement(statement) {
			continue
		}

		// check statement

		statement.Accept(checker)
	}
}

func (checker *Checker) checkValidStatement(statement ast.Statement) bool {

	// Check the statement is not a declaration which is not allowed locally

	declaration, isDeclaration := statement.(ast.Declaration)
	if !isDeclaration {
		return true
	}

	// Only function and variable declarations are allowed locally

	switch declaration.(type) {
	case *ast.FunctionDeclaration, *ast.VariableDeclaration:
		return true
	}

	identifier := declaration.DeclarationIdentifier()

	var name string
	if identifier != nil {
		name = identifier.Identifier
	}

	checker.report(
		&InvalidDeclarationError{
			Identifier: name,
			Kind:       declaration.DeclarationKind(),
			Range:      ast.NewRangeFromPositioned(statement),
		},
	)

	return false
}
