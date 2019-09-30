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
		checker.visitConditionalBranches(
			func() Type {
				checker.withValueScope(func() {
					checker.visitVariableDeclaration(test, true)
					thenElement.Accept(checker)
				})
				return nil
			},
			func() Type {
				elseElement.Accept(checker)
				return nil
			},
		)
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

// visitConditional checks a conditional.
// The test expression must be a boolean.
// The then and else elements may be expressions, in which case their types are returned.
//
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

	return checker.visitConditionalBranches(
		func() Type {
			thenResult := thenElement.Accept(checker)
			if thenResult == nil {
				return nil
			}
			return thenResult.(Type)
		},
		func() Type {
			elseResult := elseElement.Accept(checker)
			if elseResult == nil {
				return nil
			}
			return elseResult.(Type)
		},
	)
}

func (checker *Checker) visitConditionalBranches(
	checkThen func() Type,
	checkElse func() Type,
) (
	thenType, elseType Type,
) {
	initialInvalidations := checker.resourceInvalidations
	thenInvalidations := initialInvalidations.Clone()
	elseInvalidations := initialInvalidations.Clone()

	thenType = checker.visitConditionalBranch(checkThen, thenInvalidations)
	elseType = checker.visitConditionalBranch(checkElse, elseInvalidations)

	// TODO: merge. invalidations in both then and else are definite, others only potential

	thenInvalidations.Merge(elseInvalidations)
	checker.resourceInvalidations = thenInvalidations

	return
}

func (checker *Checker) visitConditionalBranch(check func() Type, temporaryInvalidations *ResourceInvalidations) Type {
	originalInvalidations := checker.resourceInvalidations
	checker.resourceInvalidations = temporaryInvalidations
	defer func() {
		checker.resourceInvalidations = originalInvalidations
	}()

	return check()
}
