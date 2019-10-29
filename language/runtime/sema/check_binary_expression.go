package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
)

func (checker *Checker) VisitBinaryExpression(expression *ast.BinaryExpression) ast.Repr {

	// The left-hand side is always evaluated.
	// However, the right-hand side might not necessarily be evaluated,
	// e.g. in boolean logic or in nil-coalescing

	leftType := expression.Left.Accept(checker).(Type)
	leftIsInvalid := leftType.IsInvalidType()

	operation := expression.Operation
	operationKind := binaryOperationKind(operation)

	unsupportedOperation := func() Type {
		panic(&unsupportedOperation{
			kind:      common.OperationKindBinary,
			operation: operation,
			Range:     ast.NewRangeFromPositioned(expression),
		})
	}

	switch operationKind {
	case BinaryOperationKindIntegerArithmetic,
		BinaryOperationKindIntegerComparison,
		BinaryOperationKindEquality,
		BinaryOperationKindConcatenation:

		// Right hand side will always be evaluated

		rightType := expression.Right.Accept(checker).(Type)
		rightIsInvalid := rightType.IsInvalidType()

		anyInvalid := leftIsInvalid || rightIsInvalid

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

		case BinaryOperationKindConcatenation:
			return checker.checkBinaryExpressionConcatenation(
				expression, operation, operationKind,
				leftType, rightType,
				leftIsInvalid, rightIsInvalid, anyInvalid,
			)

		default:
			return unsupportedOperation()
		}

	case BinaryOperationKindBooleanLogic,
		BinaryOperationKindNilCoalescing:

		// Right hand side will maybe be evaluated. That means that
		// resource invalidation and returns are not definite, but only potential

		rightType := checker.checkPotentiallyUnevaluated(func() Type {
			return expression.Right.Accept(checker).(Type)
		})

		rightIsInvalid := rightType.IsInvalidType()

		anyInvalid := leftIsInvalid || rightIsInvalid

		switch operationKind {
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

		default:
			return unsupportedOperation()
		}
	default:
		return unsupportedOperation()
	}
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
					Range:     ast.NewRangeFromPositioned(expression),
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
					Range:        ast.NewRangeFromPositioned(expression.Left),
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
					Range:        ast.NewRangeFromPositioned(expression.Right),
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
				Range:     ast.NewRangeFromPositioned(expression),
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
				Range:     ast.NewRangeFromPositioned(expression),
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
					Range:     ast.NewRangeFromPositioned(expression),
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
					Range:        ast.NewRangeFromPositioned(expression.Left),
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
					Range:        ast.NewRangeFromPositioned(expression.Right),
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
					Range:        ast.NewRangeFromPositioned(expression.Left),
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
						Range:        ast.NewRangeFromPositioned(expression.Right),
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
					Range:     ast.NewRangeFromPositioned(expression),
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
					Range:        ast.NewRangeFromPositioned(expression.Left),
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
					Range:        ast.NewRangeFromPositioned(expression.Right),
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
				Range:     ast.NewRangeFromPositioned(expression),
			},
		)
	}

	return leftType
}
