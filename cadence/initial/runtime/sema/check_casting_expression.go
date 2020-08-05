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
	"github.com/onflow/cadence-initial/runtime/errors"
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

		if expression.Operation == ast.OperationFailableCast {

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
	case ast.OperationFailableCast, ast.OperationForceCast:

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

			if !FailableCastCanSucceed(leftHandType, rightHandType) {

				checker.report(
					&TypeMismatchError{
						ActualType:   leftHandType,
						ExpectedType: rightHandType,
						Range:        ast.NewRangeFromPositioned(leftHandExpression),
					},
				)
			}
		}

		if expression.Operation == ast.OperationFailableCast {
			return &OptionalType{Type: rightHandType}
		}

		return rightHandType

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

// FailableCastCanSucceed checks a failable (dynamic) cast, i.e. a cast that might succeed at run-time.
// It returns true if the cast from subType to superType could potentially succeed at run-time,
// and returns false if the cast will definitely always fail.
//
func FailableCastCanSucceed(subType, superType Type) bool {

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
				return FailableCastCanSucceed(typedSubType.Type, typedSuperType.Type)
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

	case *RestrictedType:

		switch typedSubType := subType.(type) {
		case *RestrictedType:

			switch restrictedSuperType := typedSuperType.Type.(type) {

			case *AnyResourceType:
				// A restricted type `T{Us}`
				// is a subtype of a restricted type `AnyResource{Vs}`:
				//
				// When `T == AnyResource || T == Any`:
				// if the run-time type conforms to `Vs`
				//
				// When `T != AnyResource && T != Any`:
				// if `T` conforms to `Vs`.
				// `Us` and `Vs` do *not* have to be subsets.

				switch typedSubType.Type.(type) {
				case *AnyResourceType, *AnyType:
					return true
				default:
					if typedInnerSubType, ok := typedSubType.Type.(*CompositeType); ok {

						return IsSubType(typedInnerSubType, restrictedSuperType) &&
							typedSuperType.RestrictionSet().
								IsSubsetOf(typedInnerSubType.ExplicitInterfaceConformanceSet())
					}
				}

			case *AnyStructType:
				// A restricted type `T{Us}`
				// is a subtype of a restricted type `AnyStruct{Vs}`:
				//
				// When `T == AnyStruct || T == Any`: if the run-time type conforms to `Vs`
				//
				// When `T != AnyStruct && T != Any`: if `T` conforms to `Vs`.
				// `Us` and `Vs` do *not* have to be subsets.

				switch typedSubType.Type.(type) {
				case *AnyStructType, *AnyType:
					return true
				default:
					if typedInnerSubType, ok := typedSubType.Type.(*CompositeType); ok {

						return IsSubType(typedInnerSubType, restrictedSuperType) &&
							typedSuperType.RestrictionSet().
								IsSubsetOf(typedInnerSubType.ExplicitInterfaceConformanceSet())
					}
				}

			case *AnyType:
				// A restricted type `T{Us}`
				// is a subtype of a restricted type `Any{Vs}`:
				//
				// When `T == AnyResource || T == AnyStruct || T == Any`:
				// if the run-time type conforms to `Vs`
				//
				// When `T != AnyResource && T != AnyStruct && T != Any`:
				// if `T` conforms to `Vs`.
				// `Us` and `Vs` do *not* have to be subsets.

				switch typedSubType.Type.(type) {
				case *AnyResourceType, *AnyStructType, *AnyType:
					return true
				default:
					if typedInnerSubType, ok := typedSubType.Type.(*CompositeType); ok {

						return IsSubType(typedInnerSubType, restrictedSuperType) &&
							typedSuperType.RestrictionSet().
								IsSubsetOf(typedInnerSubType.ExplicitInterfaceConformanceSet())
					}
				}

			default:

				// A restricted type `T{Us}`
				// is a subtype of a restricted type `V{Ws}`:
				//
				// When `T == AnyResource || T == AnyStruct || T == Any`:
				// if the run-time type is `V`.
				//
				// When `T != AnyResource && T != AnyStruct && T != Any`:
				// if `T == V`.
				// `Us` and `Ws` do *not* have to be subsets:
				// The owner may freely restrict and unrestrict.

				switch typedSubType.Type.(type) {
				case *AnyResourceType, *AnyStructType, *AnyType:
					return true
				default:
					return typedSubType.Type.Equal(typedSuperType.Type)
				}
			}

		case *CompositeType:

			switch typedSuperType.Type.(type) {
			case *AnyResourceType, *AnyStructType, *AnyType:

				// An unrestricted type `T`
				// is a subtype of a restricted type `AnyResource{Us}` / `AnyStruct{Us}` / `Any{Us}`:
				//
				// When `T != AnyResource && T != AnyStruct && T != Any`:
				// if `T` is a subtype of the restricted supertype,
				// and `T` conforms to `Us`.

				return IsSubType(typedSubType, typedSuperType.Type) &&
					typedSuperType.RestrictionSet().
						IsSubsetOf(typedSubType.ExplicitInterfaceConformanceSet())

			default:

				// An unrestricted type `T`
				// is a subtype of a restricted type `U{Vs}`:
				//
				// When `T != AnyResource && T != AnyStruct && T != Any`:
				// if `T == U`.

				return typedSubType.Equal(typedSuperType.Type)
			}

		case *AnyResourceType, *AnyStructType, *AnyType:

			switch typedSuperType.Type.(type) {
			case *AnyResourceType, *AnyStructType, *AnyType:

				// An unrestricted type `T`
				// is a subtype of a restricted type `AnyResource{Us}` / `AnyStruct{Us}` / `Any{Us}`:
				//
				// When `T == AnyResource || T == AnyStruct` || T == Any`:
				// if the run-time type conforms to `Vs`

				return true

			default:

				// An unrestricted type `T`
				// is a subtype of a restricted type `U{Vs}`:
				//
				// When `T == AnyResource || T == AnyStruct || T == Any`:
				// if the run-time type is U.

				// NOTE: inverse!

				return IsSubType(typedSuperType.Type, typedSubType)
			}
		}

	case *CompositeType:

		switch typedSubType := subType.(type) {
		case *RestrictedType:

			// A restricted type `T{Us}`
			// is a subtype of an unrestricted type `V`:
			//
			// When `T == AnyResource || T == AnyStruct || T == Any`:
			// if the run-time type is V.
			//
			// When `T != AnyResource && T != AnyStruct && T != Any`:
			// if `T == V`.
			// The owner may freely unrestrict.

			switch typedSubType.Type.(type) {
			case *AnyResourceType, *AnyStructType, *AnyType:
				return true

			default:
				return typedSubType.Type.Equal(typedSuperType)
			}
		}

	case *AnyResourceType, *AnyStructType:

		// A restricted type `T{Us}`
		// or unrestricted type `T`
		// is a subtype of the type `AnyResource` / `AnyStruct`:
		// if `T` is `AnyType`, or `T` is a subtype of `AnyResource` / `AnyStruct`.

		innerSubtype := subType
		if restrictedSubType, ok := subType.(*RestrictedType); ok {
			innerSubtype = restrictedSubType.Type
		}

		if _, ok := innerSubtype.(*AnyType); ok {
			return true
		}

		return IsSubType(innerSubtype, superType)

	case *AnyType:
		return true
	}

	return true
}
