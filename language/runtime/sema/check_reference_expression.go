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

	indexExpression, isIndexExpression := referencedExpression.(*ast.IndexExpression)
	if isIndexExpression {
		var targetType Type
		referencedType, targetType = checker.visitIndexExpression(indexExpression, false)

		// The referenced expression will evaluate to an optional type, unwrap it

		referencedType = UnwrapOptionalType(referencedType)

		// Check that the index expression's target expression is a storage type

		if _, isStorageType := targetType.(*StorageType); !isStorageType {
			checker.report(
				&NonStorageReferenceError{
					Range: ast.NewRangeFromPositioned(indexExpression.TargetExpression),
				},
			)
		}
	} else {
		checker.report(
			&NonStorageReferenceError{
				Range: ast.NewRangeFromPositioned(referencedExpression),
			},
		)

		// If the referenced expression is not an index expression, still type check it

		referencedType = UnwrapOptionalType(referencedExpression.Accept(checker).(Type))
	}

	// Check the result type

	resultType := checker.ConvertType(referenceExpression.Type)

	// Check that the referenced expression's type is a resource or resource interface

	if !referencedType.IsInvalidType() &&
		!referencedType.IsResourceType() {

		checker.report(
			&NonResourceReferenceError{
				ActualType: referencedType,
				Range:      ast.NewRangeFromPositioned(referencedExpression),
			},
		)
	}

	// Check that the result type is a resource or resource interface

	if !resultType.IsInvalidType() &&
		!resultType.IsResourceType() {

		checker.report(
			&NonResourceReferenceError{
				ActualType: resultType,
				Range:      ast.NewRangeFromPositioned(referencedExpression),
			},
		)
	}

	// Check that the referenced expression's type is a subtype of the result type

	if !referencedType.IsInvalidType() &&
		!resultType.IsInvalidType() &&
		!IsSubType(referencedType, resultType) {

		checker.report(
			&TypeMismatchError{
				ExpectedType: resultType,
				ActualType:   referencedType,
				Range:        ast.NewRangeFromPositioned(referencedExpression),
			},
		)
	}

	return &ReferenceType{Type: resultType}
}
