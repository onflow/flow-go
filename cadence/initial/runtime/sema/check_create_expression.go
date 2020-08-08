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
	"example.com/cadence-initial/runtime/ast"
	"example.com/cadence-initial/runtime/common"
)

func (checker *Checker) VisitCreateExpression(expression *ast.CreateExpression) ast.Repr {
	inCreate := checker.inCreate
	checker.inCreate = true
	defer func() {
		checker.inCreate = inCreate
	}()

	// TODO: maybe check that invoked expression is a composite constructor

	invocation := expression.InvocationExpression

	ty := invocation.Accept(checker).(Type)

	if ty.IsInvalidType() {
		return ty
	}

	// Check that the created expression is a resource

	compositeType, isCompositeType := ty.(*CompositeType)

	// NOTE: not using `isResourceType`,
	// as only direct resource types can be constructed

	if !isCompositeType || compositeType.Kind != common.CompositeKindResource {

		checker.report(
			&InvalidConstructionError{
				Range: ast.NewRangeFromPositioned(invocation),
			},
		)

		return ty
	}

	checker.checkResourceCreationOrDestruction(compositeType, invocation)

	return ty
}

// checkResourceCreationOrDestruction checks that the create or destroy expression occurs
// in the same contract that declares the composite, or if not contained in a contract,
// it occurs at least in the same location

func (checker *Checker) checkResourceCreationOrDestruction(compositeType *CompositeType, positioned ast.HasPosition) {

	contractType := containingContractKindedType(compositeType)

	if contractType == nil {
		if ast.LocationsMatch(compositeType.Location, checker.Location) {
			return
		}
	} else {
		if checker.containerTypes[contractType] {
			return
		}
	}

	checker.report(
		&InvalidResourceCreationError{
			Type:  compositeType,
			Range: ast.NewRangeFromPositioned(positioned),
		},
	)
}
