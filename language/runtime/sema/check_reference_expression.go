package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
)

// VisitReferenceExpression checks a reference expression `&t as T`,
// where `t` is the referenced expression, and `T` is the result type.
//
func (checker *Checker) VisitReferenceExpression(referenceExpression *ast.ReferenceExpression) ast.Repr {

	// Type-check the referenced expression

	referencedExpression := referenceExpression.Expression

	// Check that the referenced expression is an index expression and type-check it

	var referencedType Type
	var targetIsStorage bool

	// If the referenced expression is an index expression, it might be into storage

	indexExpression, isIndexExpression := referencedExpression.(*ast.IndexExpression)
	if isIndexExpression {
		var targetType Type
		referencedType, targetType = checker.visitIndexExpression(indexExpression, false)

		// The referenced expression will evaluate to an optional type if it is indexing into storage:
		// the result of the storage access is an optional.
		//
		// Unwrap the optional one level, but not infinitely

		if optionalReferencedType, ok := referencedType.(*OptionalType); ok {
			referencedType = optionalReferencedType.Type
		}

		// Check if the index expression's target expression is a storage type

		if !targetType.IsInvalidType() {
			_, targetIsStorage = targetType.(*StorageType)
		}
	} else {
		// If the referenced expression is not an index expression, check it normally

		referencedType = referencedExpression.Accept(checker).(Type)
	}

	checker.Elaboration.IsReferenceIntoStorage[referenceExpression] = targetIsStorage

	// Check that the referenced expression's type is a resource type

	if !referencedType.IsInvalidType() &&
		!referencedType.IsResourceType() {

		checker.report(
			&NonResourceTypeReferenceError{
				ActualType: referencedType,
				Range:      ast.NewRangeFromPositioned(referencedExpression),
			},
		)
	}

	// Check that the references expression's type is not optional

	if _, ok := referencedType.(*OptionalType); ok {

		checker.report(
			&OptionalTypeReferenceError{
				ActualType: referencedType,
				Range:      ast.NewRangeFromPositioned(referencedExpression),
			},
		)
	}

	// Check the result type and ensure it is a reference type

	resultType := checker.ConvertType(referenceExpression.Type)

	var referenceType *ReferenceType

	if !resultType.IsInvalidType() {
		var ok bool
		referenceType, ok = resultType.(*ReferenceType)
		if !ok {
			checker.report(
				&NonReferenceTypeReferenceError{
					ActualType: resultType,
					Range:      ast.NewRangeFromPositioned(referenceExpression.Type),
				},
			)
		} else if !targetIsStorage && referenceType.Storable {

			// References which are not into storage cannot be storable

			checker.report(
				&InvalidNonStorageStorableReferenceError{
					Range: ast.NewRangeFromPositioned(referenceExpression.Type),
				},
			)
		}
	}

	// Check that the referenced expression's type is a subtype of the result type

	if !referencedType.IsInvalidType() &&
		referenceType != nil &&
		!referenceType.Type.IsInvalidType() &&
		!IsSubType(referencedType, referenceType.Type) {

		checker.report(
			&TypeMismatchError{
				ExpectedType: referenceType.Type,
				ActualType:   referencedType,
				Range:        ast.NewRangeFromPositioned(referencedExpression),
			},
		)
	}

	if referenceType == nil {
		return &InvalidType{}
	}

	return referenceType
}
