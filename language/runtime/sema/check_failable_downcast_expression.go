package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
)

func (checker *Checker) VisitCastingExpression(expression *ast.CastingExpression) ast.Repr {

	leftHandExpression := expression.Expression
	leftHandType := leftHandExpression.Accept(checker).(Type)

	checker.Elaboration.CastingStaticValueTypes[expression] = leftHandType

	rightHandTypeAnnotation := checker.ConvertTypeAnnotation(expression.TypeAnnotation)
	checker.checkTypeAnnotation(rightHandTypeAnnotation, expression.TypeAnnotation.StartPos)

	rightHandType := rightHandTypeAnnotation.Type

	checker.Elaboration.CastingTargetTypes[expression] = rightHandType

	switch expression.Operation {
	case ast.OperationFailableCast:
		// TODO: non-Any types (interfaces, wrapped (e.g Any?, [Any], etc.)) are not supported for now
		if _, ok := leftHandType.(*AnyType); !ok {
			checker.report(
				&UnsupportedTypeError{
					Type:  leftHandType,
					Range: ast.NewRangeFromPositioned(leftHandExpression),
				},
			)
		}

		return &OptionalType{Type: rightHandType}

	case ast.OperationCast:
		if !checker.IsTypeCompatible(leftHandExpression, leftHandType, rightHandType) {
			checker.report(
				&TypeMismatchError{
					ActualType:   leftHandType,
					ExpectedType: rightHandType,
					Range:        ast.NewRangeFromPositioned(leftHandExpression),
				},
			)
		}

		return rightHandType

	default:
		panic(errors.NewUnreachableError())
	}
}
