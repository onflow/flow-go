package interpreter

import "bamboo-runtime/execution/strictus/errors"

//go:generate stringer -type=DeclarationKind

type DeclarationKind int

const (
	DeclarationKindAny DeclarationKind = iota
	DeclarationKindFunction
	DeclarationKindVariable
	DeclarationKindConstant
	DeclarationKindType
)

func (k DeclarationKind) Name() string {
	switch k {
	case DeclarationKindAny:
		return "any"
	case DeclarationKindFunction:
		return "function"
	case DeclarationKindVariable:
		return "variable"
	case DeclarationKindConstant:
		return "constant"
	case DeclarationKindType:
		return "type"
	}

	panic(&errors.UnreachableError{})
}
