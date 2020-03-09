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
	checker.checkTypeAnnotation(rightHandTypeAnnotation, expression.TypeAnnotation)

	rightHandType := rightHandTypeAnnotation.Type

	checker.Elaboration.CastingTargetTypes[expression] = rightHandType

	if leftHandType.IsResourceType() {
		checker.recordResourceInvalidation(
			leftHandExpression,
			leftHandType,
			ResourceInvalidationKindMove,
		)

		// If the failable casted type is a resource, the failable cast expression
		// must occur in an optional binding, i.e. inside a variable declaration
		// as the if-statement test element

		if expression.Operation != ast.OperationCast {

			if expression.ParentVariableDeclaration == nil ||
				expression.ParentVariableDeclaration.ParentIfStatement == nil {

				checker.report(
					&InvalidFailableResourceDowncastOutsideOptionalBindingError{
						Range: ast.NewRangeFromPositioned(expression),
					},
				)
			}
		}
	}

	switch expression.Operation {
	case ast.OperationFailableCast:

		if !leftHandType.IsInvalidType() &&
			!rightHandType.IsInvalidType() {

			if leftHandType.IsResourceType() {
				if !rightHandType.IsResourceType() {
					checker.report(
						&AlwaysFailingNonResourceCastingTypeError{
							ValueType:  leftHandType,
							TargetType: rightHandType,
							Range:      ast.NewRangeFromPositioned(expression.TypeAnnotation),
						},
					)
				}
			} else {
				if rightHandType.IsResourceType() {
					checker.report(
						&AlwaysFailingResourceCastingTypeError{
							ValueType:  leftHandType,
							TargetType: rightHandType,
							Range:      ast.NewRangeFromPositioned(expression.TypeAnnotation),
						},
					)
				}
			}
		}

		// TODO: support more types

		if !leftHandType.IsInvalidType() {
			switch leftHandType.(type) {
			case *AnyStructType, *AnyResourceType, *RestrictedResourceType,
				*CompositeType, *InterfaceType,
				*VoidType, *StringType, *BoolType, *AddressType,
				*VariableSizedType, *ConstantSizedType,
				*DictionaryType, *OptionalType:

				break

			default:
				if IsSubType(leftHandType, &NumberType{}) {
					break
				}

				checker.report(
					&UnsupportedCastedTypeError{
						Type:  leftHandType,
						Range: ast.NewRangeFromPositioned(leftHandExpression),
					},
				)
			}
		}

		return &OptionalType{Type: rightHandType}

	case ast.OperationCast:
		if !leftHandType.IsInvalidType() &&
			!rightHandType.IsInvalidType() &&
			!checker.checkTypeCompatibility(leftHandExpression, leftHandType, rightHandType) {

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
