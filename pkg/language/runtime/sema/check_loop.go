package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
)

func (checker *Checker) VisitWhileStatement(statement *ast.WhileStatement) ast.Repr {

	testExpression := statement.Test
	testType := testExpression.Accept(checker).(Type)

	if !IsSubType(testType, &BoolType{}) {
		checker.report(
			&TypeMismatchError{
				ExpectedType: &BoolType{},
				ActualType:   testType,
				StartPos:     testExpression.StartPosition(),
				EndPos:       testExpression.EndPosition(),
			},
		)
	}

	originalResources := checker.resources
	temporaryResources := originalResources.Clone()

	checkLoop := func() Type {
		checker.functionActivations.WithLoop(func() {
			statement.Block.Accept(checker)
		})
		return &VoidType{}
	}
	checker.checkWithResources(checkLoop, temporaryResources)

	checker.resources.MergeBranches(temporaryResources, nil)

	checker.reportResourceUsesInLoop(statement.StartPos, statement.EndPos)

	return nil
}

func (checker *Checker) reportResourceUsesInLoop(startPos, endPos ast.Position) {
	var resource interface{}
	var info ResourceInfo

	resources := checker.resources
	for resources.Size() != 0 {
		resource, info, resources = resources.FirstRest()
		variable := resource.(*Variable)

		// only report if the variable was invalidated
		if info.Invalidations.IsEmpty() {
			continue
		}

		invalidations := info.Invalidations.All()

		for _, usePosition := range info.UsePositions.AllPositions() {
			// only report if the variable is inside the loop
			if usePosition.Compare(startPos) < 0 ||
				usePosition.Compare(endPos) > 0 {

				continue
			}

			if checker.resources.IsUseAfterInvalidationReported(variable, usePosition) {
				continue
			}

			checker.resources.MarkUseAfterInvalidationReported(variable, usePosition)

			checker.report(
				&ResourceUseAfterInvalidationError{
					Name:          variable.Identifier,
					Pos:           usePosition,
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
				StartPos:         statement.StartPos,
				EndPos:           statement.EndPos,
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
				StartPos:         statement.StartPos,
				EndPos:           statement.EndPos,
			},
		)
	}

	return nil
}
