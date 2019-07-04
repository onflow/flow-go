package interpreter

import "bamboo-runtime/execution/strictus/errors"

//go:generate stringer -type=OperationKind

type OperationKind int

const (
	OperationKindUnary OperationKind = iota
	OperationKindBinary
	OperationKindTernary
)

func (k OperationKind) Name() string {
	switch k {
	case OperationKindUnary:
		return "unary"
	case OperationKindBinary:
		return "binary"
	case OperationKindTernary:
		return "ternary"
	}

	panic(&errors.UnreachableError{})
}
