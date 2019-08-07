package common

import "github.com/dapperlabs/bamboo-node/pkg/language/runtime/errors"

//go:generate stringer -type=DeclarationKind

type DeclarationKind int

const (
	DeclarationKindUnknown DeclarationKind = iota
	DeclarationKindValue
	DeclarationKindFunction
	DeclarationKindVariable
	DeclarationKindConstant
	DeclarationKindType
	DeclarationKindParameter
	DeclarationKindArgumentLabel
	DeclarationKindStructure
	DeclarationKindField
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
	case DeclarationKindParameter:
		return "parameter"
	case DeclarationKindArgumentLabel:
		return "argument label"
	case DeclarationKindStructure:
		return "structure"
	case DeclarationKindField:
		return "field"
	}

	panic(&errors.UnreachableError{})
}
