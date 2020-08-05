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
)

// VisitReferenceExpression checks a reference expression `&t as T`,
// where `t` is the referenced expression, and `T` is the result type.
//
func (checker *Checker) VisitReferenceExpression(referenceExpression *ast.ReferenceExpression) ast.Repr {

	// Type-check the referenced expression

	referencedExpression := referenceExpression.Expression

	// Check that the referenced expression is an index expression and type-check it

	var referencedType Type

	// If the referenced expression is an index expression, it might be into storage

	indexExpression, isIndexExpression := referencedExpression.(*ast.IndexExpression)
	if isIndexExpression {
		referencedType = checker.visitIndexExpression(indexExpression, false)

		// The referenced expression will evaluate to an optional type if it is indexing into storage:
		// the result of the storage access is an optional.
		//
		// Unwrap the optional one level, but not infinitely

		if optionalReferencedType, ok := referencedType.(*OptionalType); ok {
			referencedType = optionalReferencedType.Type
		}

		// Check if the index expression's target expression is a storage type

	} else {
		// If the referenced expression is not an index expression, check it normally

		referencedType = referencedExpression.Accept(checker).(Type)
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
