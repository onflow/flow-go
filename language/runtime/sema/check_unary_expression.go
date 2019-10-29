package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
)

func (checker *Checker) VisitUnaryExpression(expression *ast.UnaryExpression) ast.Repr {

	valueType := expression.Expression.Accept(checker).(Type)

	switch expression.Operation {
	case ast.OperationNegate:
		if !IsSubType(valueType, &BoolType{}) {
			checker.report(
				&InvalidUnaryOperandError{
					Operation:    expression.Operation,
					ExpectedType: &BoolType{},
					ActualType:   valueType,
					Range:        ast.NewRangeFromPositioned(expression.Expression),
				},
			)
		}
		return valueType

	case ast.OperationMinus:
		if !IsSubType(valueType, &IntegerType{}) {
			checker.report(
				&InvalidUnaryOperandError{
					Operation:    expression.Operation,
					ExpectedType: &IntegerType{},
					ActualType:   valueType,
					Range:        ast.NewRangeFromPositioned(expression.Expression),
				},
			)
		}
		return valueType

	case ast.OperationMove:
		if !valueType.IsResourceType() {
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
