/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package sema

import (
	"github.com/onflow/cadence-initial/runtime/ast"
	"github.com/onflow/cadence-initial/runtime/common"
	"github.com/onflow/cadence-initial/runtime/errors"
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
	case BinaryOperationKindArithmetic,
		BinaryOperationKindNonEqualityComparison,
		BinaryOperationKindEquality,
		BinaryOperationKindBitwise:

		// Right hand side will always be evaluated

		rightType := expression.Right.Accept(checker).(Type)
		rightIsInvalid := rightType.IsInvalidType()

		anyInvalid := leftIsInvalid || rightIsInvalid

		switch operationKind {
		case BinaryOperationKindArithmetic,
			BinaryOperationKindNonEqualityComparison,
			BinaryOperationKindBitwise:

			return checker.checkBinaryExpressionArithmeticOrNonEqualityComparisonOrBitwise(
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

		default:
			return unsupportedOperation()
		}

	case BinaryOperationKindBooleanLogic,
		BinaryOperationKindNilCoalescing:

		// The evaluation of the right-hand side is not guaranteed.
		// That means that resource invalidation and returns
		// are not definite, but only potential.

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

func (checker *Checker) checkBinaryExpressionArithmeticOrNonEqualityComparisonOrBitwise(
	expression *ast.BinaryExpression,
	operation ast.Operation,
	operationKind BinaryOperationKind,
	leftType, rightType Type,
	leftIsInvalid, rightIsInvalid, anyInvalid bool,
) Type {
	// check both types are number/integer subtypes

	var expectedSuperType Type

	switch operationKind {
	case BinaryOperationKindArithmetic,
		BinaryOperationKindNonEqualityComparison:

		expectedSuperType = &NumberType{}

	case BinaryOperationKindBitwise:
		expectedSuperType = &IntegerType{}

	default:
		panic(errors.NewUnreachableError())
	}

	leftIsNumber := IsSubType(leftType, expectedSuperType)
	rightIsNumber := IsSubType(rightType, expectedSuperType)

	reportedInvalidOperands := false

	if !leftIsNumber && !rightIsNumber {
		if !anyInvalid {
			checker.report(
				&InvalidBinaryOperandsError{
					Operation: operation,
					LeftType:  leftType,
					RightType: rightType,
					Range:     ast.NewRangeFromPositioned(expression),
				},
			)
			reportedInvalidOperands = true
		}
	} else if !leftIsNumber {
		if !leftIsInvalid {
			checker.report(
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideLeft,
					ExpectedType: expectedSuperType,
					ActualType:   leftType,
					Range:        ast.NewRangeFromPositioned(expression.Left),
				},
			)
		}
	} else if !rightIsNumber {
		if !rightIsInvalid {
			checker.report(
				&InvalidBinaryOperandError{
					Operation:    operation,
					Side:         common.OperandSideRight,
					ExpectedType: expectedSuperType,
					ActualType:   rightType,
					Range:        ast.NewRangeFromPositioned(expression.Right),
				},
			)
		}
	}

	// check both types are equal

	if !reportedInvalidOperands &&
		!anyInvalid &&
		!leftType.Equal(rightType) {

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
	case BinaryOperationKindArithmetic,
		BinaryOperationKindBitwise:

		return leftType

	case BinaryOperationKindNonEqualityComparison:
		return &BoolType{}

	default:
		panic(errors.NewUnreachableError())
	}
}

func (checker *Checker) checkBinaryExpressionEquality(
	expression *ast.BinaryExpression,
	operation ast.Operation,
	operationKind BinaryOperationKind,
	leftType, rightType Type,
	leftIsInvalid, rightIsInvalid, anyInvalid bool,
) (resultType Type) {

	resultType = &BoolType{}

	if anyInvalid {
		return
	}

	if !AreCompatibleEquatableTypes(leftType, rightType) {
		checker.report(
			&InvalidBinaryOperandsError{
				Operation: operation,
				LeftType:  leftType,
				RightType: rightType,
				Range:     ast.NewRangeFromPositioned(expression),
			},
		)
	}

	checker.checkUnusedExpressionResourceLoss(leftType, expression.Left)
	checker.checkUnusedExpressionResourceLoss(rightType, expression.Right)

	return
}

func (checker *Checker) checkBinaryExpressionBooleanLogic(
	expression *ast.BinaryExpression,
	operation ast.Operation,
	operationKind BinaryOperationKind,
	leftType, rightType Type,
	leftIsInvalid, rightIsInvalid, anyInvalid bool,
) Type {
	// check both types are boolean subtypes

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

		checker.recordResourceInvalidation(
			expression.Left,
			leftType,
			ResourceInvalidationKindMove,
		)

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
	}
	canNarrow := false

	if !rightIsInvalid {

		if rightType.IsResourceType() {

			checker.report(
				&InvalidNilCoalescingRightResourceOperandError{
					Range: ast.NewRangeFromPositioned(expression.Right),
				},
			)
		}

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
