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
	initialMovePositions := checker.movePositions.Clone()
	initialDestructionPositions := checker.destructionPositions.Clone()

	thenType = checkThen()

	movePositionsAfterThen := checker.movePositions
	destructionPositionsAfterThen := checker.destructionPositions

	// NOTE: reset moves and destruction positions for else branch

	checker.movePositions = initialMovePositions
	checker.destructionPositions = initialDestructionPositions

	elseType = checkElse()

	checker.movePositions.Merge(movePositionsAfterThen)
	checker.destructionPositions.Merge(destructionPositionsAfterThen)

	return
}
