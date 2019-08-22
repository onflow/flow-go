package ast

import "github.com/dapperlabs/bamboo-node/pkg/language/runtime/errors"

//go:generate stringer -type=Operation

type Operation int

const (
	OperationUnknown Operation = iota
	OperationOr
	OperationAnd
	OperationEqual
	OperationUnequal
	OperationLess
	OperationGreater
	OperationLessEqual
	OperationGreaterEqual
	OperationPlus
	OperationMinus
	OperationMul
	OperationDiv
	OperationMod
	OperationNegate
	OperationNilCoalesce
)

func (s Operation) Symbol() string {
	switch s {
	case OperationOr:
		return "||"
	case OperationAnd:
		return "&&"
	case OperationEqual:
		return "=="
	case OperationUnequal:
		return "!="
	case OperationLess:
		return "<"
	case OperationGreater:
		return ">"
	case OperationLessEqual:
		return "<="
	case OperationGreaterEqual:
		return ">="
	case OperationPlus:
		return "+"
	case OperationMinus:
		return "-"
	case OperationMul:
		return "*"
	case OperationDiv:
		return "/"
	case OperationMod:
		return "%"
	case OperationNegate:
		return "!"
	case OperationNilCoalesce:
		return "??"
	}

	panic(&errors.UnreachableError{})
}
