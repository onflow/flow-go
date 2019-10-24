package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
)

func (checker *Checker) VisitSwapStatement(swap *ast.SwapStatement) ast.Repr {

	// The types of both sides must be subtypes of each other,
	// so that assignment can be performed in both directions.
	//
	// This is checked through the two `visitAssignmentValueType` calls.

	leftType := swap.Left.Accept(checker).(Type)
	rightType := swap.Right.Accept(checker).(Type)

	// Both sides must be a target expression (e.g. identifier expression,
	// indexing expression, or member access expression)

	checkRight := true

	if _, leftIsTarget := swap.Left.(ast.TargetExpression); !leftIsTarget {
		checker.report(
			&InvalidSwapExpressionError{
				Side: common.OperandSideLeft,
				Range: ast.Range{
					StartPos: swap.Left.StartPosition(),
					EndPos:   swap.Left.EndPosition(),
				},
			},
		)
	} else if !leftType.IsInvalidType() {
		// Only check the right-hand side if checking the left-hand side
		// doesn't produce errors. This prevents potentially confusing
		// duplicate errors

		errorCountBefore := len(checker.errors)

		checker.visitAssignmentValueType(swap.Left, swap.Right, rightType)

		errorCountAfter := len(checker.errors)
		if errorCountAfter != errorCountBefore {
			checkRight = false
		}
	}

	if _, rightIsTarget := swap.Right.(ast.TargetExpression); !rightIsTarget {
		checker.report(
			&InvalidSwapExpressionError{
				Side: common.OperandSideRight,
				Range: ast.Range{
					StartPos: swap.Right.StartPosition(),
					EndPos:   swap.Right.EndPosition(),
				},
			},
		)
	} else if !rightType.IsInvalidType() {
		// Only check the right-hand side if checking the left-hand side
		// doesn't produce errors. This prevents potentially confusing
		// duplicate errors

		if checkRight {
			checker.visitAssignmentValueType(swap.Right, swap.Left, leftType)
		}
	}

	return nil
}
