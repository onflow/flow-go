package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
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

			if !FailableCanSucceed(leftHandType, rightHandType) {

				checker.report(
					&TypeMismatchError{
						ActualType:   leftHandType,
						ExpectedType: rightHandType,
						Range:        ast.NewRangeFromPositioned(leftHandExpression),
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

// FailableCanSucceed checks a failable (dynamic) cast, i.e. a cast that might succeed at run-time.
// It returns true if the cast from subType to superType could potentially succeed at run-time,
// and returns false if the cast will definitely always fail.
//
func FailableCanSucceed(subType, superType Type) bool {
	// TODO: report impossible casts, e.g.
	//   - primitive/composite T -> composite U where T != U
	//   - array/dictionary where key or value cast is impossible
	//   => move checks from interpreter here

	switch typedSuperType := superType.(type) {
	case *ReferenceType:
		// References types are only subtypes of reference types

		if typedSubType, ok := subType.(*ReferenceType); ok {
			// An authorized reference type `auth &T`
			// is a subtype of a reference type `&U` (authorized or non-authorized),
			// if `T` is a subtype of `U`

			if typedSubType.Authorized {
				return FailableCanSucceed(typedSubType.Type, typedSuperType.Type)
			}

			// An unauthorized reference type is not a subtype of an authorized reference type.
			// Not even dynamically.
			//
			// The holder of the reference may not gain more permissions.

			if typedSuperType.Authorized {
				return false
			}

			// A failable cast from an unauthorized reference type
			// to an unauthorized reference type
			// has the same semantics as a static/non-failable cast

			return IsSubType(subType, superType)
		}

	case *RestrictedResourceType:

		switch typedSubType := subType.(type) {
		case *RestrictedResourceType:

			if _, ok := typedSuperType.Type.(*AnyResourceType); ok {

				// A restricted resource type `T{Us}`
				// is a subtype of a restricted resource type `AnyResource{Vs}`:
				//
				// When `T == AnyResource`: if the run-time type conforms to `Vs`
				//
				// When `T != AnyResource`: if `T` conforms to `Vs`.
				// `Us` and `Vs` do *not* have to be subsets.

				if _, ok := typedSubType.Type.(*AnyResourceType); ok {
					return true
				} else {
					if typedInnerSubType, ok := typedSubType.Type.(*CompositeType); ok {

						return typedSuperType.RestrictionSet().
							IsSubsetOf(typedInnerSubType.ConformanceSet())
					}
				}

			} else {

				// A restricted resource type `T{Us}`
				// is a subtype of a restricted resource type `V{Ws}`:
				//
				// When `T == AnyResource`: if the run-time type is `V`.
				//
				// When `T != AnyResource`: if `T == V`.
				// `Us` and `Ws` do *not* have to be subsets:
				// The owner of the resource may freely restrict and unrestrict the resource.
				//

				if _, ok := typedSubType.Type.(*AnyResourceType); ok {
					return true
				} else {
					return typedSubType.Type.Equal(typedSuperType.Type)
				}
			}

		case *CompositeType:
			if typedSubType.Kind == common.CompositeKindResource {

				if _, ok := typedSuperType.Type.(*AnyResourceType); ok {

					// An unrestricted resource type `T`
					// is a subtype of a restricted resource type `AnyResource{Us}`:
					//
					// When `T != AnyResource`: if `T` conforms to `Us`.

					return typedSuperType.RestrictionSet().
						IsSubsetOf(typedSubType.ConformanceSet())

				} else {

					// An unrestricted resource type `T`
					// is a subtype of a restricted resource type `U{Vs}`:
					//
					// When `T != AnyResource`: if `T == U`.

					return typedSubType.Equal(typedSuperType.Type)
				}
			}

		case *AnyResourceType:
			// Commented out because the result is true in both cases,
			// so avoid unnecessary checks.

			//if _, ok := typedSuperType.Type.(*AnyResourceType); ok {
			//
			//	// An unrestricted resource type `T`
			//	// is a subtype of a restricted resource type `AnyResource{Us}`:
			//	//
			//	// When `T == AnyResource`: if the run-time type conforms to `Vs`
			//
			//	return true
			//
			//} else {
			//
			//	// An unrestricted resource type `T`
			//	// is a subtype of a restricted resource type `U{Vs}`:
			//	//
			//	// When `T == AnyResource`: if the run-time type is U.
			//
			//	return true
			//}
		}

	case *CompositeType:
		if typedSuperType.Kind == common.CompositeKindResource {

			switch typedSubType := subType.(type) {
			case *RestrictedResourceType:

				// A restricted resource type `T{Us}`
				// is a subtype of an unrestricted resource type `V`:
				//
				// When `T != AnyResource`: if `T == V`.
				// The owner of the resource may freely unrestrict the resource.
				//
				// When `T == AnyResource`: if the run-time type is V.

				if _, ok := typedSubType.Type.(*AnyResourceType); !ok {
					return typedSubType.Type.Equal(typedSuperType)
				}
			}
		}

	case *AnyResourceType:

		// A restricted resource type `T{Us}`
		// or unrestricted resource type `T`
		// is a subtype of the type `AnyResource`: always.

		return true
	}

	return true
}
