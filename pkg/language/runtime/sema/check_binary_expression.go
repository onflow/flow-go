package sema

import (
	"github.com/dapperlabs/flow-go/pkg/language/runtime/ast"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/common"
	"github.com/dapperlabs/flow-go/pkg/language/runtime/errors"
)

func (checker *Checker) visitBinaryOperation(expr *ast.BinaryExpression) (left, right Type) {
	left = expr.Left.Accept(checker).(Type)
	right = expr.Right.Accept(checker).(Type)
	return
}

func (checker *Checker) VisitBinaryExpression(expression *ast.BinaryExpression) ast.Repr {

	leftType, rightType := checker.visitBinaryOperation(expression)

	leftIsInvalid := IsInvalidType(leftType)
	rightIsInvalid := IsInvalidType(rightType)
	anyInvalid := leftIsInvalid || rightIsInvalid

	operation := expression.Operation
	operationKind := binaryOperationKind(operation)

	switch operationKind {
	case BinaryOperationKindIntegerArithmetic,
		BinaryOperationKindIntegerComparison:

		return checker.checkBinaryExpressionIntegerArithmeticOrComparison(
			expression, operation, operationKind,
			leftType, rightType,
			leftIsInvalid, rightIsInvalid, anyInvalid,
		)

	case BinaryOperationKindEquality:

		return checker.checkBinaryExpressionEquality(
			expression, operation, operationKind,
			leftType, rightType,
			leftIsInvalid, rightIsInvalid, anyInvalid,
		)

	case BinaryOperationKindBooleanLogic:

		return checker.checkBinaryExpressionBooleanLogic(
			expression, operation, operationKind,
			leftType, rightType,
			leftIsInvalid, rightIsInvalid, anyInvalid,
		)

	case BinaryOperationKindNilCoalescing:
		resultType := checker.checkBinaryExpressionNilCoalescing(
			expression, operation, operationKind,
			leftType, rightType,
			leftIsInvalid, rightIsInvalid, anyInvalid,
		)

		checker.Elaboration.BinaryExpressionResultTypes[expression] = resultType
		checker.Elaboration.BinaryExpressionRightTypes[expression] = rightType

		return resultType

	case BinaryOperationKindConcatenation:
		return checker.checkBinaryExpressionConcatenation(
			expression, operation, operationKind,
			leftType, rightType,
			leftIsInvalid, rightIsInvalid, anyInvalid,
		)
	}

	panic(&unsupportedOperation{
		kind:      common.OperationKindBinary,
		operation: operation,
		Range: ast.Range{
			StartPos: expression.StartPosition(),
			EndPos:   expression.EndPosition(),
		},
	})
}

func (checker *Checker) checkBinaryExpressionIntegerArithmeticOrComparison(
	expression *ast.BinaryExpression,
	operation ast.Operation,
	operationKind BinaryOperationKind,
	leftType, rightType Type,
	leftIsInvalid, rightIsInvalid, anyInvalid bool,
) Type {
	// check both types are integer subtypes

	leftIsInteger := IsSubType(leftType, &IntegerType{})
	rightIsInteger := IsSubType(rightType, &IntegerType{})

	if !leftIsInteger && !rightIsInteger {
		if !anyInvalid {
			checker.report(
				&InvalidBinaryOperandsError{
					Operation: operation,
					LeftType:  leftType,
					RightType: rightType,
					Range: ast.Range{
						StartPos: expression.StartPosition(),
						EndPos:   expression.EndPosition(),
					},
				},
			)
		}
	} else if !leftIsInteger {
		if !leftIsInvalid {
			checker.report(
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideLeft,
					ExpectedType: &IntegerType{},
					ActualType:   leftType,
					Range: ast.Range{
						StartPos: expression.Left.StartPosition(),
						EndPos:   expression.Left.EndPosition(),
					},
				},
			)
		}
	} else if !rightIsInteger {
		if !rightIsInvalid {
			checker.report(
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideRight,
					ExpectedType: &IntegerType{},
					ActualType:   rightType,
					Range: ast.Range{
						StartPos: expression.Right.StartPosition(),
						EndPos:   expression.Right.EndPosition(),
					},
				},
			)
		}
	}

	// check both types are equal
	if !anyInvalid && !leftType.Equal(rightType) {
		checker.report(
			&InvalidBinaryOperandsError{
				Operation: operation,
				LeftType:  leftType,
				RightType: rightType,
				Range: ast.Range{
					StartPos: expression.StartPosition(),
					EndPos:   expression.EndPosition(),
				},
			},
		)
	}

	switch operationKind {
	case BinaryOperationKindIntegerArithmetic:
		return leftType
	case BinaryOperationKindIntegerComparison:
		return &BoolType{}
	}

	panic(&errors.UnreachableError{})
}

func (checker *Checker) checkBinaryExpressionEquality(
	expression *ast.BinaryExpression,
	operation ast.Operation,
	operationKind BinaryOperationKind,
	leftType, rightType Type,
	leftIsInvalid, rightIsInvalid, anyInvalid bool,
) (resultType Type) {
	// check both types are equal, and boolean subtypes or integer subtypes

	resultType = &BoolType{}

	if !anyInvalid &&
		leftType != nil &&
		!(IsValidEqualityType(leftType) &&
			AreCompatibleEqualityTypes(leftType, rightType)) {

		checker.report(
			&InvalidBinaryOperandsError{
				Operation: operation,
				LeftType:  leftType,
				RightType: rightType,
				Range: ast.Range{
					StartPos: expression.StartPosition(),
					EndPos:   expression.EndPosition(),
				},
			},
		)
	}

	return
}

func (checker *Checker) checkBinaryExpressionBooleanLogic(
	expression *ast.BinaryExpression,
	operation ast.Operation,
	operationKind BinaryOperationKind,
	leftType, rightType Type,
	leftIsInvalid, rightIsInvalid, anyInvalid bool,
) Type {
	// check both types are integer subtypes

	leftIsBool := IsSubType(leftType, &BoolType{})
	rightIsBool := IsSubType(rightType, &BoolType{})

	if !leftIsBool && !rightIsBool {
		if !anyInvalid {
			checker.report(
				&InvalidBinaryOperandsError{
					Operation: operation,
					LeftType:  leftType,
					RightType: rightType,
					Range: ast.Range{
						StartPos: expression.StartPosition(),
						EndPos:   expression.EndPosition(),
					},
				},
			)
		}
	} else if !leftIsBool {
		if !leftIsInvalid {
			checker.report(
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideLeft,
					ExpectedType: &BoolType{},
					ActualType:   leftType,
					Range: ast.Range{
						StartPos: expression.Left.StartPosition(),
						EndPos:   expression.Left.EndPosition(),
					},
				},
			)
		}
	} else if !rightIsBool {
		if !rightIsInvalid {
			checker.report(
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideRight,
					ExpectedType: &BoolType{},
					ActualType:   rightType,
					Range: ast.Range{
						StartPos: expression.Right.StartPosition(),
						EndPos:   expression.Right.EndPosition(),
					},
				},
			)
		}
	}

	return &BoolType{}
}

func (checker *Checker) checkBinaryExpressionNilCoalescing(
	expression *ast.BinaryExpression,
	operation ast.Operation,
	operationKind BinaryOperationKind,
	leftType, rightType Type,
	leftIsInvalid, rightIsInvalid, anyInvalid bool,
) Type {
	leftOptional, leftIsOptional := leftType.(*OptionalType)

	if !leftIsInvalid {
		if !leftIsOptional {
			checker.report(
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideLeft,
					ExpectedType: &OptionalType{},
					ActualType:   leftType,
					Range: ast.Range{
						StartPos: expression.Left.StartPosition(),
						EndPos:   expression.Left.EndPosition(),
					},
				},
			)
		}
	}

	if leftIsInvalid || !leftIsOptional {
		return &InvalidType{}
	}

	leftInner := leftOptional.Type

	if _, ok := leftInner.(*NeverType); ok {
		return rightType
	} else {
		canNarrow := false

		if !rightIsInvalid {
			if !IsSubType(rightType, leftOptional) {
				checker.report(
					&InvalidBinaryOperandError{
						Operation:    operation,
						Side:         common.OperandSideRight,
						ExpectedType: leftOptional,
						ActualType:   rightType,
						Range: ast.Range{
							StartPos: expression.Right.StartPosition(),
							EndPos:   expression.Right.EndPosition(),
						},
					},
				)
			} else {
				canNarrow = IsSubType(rightType, leftInner)
			}
		}

		if !canNarrow {
			return leftOptional
		}
		return leftInner
	}
}

func (checker *Checker) checkBinaryExpressionConcatenation(
	expression *ast.BinaryExpression,
	operation ast.Operation,
	operationKind BinaryOperationKind,
	leftType, rightType Type,
	leftIsInvalid, rightIsInvalid, anyInvalid bool,
) Type {

	// check both types are concatenatable
	leftIsConcat := IsConcatenatableType(leftType)
	rightIsConcat := IsConcatenatableType(rightType)

	if !leftIsConcat && !rightIsConcat {
		if !anyInvalid {
			checker.report(
				&InvalidBinaryOperandsError{
					Operation: operation,
					LeftType:  leftType,
					RightType: rightType,
					Range: ast.Range{
						StartPos: expression.StartPosition(),
						EndPos:   expression.EndPosition(),
					},
				},
			)
		}
	} else if !leftIsConcat {
		if !leftIsInvalid {
			checker.report(
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideLeft,
					ExpectedType: rightType,
					ActualType:   leftType,
					Range: ast.Range{
						StartPos: expression.Left.StartPosition(),
						EndPos:   expression.Left.EndPosition(),
					},
				},
			)
		}
	} else if !rightIsConcat {
		if !rightIsInvalid {
			checker.report(
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideRight,
					ExpectedType: leftType,
					ActualType:   rightType,
					Range: ast.Range{
						StartPos: expression.Right.StartPosition(),
						EndPos:   expression.Right.EndPosition(),
					},
				},
			)
		}
	}

	// check both types are equal
	if !leftType.Equal(rightType) {
		checker.report(
			&InvalidBinaryOperandsError{
				Operation: operation,
				LeftType:  leftType,
				RightType: rightType,
				Range: ast.Range{
					StartPos: expression.StartPosition(),
					EndPos:   expression.EndPosition(),
				},
			},
		)
	}

	return leftType
}
