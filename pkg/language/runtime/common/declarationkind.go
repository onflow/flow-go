package common

import "github.com/dapperlabs/bamboo-node/pkg/language/runtime/errors"

//go:generate stringer -type=DeclarationKind

type DeclarationKind int

const (
	DeclarationKindValue DeclarationKind = iota
	DeclarationKindFunction
	DeclarationKindVariable
	DeclarationKindConstant
	DeclarationKindType
)

func (k DeclarationKind) Name() string {
	switch k {
	case DeclarationKindValue:
		return "value"
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
