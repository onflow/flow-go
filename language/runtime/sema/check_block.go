package sema

import "github.com/dapperlabs/flow-go/language/runtime/ast"

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

		if functionActivation.ReturnInfo.DefinitelyReturned &&
			!functionActivation.ReportedDeadCode {

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

	checker.report(
		&InvalidDeclarationError{
			Identifier: declaration.DeclarationIdentifier().Identifier,
			Kind:       declaration.DeclarationKind(),
			Range:      ast.NewRangeFromPositioned(statement),
		},
	)

	return false
}
