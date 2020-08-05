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

//go:generate stringer -type=BinaryOperationKind

type BinaryOperationKind uint

const (
	BinaryOperationKindUnknown BinaryOperationKind = iota
	BinaryOperationKindArithmetic
	BinaryOperationKindNonEqualityComparison
	BinaryOperationKindBooleanLogic
	BinaryOperationKindEquality
	BinaryOperationKindNilCoalescing
	BinaryOperationKindBitwise
)

func binaryOperationKind(operation ast.Operation) BinaryOperationKind {
	switch operation {
	case ast.OperationPlus,
		ast.OperationMinus,
		ast.OperationMod,
		ast.OperationMul,
		ast.OperationDiv:

		return BinaryOperationKindArithmetic

	case ast.OperationLess,
		ast.OperationLessEqual,
		ast.OperationGreater,
		ast.OperationGreaterEqual:

		return BinaryOperationKindNonEqualityComparison

	case ast.OperationOr,
		ast.OperationAnd:

		return BinaryOperationKindBooleanLogic

	case ast.OperationEqual,
		ast.OperationNotEqual:

		return BinaryOperationKindEquality

	case ast.OperationNilCoalesce:
		return BinaryOperationKindNilCoalescing

	case ast.OperationBitwiseOr,
		ast.OperationBitwiseXor,
		ast.OperationBitwiseAnd,
		ast.OperationBitwiseLeftShift,
		ast.OperationBitwiseRightShift:

		return BinaryOperationKindBitwise
	}

	panic(errors.NewUnreachableError())
}
