package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
)

func (checker *Checker) VisitUnaryExpression(expression *ast.UnaryExpression) ast.Repr {

	valueType := expression.Expression.Accept(checker).(Type)

	reportInvalidUnaryOperator := func(expectedType Type) {
		checker.report(
			&InvalidUnaryOperandError{
				Operation:    expression.Operation,
				ExpectedType: expectedType,
				ActualType:   valueType,
				Range:        ast.NewRangeFromPositioned(expression.Expression),
			},
		)
	}

	switch expression.Operation {
	case ast.OperationNegate:
		expectedType := &BoolType{}
		if !IsSubType(valueType, expectedType) {
			reportInvalidUnaryOperator(expectedType)
		}
		return valueType

	case ast.OperationMinus:
		expectedType := &SignedNumberType{}
		if !IsSubType(valueType, expectedType) {
			reportInvalidUnaryOperator(expectedType)
		}

		return valueType

	case ast.OperationMove:
		if !valueType.IsInvalidType() &&
			!valueType.IsResourceType() {

			checker.report(
				&InvalidMoveOperationError{
					Range: ast.Range{
						StartPos: expression.StartPos,
						EndPos:   expression.Expression.StartPosition(),
					},
				},
			)
		}

		return valueType
	}

	panic(&unsupportedOperation{
		kind:      common.OperationKindUnary,
		operation: expression.Operation,
		Range:     ast.NewRangeFromPositioned(expression),
	})
}
