package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/errors"
)

func (checker *Checker) VisitIfStatement(statement *ast.IfStatement) ast.Repr {

	thenElement := statement.Then

	var elseElement ast.Element = ast.NotAnElement{}
	if statement.Else != nil {
		elseElement = statement.Else
	}

	switch test := statement.Test.(type) {
	case ast.Expression:
		checker.visitConditional(test, thenElement, elseElement)

	case *ast.VariableDeclaration:
		checker.withValueScope(func() {

			checker.visitVariableDeclaration(test, true)

			thenElement.Accept(checker)
		})

		elseElement.Accept(checker)

	default:
		panic(&errors.UnreachableError{})
	}

	return nil
}

func (checker *Checker) VisitConditionalExpression(expression *ast.ConditionalExpression) ast.Repr {

	thenType, elseType := checker.visitConditional(expression.Test, expression.Then, expression.Else)

	if thenType == nil || elseType == nil {
		panic(&errors.UnreachableError{})
	}

	// TODO: improve
	resultType := thenType

	if !IsSubType(elseType, resultType) {
		checker.report(
			&TypeMismatchError{
				ExpectedType: resultType,
				ActualType:   elseType,
				StartPos:     expression.Else.StartPosition(),
				EndPos:       expression.Else.EndPosition(),
			},
		)
	}

	return resultType
}

// visitConditional checks a conditional. the test expression must be a boolean.
// the then and else elements may be expressions, in which case the types are returned.
func (checker *Checker) visitConditional(
	test ast.Expression,
	thenElement ast.Element,
	elseElement ast.Element,
) (
	thenType, elseType Type,
) {
	testType := test.Accept(checker).(Type)

	if !IsSubType(testType, &BoolType{}) {
		checker.report(
			&TypeMismatchError{
				ExpectedType: &BoolType{},
				ActualType:   testType,
				StartPos:     test.StartPosition(),
				EndPos:       test.EndPosition(),
			},
		)
	}

	thenResult := thenElement.Accept(checker)
	if thenResult != nil {
		thenType = thenResult.(Type)
	}

	elseResult := elseElement.Accept(checker)
	if elseResult != nil {
		elseType = elseResult.(Type)
	}

	return
}
