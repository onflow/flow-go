package sema

import "github.com/dapperlabs/flow-go/pkg/language/runtime/ast"

func (checker *Checker) VisitBlock(block *ast.Block) ast.Repr {
	checker.withValueScope(func() {
		checker.visitStatements(block.Statements)
	})
	return nil
}

func (checker *Checker) visitStatements(statements []ast.Statement) {

	// check all statements
	for _, statement := range statements {

		// check statement is not a local composite or interface declaration

		if compositeDeclaration, ok := statement.(*ast.CompositeDeclaration); ok {
			checker.report(
				&InvalidDeclarationError{
					Kind:     compositeDeclaration.DeclarationKind(),
					StartPos: statement.StartPosition(),
					EndPos:   statement.EndPosition(),
				},
			)

			continue
		}

		if interfaceDeclaration, ok := statement.(*ast.InterfaceDeclaration); ok {
			checker.report(
				&InvalidDeclarationError{
					Kind:     interfaceDeclaration.DeclarationKind(),
					StartPos: statement.StartPosition(),
					EndPos:   statement.EndPosition(),
				},
			)

			continue
		}

		// check statement

		statement.Accept(checker)
	}
}
