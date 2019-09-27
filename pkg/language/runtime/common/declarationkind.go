package common

import "github.com/dapperlabs/flow-go/pkg/language/runtime/errors"

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
	DeclarationKindResource
	DeclarationKindContract
	DeclarationKindField
	DeclarationKindInitializer
	DeclarationKindStructureInterface
	DeclarationKindResourceInterface
	DeclarationKindContractInterface
	DeclarationKindImport
	DeclarationKindSelf
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
	case DeclarationKindResource:
		return "resource"
	case DeclarationKindContract:
		return "contract"
	case DeclarationKindField:
		return "field"
	case DeclarationKindInitializer:
		return "initializer"
	case DeclarationKindStructureInterface:
		return "structure interface"
	case DeclarationKindResourceInterface:
		return "resource interface"
	case DeclarationKindContractInterface:
		return "contract interface"
	case DeclarationKindImport:
		return "import"
	case DeclarationKindSelf:
		return "self"
	}

	panic(&errors.UnreachableError{})
}
