package sema

import (
	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/errors"
)

//go:generate stringer -type=BinaryOperationKind

type BinaryOperationKind int

const (
	BinaryOperationKindUnknown BinaryOperationKind = iota
	BinaryOperationKindIntegerArithmetic
	BinaryOperationKindIntegerComparison
	BinaryOperationKindBooleanLogic
	BinaryOperationKindEquality
	BinaryOperationKindNilCoalescing
	BinaryOperationKindConcatenation
)

func binaryOperationKind(operation ast.Operation) BinaryOperationKind {
	switch operation {
	case ast.OperationPlus,
		ast.OperationMinus,
		ast.OperationMod,
		ast.OperationMul,
		ast.OperationDiv:

		return BinaryOperationKindIntegerArithmetic

	case ast.OperationLess,
		ast.OperationLessEqual,
		ast.OperationGreater,
		ast.OperationGreaterEqual:

		return BinaryOperationKindIntegerComparison

	case ast.OperationOr,
		ast.OperationAnd:

		return BinaryOperationKindBooleanLogic

	case ast.OperationEqual,
		ast.OperationUnequal:

		return BinaryOperationKindEquality

	case ast.OperationNilCoalesce:
		return BinaryOperationKindNilCoalescing

	case ast.OperationConcat:
		return BinaryOperationKindConcatenation
	}

	panic(&errors.UnreachableError{})
}
