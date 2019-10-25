package sema

import "github.com/dapperlabs/flow-go/pkg/language/runtime/ast"

func (checker *Checker) VisitBlock(block *ast.Block) ast.Repr {
	checker.withValueScope(func() {
		checker.visitStatements(block.Statements)
	})
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

		// check statement is not a local composite or interface declaration

		if compositeDeclaration, ok := statement.(*ast.CompositeDeclaration); ok {
			checker.report(
				&InvalidDeclarationError{
					Kind:  compositeDeclaration.DeclarationKind(),
					Range: ast.NewRangeFromPositioned(statement),
				},
			)

			continue
		}

		if interfaceDeclaration, ok := statement.(*ast.InterfaceDeclaration); ok {
			checker.report(
				&InvalidDeclarationError{
					Kind:  interfaceDeclaration.DeclarationKind(),
					Range: ast.NewRangeFromPositioned(statement),
				},
			)

			continue
		}

		// check statement

		statement.Accept(checker)
	}
}
